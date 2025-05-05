package jsonline

import (
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vlinsert/insertutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vlstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/protoparserutil"
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

	cp, err := insertutil.GetCommonParams(r)
	if err != nil {
		httpserver.Errorf(w, r, "%s", err)
		return
	}
	if err := vlstorage.CanWriteData(); err != nil {
		httpserver.Errorf(w, r, "%s", err)
		return
	}

	encoding := r.Header.Get("Content-Encoding")
	reader, err := protoparserutil.GetUncompressedReader(r.Body, encoding)
	if err != nil {
		logger.Errorf("cannot decode jsonline request: %s", err)
		return
	}
	defer protoparserutil.PutUncompressedReader(reader)

	lmp := cp.NewLogMessageProcessor("jsonline", true)
	streamName := fmt.Sprintf("remoteAddr=%s, requestURI=%q", httpserver.GetQuotedRemoteAddr(r), r.RequestURI)
	processStreamInternal(streamName, reader, cp.TimeField, cp.MsgFields, lmp)
	lmp.MustClose()

	requestDuration.UpdateDuration(startTime)
}

func processStreamInternal(streamName string, r io.Reader, timeField string, msgFields []string, lmp insertutil.LogMessageProcessor) {
	wcr := writeconcurrencylimiter.GetReader(r)
	defer writeconcurrencylimiter.PutReader(wcr)

	lr := insertutil.NewLineReader(streamName, wcr)

	n := 0
	for {
		ok, err := readLine(lr, timeField, msgFields, lmp)
		wcr.DecConcurrency()
		if err != nil {
			errorsTotal.Inc()
			logger.Warnf("jsonline: cannot read line #%d in /jsonline request: %s", n, err)
		}
		if !ok {
			return
		}
		n++
	}
}

func readLine(lr *insertutil.LineReader, timeField string, msgFields []string, lmp insertutil.LogMessageProcessor) (bool, error) {
	var line []byte
	for len(line) == 0 {
		if !lr.NextLine() {
			err := lr.Err()
			return false, err
		}
		line = lr.Line
	}

	p := logstorage.GetJSONParser()
	defer logstorage.PutJSONParser(p)

	if err := p.ParseLogMessage(line); err != nil {
		return true, fmt.Errorf("cannot parse json-encoded line: %w; line contents: %q", err, line)
	}
	ts, err := insertutil.ExtractTimestampFromFields(timeField, p.Fields)
	if err != nil {
		return true, fmt.Errorf("cannot get timestamp from json-encoded line: %w; line contents: %q", err, line)
	}
	logstorage.RenameField(p.Fields, msgFields, "_msg")
	lmp.AddRow(ts, p.Fields, nil)

	return true, nil
}

var (
	requestsTotal = metrics.NewCounter(`vl_http_requests_total{path="/insert/jsonline"}`)
	errorsTotal   = metrics.NewCounter(`vl_http_errors_total{path="/insert/jsonline"}`)

	requestDuration = metrics.NewHistogram(`vl_http_request_duration_seconds{path="/insert/jsonline"}`)
)
