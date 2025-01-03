package jsonline

import (
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vlinsert/insertutils"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vlstorage"
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

	lmp := cp.NewLogMessageProcessor("jsonline")
	streamName := fmt.Sprintf("remoteAddr=%s, requestURI=%q", httpserver.GetQuotedRemoteAddr(r), r.RequestURI)
	err = processStreamInternal(streamName, reader, cp.TimeField, cp.MsgFields, lmp)
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

func processStreamInternal(streamName string, r io.Reader, timeField string, msgFields []string, lmp insertutils.LogMessageProcessor) error {
	wcr := writeconcurrencylimiter.GetReader(r)
	defer writeconcurrencylimiter.PutReader(wcr)

	lr := insertutils.NewLineReader(streamName, wcr)

	n := 0
	for {
		ok, err := readLine(lr, timeField, msgFields, lmp)
		wcr.DecConcurrency()
		if err != nil {
			errorsTotal.Inc()
			return fmt.Errorf("cannot read line #%d in /jsonline request: %s", n, err)
		}
		if !ok {
			return nil
		}
		n++
	}
}

func readLine(lr *insertutils.LineReader, timeField string, msgFields []string, lmp insertutils.LogMessageProcessor) (bool, error) {
	var line []byte
	for len(line) == 0 {
		if !lr.NextLine() {
			err := lr.Err()
			return false, err
		}
		line = lr.Line
	}

	p := logstorage.GetJSONParser()
	if err := p.ParseLogMessage(line); err != nil {
		return false, fmt.Errorf("cannot parse json-encoded log entry: %w", err)
	}
	ts, err := insertutils.ExtractTimestampRFC3339NanoFromFields(timeField, p.Fields)
	if err != nil {
		return false, fmt.Errorf("cannot get timestamp: %w", err)
	}
	logstorage.RenameField(p.Fields, msgFields, "_msg")
	lmp.AddRow(ts, p.Fields, nil)
	logstorage.PutJSONParser(p)

	return true, nil
}

var (
	requestsTotal = metrics.NewCounter(`vl_http_requests_total{path="/insert/jsonline"}`)
	errorsTotal   = metrics.NewCounter(`vl_http_errors_total{path="/insert/jsonline"}`)

	requestDuration = metrics.NewHistogram(`vl_http_request_duration_seconds{path="/insert/jsonline"}`)
)
