package insertutils

import (
	"net/http"
	"sync"
	"time"

	"github.com/VictoriaMetrics/metrics"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vlstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httputils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/timeutil"
)

// CommonParams contains common HTTP parameters used by log ingestion APIs.
//
// See https://docs.victoriametrics.com/victorialogs/data-ingestion/#http-parameters
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

// GetCommonParamsForSyslog returns common params needed for parsing syslog messages and storing them to the given tenantID.
func GetCommonParamsForSyslog(tenantID logstorage.TenantID) *CommonParams {
	// See https://docs.victoriametrics.com/victorialogs/logsql/#unpack_syslog-pipe
	cp := &CommonParams{
		TenantID:  tenantID,
		TimeField: "timestamp",
		MsgField:  "message",
		StreamFields: []string{
			"hostname",
			"app_name",
			"proc_id",
		},
	}

	return cp
}

// LogMessageProcessor is an interface for log message processors.
type LogMessageProcessor interface {
	// AddRow must add row to the LogMessageProcessor with the given timestamp and the given fields.
	//
	// The LogMessageProcessor implementation cannot hold references to fields, since the caller can re-use them.
	AddRow(timestamp int64, fields []logstorage.Field)

	// MustClose() must flush all the remaining fields and free up resources occupied by LogMessageProcessor.
	MustClose()
}

type logMessageProcessor struct {
	mu            sync.Mutex
	wg            sync.WaitGroup
	stopCh        chan struct{}
	lastFlushTime time.Time

	cp *CommonParams
	lr *logstorage.LogRows
}

func (lmp *logMessageProcessor) initPeriodicFlush() {
	lmp.lastFlushTime = time.Now()

	lmp.wg.Add(1)
	go func() {
		defer lmp.wg.Done()

		d := timeutil.AddJitterToDuration(time.Second)
		ticker := time.NewTicker(d)
		defer ticker.Stop()

		for {
			select {
			case <-lmp.stopCh:
				return
			case <-ticker.C:
				lmp.mu.Lock()
				if time.Since(lmp.lastFlushTime) >= d {
					lmp.flushLocked()
				}
				lmp.mu.Unlock()
			}
		}
	}()
}

// AddRow adds new log message to lmp with the given timestamp and fields.
func (lmp *logMessageProcessor) AddRow(timestamp int64, fields []logstorage.Field) {
	lmp.mu.Lock()
	defer lmp.mu.Unlock()

	if len(fields) > *MaxFieldsPerLine {
		rf := logstorage.RowFormatter(fields)
		logger.Warnf("dropping log line with %d fields; it exceeds -insert.maxFieldsPerLine=%d; %s", len(fields), *MaxFieldsPerLine, rf)
		rowsDroppedTotalTooManyFields.Inc()
		return
	}

	lmp.lr.MustAdd(lmp.cp.TenantID, timestamp, fields)
	if lmp.cp.Debug {
		s := lmp.lr.GetRowString(0)
		lmp.lr.ResetKeepSettings()
		logger.Infof("remoteAddr=%s; requestURI=%s; ignoring log entry because of `debug` query arg: %s", lmp.cp.DebugRemoteAddr, lmp.cp.DebugRequestURI, s)
		rowsDroppedTotalDebug.Inc()
		return
	}
	if lmp.lr.NeedFlush() {
		lmp.flushLocked()
	}
}

// flushLocked must be called under locked lmp.mu.
func (lmp *logMessageProcessor) flushLocked() {
	lmp.lastFlushTime = time.Now()
	vlstorage.MustAddRows(lmp.lr)
	lmp.lr.ResetKeepSettings()
}

// MustClose flushes the remaining data to the underlying storage and closes lmp.
func (lmp *logMessageProcessor) MustClose() {
	close(lmp.stopCh)
	lmp.wg.Wait()

	lmp.flushLocked()
	logstorage.PutLogRows(lmp.lr)
	lmp.lr = nil
}

// NewLogMessageProcessor returns new LogMessageProcessor for the given cp.
//
// MustClose() must be called on the returned LogMessageProcessor when it is no longer needed.
func (cp *CommonParams) NewLogMessageProcessor() LogMessageProcessor {
	lr := logstorage.GetLogRows(cp.StreamFields, cp.IgnoreFields)
	lmp := &logMessageProcessor{
		cp: cp,
		lr: lr,

		stopCh: make(chan struct{}),
	}
	lmp.initPeriodicFlush()

	return lmp
}

var rowsDroppedTotalDebug = metrics.NewCounter(`vl_rows_dropped_total{reason="debug"}`)
var rowsDroppedTotalTooManyFields = metrics.NewCounter(`vl_rows_dropped_total{reason="too_many_fields"}`)
