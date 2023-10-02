package jsonline

import (
	"bufio"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vlinsert/insertutils"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vlstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logjson"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/writeconcurrencylimiter"
	"github.com/VictoriaMetrics/metrics"
)

// RequestHandler processes jsonline insert requests
func RequestHandler(w http.ResponseWriter, r *http.Request) bool {
	startTime := time.Now()
	w.Header().Add("Content-Type", "application/json")

	if r.Method != "POST" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return true
	}

	requestsTotal.Inc()

	cp, err := insertutils.GetCommonParams(r)
	if err != nil {
		httpserver.Errorf(w, r, "%s", err)
		return true
	}
	if err := vlstorage.CanWriteData(); err != nil {
		httpserver.Errorf(w, r, "%s", err)
		return true
	}
	lr := logstorage.GetLogRows(cp.StreamFields, cp.IgnoreFields)
	processLogMessage := cp.GetProcessLogMessageFunc(lr)

	reader := r.Body
	if r.Header.Get("Content-Encoding") == "gzip" {
		zr, err := common.GetGzipReader(reader)
		if err != nil {
			logger.Errorf("cannot read gzipped _bulk request: %s", err)
			return true
		}
		defer common.PutGzipReader(zr)
		reader = zr
	}

	wcr := writeconcurrencylimiter.GetReader(reader)
	defer writeconcurrencylimiter.PutReader(wcr)

	lb := lineBufferPool.Get()
	defer lineBufferPool.Put(lb)

	lb.B = bytesutil.ResizeNoCopyNoOverallocate(lb.B, insertutils.MaxLineSizeBytes.IntN())
	sc := bufio.NewScanner(wcr)
	sc.Buffer(lb.B, len(lb.B))

	n := 0
	for {
		ok, err := readLine(sc, cp.TimeField, cp.MsgField, processLogMessage)
		wcr.DecConcurrency()
		if err != nil {
			logger.Errorf("cannot read line #%d in /jsonline request: %s", n, err)
			break
		}
		if !ok {
			break
		}
		n++
		rowsIngestedTotal.Inc()
	}

	vlstorage.MustAddRows(lr)
	logstorage.PutLogRows(lr)

	// update jsonlineRequestDuration only for successfully parsed requests.
	// There is no need in updating jsonlineRequestDuration for request errors,
	// since their timings are usually much smaller than the timing for successful request parsing.
	jsonlineRequestDuration.UpdateDuration(startTime)

	return true
}

func readLine(sc *bufio.Scanner, timeField, msgField string, processLogMessage func(timestamp int64, fields []logstorage.Field)) (bool, error) {
	var line []byte
	for len(line) == 0 {
		if !sc.Scan() {
			if err := sc.Err(); err != nil {
				if errors.Is(err, bufio.ErrTooLong) {
					return false, fmt.Errorf(`cannot read json line, since its size exceeds -insert.maxLineSizeBytes=%d`, insertutils.MaxLineSizeBytes.IntN())
				}
				return false, err
			}
			return false, nil
		}
		line = sc.Bytes()
	}

	p := logjson.GetParser()
	if err := p.ParseLogMessage(line); err != nil {
		return false, fmt.Errorf("cannot parse json-encoded log entry: %w", err)
	}
	ts, err := extractTimestampFromFields(timeField, p.Fields)
	if err != nil {
		return false, fmt.Errorf("cannot parse timestamp: %w", err)
	}
	if ts == 0 {
		ts = time.Now().UnixNano()
	}
	p.RenameField(msgField, "_msg")
	processLogMessage(ts, p.Fields)
	logjson.PutParser(p)

	return true, nil
}

func extractTimestampFromFields(timeField string, fields []logstorage.Field) (int64, error) {
	for i := range fields {
		f := &fields[i]
		if f.Name != timeField {
			continue
		}
		timestamp, err := parseISO8601Timestamp(f.Value)
		if err != nil {
			return 0, err
		}
		f.Value = ""
		return timestamp, nil
	}
	return 0, nil
}

func parseISO8601Timestamp(s string) (int64, error) {
	if s == "0" || s == "" {
		// Special case for returning the current timestamp.
		// It must be automatically converted to the current timestamp by the caller.
		return 0, nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return 0, fmt.Errorf("cannot parse timestamp %q: %w", s, err)
	}
	return t.UnixNano(), nil
}

var lineBufferPool bytesutil.ByteBufferPool

var (
	requestsTotal           = metrics.NewCounter(`vl_http_requests_total{path="/insert/jsonline"}`)
	rowsIngestedTotal       = metrics.NewCounter(`vl_rows_ingested_total{type="jsonline"}`)
	jsonlineRequestDuration = metrics.NewHistogram(`vl_http_request_duration_seconds{path="/insert/jsonline"}`)
)
