package insertutil

import (
	"flag"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/VictoriaMetrics/metrics"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httputil"
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
	TenantID         logstorage.TenantID
	TimeFields       []string
	MsgFields        []string
	StreamFields     []string
	IgnoreFields     []string
	DecolorizeFields []string
	ExtraFields      []logstorage.Field

	IsTimeFieldSet  bool
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

	var isTimeFieldSet bool
	timeFields := []string{"_time"}
	if tfs := httputil.GetArray(r, "_time_field", "VL-Time-Field"); len(tfs) > 0 {
		isTimeFieldSet = true
		timeFields = tfs
	}

	msgFields := httputil.GetArray(r, "_msg_field", "VL-Msg-Field")
	streamFields := httputil.GetArray(r, "_stream_fields", "VL-Stream-Fields")
	ignoreFields := httputil.GetArray(r, "ignore_fields", "VL-Ignore-Fields")
	decolorizeFields := httputil.GetArray(r, "decolorize_fields", "VL-Decolorize-Fields")

	extraFields, err := getExtraFields(r)
	if err != nil {
		return nil, err
	}

	debug := false
	if dv := httputil.GetRequestValue(r, "debug", "VL-Debug"); dv != "" {
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
		TenantID:         tenantID,
		TimeFields:       timeFields,
		MsgFields:        msgFields,
		StreamFields:     streamFields,
		IgnoreFields:     ignoreFields,
		DecolorizeFields: decolorizeFields,
		ExtraFields:      extraFields,

		IsTimeFieldSet:  isTimeFieldSet,
		Debug:           debug,
		DebugRequestURI: debugRequestURI,
		DebugRemoteAddr: debugRemoteAddr,
	}

	return cp, nil
}

func getExtraFields(r *http.Request) ([]logstorage.Field, error) {
	efs := httputil.GetArray(r, "extra_fields", "VL-Extra-Fields")
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
func GetCommonParamsForSyslog(tenantID logstorage.TenantID, streamFields, ignoreFields, decolorizeFields []string, extraFields []logstorage.Field) *CommonParams {
	// See https://docs.victoriametrics.com/victorialogs/logsql/#unpack_syslog-pipe
	if streamFields == nil {
		streamFields = []string{
			"hostname",
			"app_name",
			"proc_id",
		}
	}
	cp := &CommonParams{
		TenantID: tenantID,
		TimeFields: []string{
			"timestamp",
		},
		MsgFields: []string{
			"message",
		},
		StreamFields:     streamFields,
		IgnoreFields:     ignoreFields,
		DecolorizeFields: decolorizeFields,
		ExtraFields:      extraFields,
	}

	return cp
}

// LogRowsStorage is an interface for ingesting logs into the storage.
type LogRowsStorage interface {
	// MustAddRows must add lr to the underlying storage.
	MustAddRows(lr *logstorage.LogRows)

	// CanWriteData must returns non-nil error if logs cannot be added to the underlying storage.
	CanWriteData() error
}

var logRowsStorage LogRowsStorage

// SetLogRowsStorage sets the storage for writing data to via LogMessageProcessor.
//
// This function must be called before using LogMessageProcessor and CanWriteData from this package.
func SetLogRowsStorage(storage LogRowsStorage) {
	logRowsStorage = storage
}

// CanWriteData returns non-nil error if data cannot be written to the underlying storage.
func CanWriteData() error {
	return logRowsStorage.CanWriteData()
}

// LogMessageProcessor is an interface for log message processors.
type LogMessageProcessor interface {
	// AddRow must add row to the LogMessageProcessor with the given timestamp and fields.
	//
	// If streamFields is non-nil, then the given streamFields must be used as log stream fields instead of pre-configured fields.
	//
	// The LogMessageProcessor implementation cannot hold references to fields, since the caller can reuse them.
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
	flushDuration      *metrics.Summary
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
	lmp.rowsIngestedTotal.Inc()
	n := logstorage.EstimatedJSONRowLen(fields)
	lmp.bytesIngestedTotal.Add(n)

	if len(fields) > *MaxFieldsPerLine {
		line := logstorage.MarshalFieldsToJSON(nil, fields)
		logger.Warnf("dropping log line with %d fields; it exceeds -insert.maxFieldsPerLine=%d; %s", len(fields), *MaxFieldsPerLine, line)
		rowsDroppedTotalTooManyFields.Inc()
		return
	}

	lmp.mu.Lock()
	defer lmp.mu.Unlock()

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

// InsertRowProcessor is used by native data ingestion protocol parser.
type InsertRowProcessor interface {
	// AddInsertRow must add r to the underlying storage.
	AddInsertRow(r *logstorage.InsertRow)
}

// AddInsertRow adds r to lmp.
func (lmp *logMessageProcessor) AddInsertRow(r *logstorage.InsertRow) {
	lmp.rowsIngestedTotal.Inc()
	n := logstorage.EstimatedJSONRowLen(r.Fields)
	lmp.bytesIngestedTotal.Add(n)

	if len(r.Fields) > *MaxFieldsPerLine {
		line := logstorage.MarshalFieldsToJSON(nil, r.Fields)
		logger.Warnf("dropping log line with %d fields; it exceeds -insert.maxFieldsPerLine=%d; %s", len(r.Fields), *MaxFieldsPerLine, line)
		rowsDroppedTotalTooManyFields.Inc()
		return
	}

	lmp.mu.Lock()
	defer lmp.mu.Unlock()

	lmp.lr.MustAddInsertRow(r)

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
	start := time.Now()
	lmp.lastFlushTime = start
	logRowsStorage.MustAddRows(lmp.lr)
	lmp.lr.ResetKeepSettings()
	lmp.flushDuration.UpdateDuration(start)
}

// MustClose flushes the remaining data to the underlying storage and closes lmp.
func (lmp *logMessageProcessor) MustClose() {
	close(lmp.stopCh)
	lmp.wg.Wait()

	lmp.flushLocked()
	logstorage.PutLogRows(lmp.lr)
	lmp.lr = nil
	messageProcessorCount.Add(-1)
}

// NewLogMessageProcessor returns new LogMessageProcessor for the given cp.
//
// MustClose() must be called on the returned LogMessageProcessor when it is no longer needed.
func (cp *CommonParams) NewLogMessageProcessor(protocolName string, isStreamMode bool) LogMessageProcessor {
	lr := logstorage.GetLogRows(cp.StreamFields, cp.IgnoreFields, cp.DecolorizeFields, cp.ExtraFields, *defaultMsgValue)
	rowsIngestedTotal := metrics.GetOrCreateCounter(fmt.Sprintf("vl_rows_ingested_total{type=%q}", protocolName))
	bytesIngestedTotal := metrics.GetOrCreateCounter(fmt.Sprintf("vl_bytes_ingested_total{type=%q}", protocolName))
	flushDuration := metrics.GetOrCreateSummary(fmt.Sprintf("vl_insert_flush_duration_seconds{type=%q}", protocolName))
	lmp := &logMessageProcessor{
		cp: cp,
		lr: lr,

		rowsIngestedTotal:  rowsIngestedTotal,
		bytesIngestedTotal: bytesIngestedTotal,
		flushDuration:      flushDuration,

		stopCh: make(chan struct{}),
	}

	if isStreamMode {
		lmp.initPeriodicFlush()
	}

	messageProcessorCount.Add(1)
	return lmp
}

var (
	rowsDroppedTotalDebug         = metrics.NewCounter(`vl_rows_dropped_total{reason="debug"}`)
	rowsDroppedTotalTooManyFields = metrics.NewCounter(`vl_rows_dropped_total{reason="too_many_fields"}`)
	_                             = metrics.NewGauge(`vl_insert_processors_count`, func() float64 { return float64(messageProcessorCount.Load()) })
	messageProcessorCount         atomic.Int64
)
