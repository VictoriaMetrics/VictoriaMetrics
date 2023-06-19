package jsonline

import (
	"bufio"
	"errors"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	common "github.com/VictoriaMetrics/VictoriaMetrics/app/vlinsert/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vlstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logstorage"
	pc "github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/writeconcurrencylimiter"
	"github.com/VictoriaMetrics/metrics"
)

// RequestHandler processes jsonline insert requests
func RequestHandler(path string, w http.ResponseWriter, r *http.Request) bool {
	w.Header().Add("Content-Type", "application/json")

	if path != "/" {
		return false
	}
	if method := r.Method; method != "POST" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return true
	}

	requestsTotal.Inc()

	// Extract tenantID
	tenantID, err := logstorage.GetTenantIDFromRequest(r)
	if err != nil {
		httpserver.Errorf(w, r, "%s", err)
		return true
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

	// Extract stream field names from _stream_fields query arg
	var streamFields []string
	if sfs := r.FormValue("_stream_fields"); sfs != "" {
		streamFields = strings.Split(sfs, ",")
	}

	// Extract field names, which must be ignored
	var ignoreFields []string
	if ifs := r.FormValue("ignore_fields"); ifs != "" {
		ignoreFields = strings.Split(ifs, ",")
	}

	lr := logstorage.GetLogRows(streamFields, ignoreFields)
	processLogMessage := func(timestamp int64, fields []logstorage.Field) {
		lr.MustAdd(tenantID, timestamp, fields)
		if lr.NeedFlush() {
			vlstorage.MustAddRows(lr)
			lr.Reset()
		}
	}

	reader := r.Body
	if r.Header.Get("Content-Encoding") == "gzip" {
		zr, err := pc.GetGzipReader(reader)
		if err != nil {
			//return 0, fmt.Errorf("cannot read gzipped _bulk request: %w", err)
			return true
		}
		defer pc.PutGzipReader(zr)
		reader = zr
	}

	wcr := writeconcurrencylimiter.GetReader(reader)
	defer writeconcurrencylimiter.PutReader(wcr)

	lb := lineBufferPool.Get()
	defer lineBufferPool.Put(lb)

	lb.B = bytesutil.ResizeNoCopyNoOverallocate(lb.B, common.MaxLineSizeBytes.IntN())
	sc := bufio.NewScanner(wcr)
	sc.Buffer(lb.B, len(lb.B))
	sc.Buffer(lb.B, len(lb.B))

	n := 0
	for {
		ok, err := readLine(sc, timeField, msgField, processLogMessage)
		wcr.DecConcurrency()
		if err != nil {
			logger.Errorf("cannot read line #%d in /jsonline request: %s", n, err)
		}
		if !ok {
			break
		}
		n++
		rowsIngestedTotal.Inc()
	}

	vlstorage.MustAddRows(lr)
	logstorage.PutLogRows(lr)

	return true
}

func readLine(sc *bufio.Scanner, timeField, msgField string, processLogMessage func(timestamp int64, fields []logstorage.Field)) (bool, error) {
	if !sc.Scan() {
		if err := sc.Err(); err != nil {
			if errors.Is(err, bufio.ErrTooLong) {
				return false, fmt.Errorf(`cannot read json line, since its size exceeds -insert.maxLineSizeBytes=%d`, common.MaxLineSizeBytes.IntN())
			}
			return false, err
		}
		return false, nil
	}

	line := sc.Bytes()
	pctx := common.GetParserCtx()
	if err := pctx.ParseLogMessage(line); err != nil {
		invalidJSONLineLogger.Warnf("cannot parse json-encoded log entry: %s", err)
		return true, nil
	}

	timestamp, err := extractTimestampFromFields(timeField, pctx.Fields())
	if err != nil {
		invalidTimestampLogger.Warnf("skipping the log entry because cannot parse timestamp: %s", err)
		return true, nil
	}
	pctx.RenameField(msgField, "_msg")
	processLogMessage(timestamp, pctx.Fields())
	common.PutParserCtx(pctx)
	return true, nil
}

func extractTimestampFromFields(timeField string, fields []logstorage.Field) (int64, error) {
	for i := range fields {
		f := &fields[i]
		if f.Name != timeField {
			continue
		}
		timestamp, err := parseTimestamp(f.Value)
		if err != nil {
			return 0, err
		}
		f.Value = ""
		return timestamp, nil
	}
	return time.Now().UnixNano(), nil
}

func parseTimestamp(s string) (int64, error) {
	if len(s) < len("YYYY-MM-DD") || s[len("YYYY")] != '-' {
		// Try parsing timestamp in milliseconds
		n, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("cannot parse timestamp in milliseconds from %q: %w", s, err)
		}
		if n > int64(math.MaxInt64)/1e6 {
			return 0, fmt.Errorf("too big timestamp in milliseconds: %d; mustn't exceed %d", n, int64(math.MaxInt64)/1e6)
		}
		if n < int64(math.MinInt64)/1e6 {
			return 0, fmt.Errorf("too small timestamp in milliseconds: %d; must be bigger than %d", n, int64(math.MinInt64)/1e6)
		}
		n *= 1e6
		return n, nil
	}
	if len(s) == len("YYYY-MM-DD") {
		t, err := time.Parse("2006-01-02", s)
		if err != nil {
			return 0, fmt.Errorf("cannot parse date %q: %w", s, err)
		}
		return t.UnixNano(), nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return 0, fmt.Errorf("cannot parse timestamp %q: %w", s, err)
	}
	return t.UnixNano(), nil
}

var lineBufferPool bytesutil.ByteBufferPool

var requestsTotal = metrics.NewCounter(`vl_http_requests_total{path="/insert/jsonline"}`)
var rowsIngestedTotal = metrics.NewCounter(`vl_rows_ingested_total{type="jsonline"}`)

var invalidTimestampLogger = logger.WithThrottler("invalidTimestampLogger", 5*time.Second)
var invalidJSONLineLogger = logger.WithThrottler("invalidJSONLineLogger", 5*time.Second)
