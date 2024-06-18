package jsonline

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vlinsert/insertutils"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vlstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/writeconcurrencylimiter"
	"github.com/VictoriaMetrics/metrics"
)

// RequestHandler processes jsonline insert requests
func RequestHandler(w http.ResponseWriter, r *http.Request) {
	startTime := time.Now()
	w.Header().Add("Content-Type", "application/json")

	if r.Method != "POST" {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	requestsTotal.Inc()

	cp, err := insertutils.GetCommonParams(r)
	if err != nil {
		httpserver.Errorf(w, r, "%s", err)
		return
	}
	if err := vlstorage.CanWriteData(); err != nil {
		httpserver.Errorf(w, r, "%s", err)
		return
	}

	reader := r.Body
	if r.Header.Get("Content-Encoding") == "gzip" {
		zr, err := common.GetGzipReader(reader)
		if err != nil {
			logger.Errorf("cannot read gzipped jsonline request: %s", err)
			return
		}
		defer common.PutGzipReader(zr)
		reader = zr
	}

	lmp := cp.NewLogMessageProcessor()
	err = processStreamInternal(reader, cp.TimeField, cp.MsgField, lmp)
	lmp.MustClose()

	if err != nil {
		logger.Errorf("jsonline: %s", err)
	} else {
		// update requestDuration only for successfully parsed requests.
		// There is no need in updating requestDuration for request errors,
		// since their timings are usually much smaller than the timing for successful request parsing.
		requestDuration.UpdateDuration(startTime)
	}
}

func processStreamInternal(r io.Reader, timeField, msgField string, lmp insertutils.LogMessageProcessor) error {
	wcr := writeconcurrencylimiter.GetReader(r)
	defer writeconcurrencylimiter.PutReader(wcr)

	lb := lineBufferPool.Get()
	defer lineBufferPool.Put(lb)

	lb.B = bytesutil.ResizeNoCopyNoOverallocate(lb.B, insertutils.MaxLineSizeBytes.IntN())
	sc := bufio.NewScanner(wcr)
	sc.Buffer(lb.B, len(lb.B))

	n := 0
	for {
		ok, err := readLine(sc, timeField, msgField, lmp)
		wcr.DecConcurrency()
		if err != nil {
			errorsTotal.Inc()
			return fmt.Errorf("cannot read line #%d in /jsonline request: %s", n, err)
		}
		if !ok {
			return nil
		}
		n++
		rowsIngestedTotal.Inc()
	}
}

func readLine(sc *bufio.Scanner, timeField, msgField string, lmp insertutils.LogMessageProcessor) (bool, error) {
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

	p := logstorage.GetJSONParser()
	if err := p.ParseLogMessage(line); err != nil {
		return false, fmt.Errorf("cannot parse json-encoded log entry: %w", err)
	}
	ts, err := insertutils.ExtractTimestampRFC3339NanoFromFields(timeField, p.Fields)
	if err != nil {
		return false, fmt.Errorf("cannot get timestamp: %w", err)
	}
	logstorage.RenameField(p.Fields, msgField, "_msg")
	lmp.AddRow(ts, p.Fields)
	logstorage.PutJSONParser(p)

	return true, nil
}

var lineBufferPool bytesutil.ByteBufferPool

var (
	rowsIngestedTotal = metrics.NewCounter(`vl_rows_ingested_total{type="jsonline"}`)

	requestsTotal = metrics.NewCounter(`vl_http_requests_total{path="/insert/jsonline"}`)
	errorsTotal   = metrics.NewCounter(`vl_http_errors_total{path="/insert/jsonline"}`)

	requestDuration = metrics.NewHistogram(`vl_http_request_duration_seconds{path="/insert/jsonline"}`)
)
