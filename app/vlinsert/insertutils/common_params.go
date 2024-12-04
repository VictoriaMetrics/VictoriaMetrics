package insertutils

import (
	"flag"
	"fmt"
	"net/http"
	"strconv"
	"strings"
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

var (
	defaultMsgValue = flag.String("defaultMsgValue", "missing _msg field; see https://docs.victoriametrics.com/victorialogs/keyconcepts/#message-field",
		"Default value for _msg field if the ingested log entry doesn't contain it; see https://docs.victoriametrics.com/victorialogs/keyconcepts/#message-field")
)

// CommonParams contains common HTTP parameters used by log ingestion APIs.
//
// See https://docs.victoriametrics.com/victorialogs/data-ingestion/#http-parameters
type CommonParams struct {
	TenantID     logstorage.TenantID
	TimeField    string
	MsgFields    []string
	StreamFields []string
	IgnoreFields []string
	ExtraFields  []logstorage.Field

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

	timeField := "_time"
	if tf := httputils.GetRequestValue(r, "_time_field", "VL-Time-Field"); tf != "" {
		timeField = tf
	}

	msgFields := httputils.GetArray(r, "_msg_field", "VL-Msg-Field")
	streamFields := httputils.GetArray(r, "_stream_fields", "VL-Stream-Fields")
	ignoreFields := httputils.GetArray(r, "ignore_fields", "VL-Ignore-Fields")

	extraFields, err := getExtraFields(r)
	if err != nil {
		return nil, err
	}

	debug := false
	if dv := httputils.GetRequestValue(r, "debug", "VL-Debug"); dv != "" {
		debug, err = strconv.ParseBool(dv)
		if err != nil {
			return nil, fmt.Errorf("cannot parse debug=%q: %w", dv, err)
		}
	}
	debugRequestURI := ""
	debugRemoteAddr := ""
	if debug {
		debugRequestURI = httpserver.GetRequestURI(r)
		debugRemoteAddr = httpserver.GetQuotedRemoteAddr(r)
	}

	cp := &CommonParams{
		TenantID:        tenantID,
		TimeField:       timeField,
		MsgFields:       msgFields,
		StreamFields:    streamFields,
		IgnoreFields:    ignoreFields,
		ExtraFields:     extraFields,
		Debug:           debug,
		DebugRequestURI: debugRequestURI,
		DebugRemoteAddr: debugRemoteAddr,
	}

	return cp, nil
}

func getExtraFields(r *http.Request) ([]logstorage.Field, error) {
	efs := httputils.GetArray(r, "extra_fields", "VL-Extra-Fields")
	if len(efs) == 0 {
		return nil, nil
	}

	extraFields := make([]logstorage.Field, len(efs))
	for i, ef := range efs {
		n := strings.Index(ef, "=")
		if n <= 0 || n == len(ef)-1 {
			return nil, fmt.Errorf(`invalid extra_field format: %q; must be in the form "field=value"`, ef)
		}
		extraFields[i] = logstorage.Field{
			Name:  ef[:n],
			Value: ef[n+1:],
		}
	}
	return extraFields, nil
}

// GetCommonParamsForSyslog returns common params needed for parsing syslog messages and storing them to the given tenantID.
func GetCommonParamsForSyslog(tenantID logstorage.TenantID, streamFields, ignoreFields []string, extraFields []logstorage.Field) *CommonParams {
	// See https://docs.victoriametrics.com/victorialogs/logsql/#unpack_syslog-pipe
	if streamFields == nil {
		streamFields = []string{
			"hostname",
			"app_name",
			"proc_id",
		}
	}
	cp := &CommonParams{
		TenantID:  tenantID,
		TimeField: "timestamp",
		MsgFields: []string{
			"message",
		},
		StreamFields: streamFields,
		IgnoreFields: ignoreFields,
		ExtraFields:  extraFields,
	}

	return cp
}

// LogMessageProcessor is an interface for log message processors.
type LogMessageProcessor interface {
	// AddRow must add row to the LogMessageProcessor with the given timestamp and fields.
	//
	// If streamFields is non-nil, then the given streamFields must be used as log stream fields instead of pre-configured fields.
	//
	// The LogMessageProcessor implementation cannot hold references to fields, since the caller can re-use them.
	AddRow(timestamp int64, fields, streamFields []logstorage.Field)

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

	rowsIngestedTotal  *metrics.Counter
	bytesIngestedTotal *metrics.Counter
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
//
// If streamFields is non-nil, then it is used as log stream fields instead of the pre-configured stream fields.
func (lmp *logMessageProcessor) AddRow(timestamp int64, fields, streamFields []logstorage.Field) {
	lmp.mu.Lock()
	defer lmp.mu.Unlock()

	lmp.rowsIngestedTotal.Inc()
	n := logstorage.EstimatedJSONRowLen(fields)
	lmp.bytesIngestedTotal.Add(n)

	if len(fields) > *MaxFieldsPerLine {
		rf := logstorage.RowFormatter(fields)
		logger.Warnf("dropping log line with %d fields; it exceeds -insert.maxFieldsPerLine=%d; %s", len(fields), *MaxFieldsPerLine, rf)
		rowsDroppedTotalTooManyFields.Inc()
		return
	}

	lmp.lr.MustAdd(lmp.cp.TenantID, timestamp, fields, streamFields)
	if lmp.cp.Debug {
		s := lmp.lr.GetRowString(0)
		lmp.lr.ResetKeepSettings()
		logger.Infof("remoteAddr=%s; requestURI=%s; ignoring log entry because of `debug` arg: %s", lmp.cp.DebugRemoteAddr, lmp.cp.DebugRequestURI, s)
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
func (cp *CommonParams) NewLogMessageProcessor(protocolName string) LogMessageProcessor {
	lr := logstorage.GetLogRows(cp.StreamFields, cp.IgnoreFields, cp.ExtraFields, *defaultMsgValue)
	rowsIngestedTotal := metrics.GetOrCreateCounter(fmt.Sprintf("vl_rows_ingested_total{type=%q}", protocolName))
	bytesIngestedTotal := metrics.GetOrCreateCounter(fmt.Sprintf("vl_bytes_ingested_total{type=%q}", protocolName))
	lmp := &logMessageProcessor{
		cp: cp,
		lr: lr,

		rowsIngestedTotal:  rowsIngestedTotal,
		bytesIngestedTotal: bytesIngestedTotal,

		stopCh: make(chan struct{}),
	}
	lmp.initPeriodicFlush()

	return lmp
}

var (
	rowsDroppedTotalDebug         = metrics.NewCounter(`vl_rows_dropped_total{reason="debug"}`)
	rowsDroppedTotalTooManyFields = metrics.NewCounter(`vl_rows_dropped_total{reason="too_many_fields"}`)
)
