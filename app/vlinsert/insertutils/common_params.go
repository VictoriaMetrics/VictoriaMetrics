package insertutils

import (
	"net/http"

	"github.com/VictoriaMetrics/metrics"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vlstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httputils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logstorage"
)

// CommonParams contains common HTTP parameters used by log ingestion APIs.
//
// See https://docs.victoriametrics.com/VictoriaLogs/data-ingestion/#http-parameters
type CommonParams struct {
	TenantID     logstorage.TenantID
	TimeField    string
	MsgField     string
	StreamFields []string
	IgnoreFields []string

	Debug           bool
	DebugRequestURI string
	DebugRemoteAddr string
}

// GetCommonParams returns CommonParams from r.
func GetCommonParams(r *http.Request) (*CommonParams, error) {
	// Extract tenantID
	tenantID, err := logstorage.GetTenantIDFromRequest(r)
	if err != nil {
		return nil, err
	}

	// Extract time field name from _time_field query arg
	var timeField = "_time"
	if tf := r.FormValue("_time_field"); tf != "" {
		timeField = tf
	}

	// Extract message field name from _msg_field query arg
	var msgField = ""
	if msgf := r.FormValue("_msg_field"); msgf != "" {
		msgField = msgf
	}

	streamFields := httputils.GetArray(r, "_stream_fields")
	ignoreFields := httputils.GetArray(r, "ignore_fields")

	debug := httputils.GetBool(r, "debug")
	debugRequestURI := ""
	debugRemoteAddr := ""
	if debug {
		debugRequestURI = httpserver.GetRequestURI(r)
		debugRemoteAddr = httpserver.GetQuotedRemoteAddr(r)
	}

	cp := &CommonParams{
		TenantID:        tenantID,
		TimeField:       timeField,
		MsgField:        msgField,
		StreamFields:    streamFields,
		IgnoreFields:    ignoreFields,
		Debug:           debug,
		DebugRequestURI: debugRequestURI,
		DebugRemoteAddr: debugRemoteAddr,
	}
	return cp, nil
}

// GetProcessLogMessageFunc returns a function, which adds parsed log messages to lr.
func (cp *CommonParams) GetProcessLogMessageFunc(lr *logstorage.LogRows) func(timestamp int64, fields []logstorage.Field) {
	return func(timestamp int64, fields []logstorage.Field) {
		if len(fields) > *MaxFieldsPerLine {
			rf := logstorage.RowFormatter(fields)
			logger.Warnf("dropping log line with %d fields; it exceeds -insert.maxFieldsPerLine=%d; %s", len(fields), *MaxFieldsPerLine, rf)
			rowsDroppedTotalTooManyFields.Inc()
			return
		}

		lr.MustAdd(cp.TenantID, timestamp, fields)
		if cp.Debug {
			s := lr.GetRowString(0)
			lr.ResetKeepSettings()
			logger.Infof("remoteAddr=%s; requestURI=%s; ignoring log entry because of `debug` query arg: %s", cp.DebugRemoteAddr, cp.DebugRequestURI, s)
			rowsDroppedTotalDebug.Inc()
			return
		}
		if lr.NeedFlush() {
			vlstorage.MustAddRows(lr)
			lr.ResetKeepSettings()
		}
	}
}

var rowsDroppedTotalDebug = metrics.NewCounter(`vl_rows_dropped_total{reason="debug"}`)
var rowsDroppedTotalTooManyFields = metrics.NewCounter(`vl_rows_dropped_total{reason="too_many_fields"}`)
