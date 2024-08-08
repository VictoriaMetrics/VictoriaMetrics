package opentelemetry

import (
	"fmt"
	"net/http"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vlinsert/insertutils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logstorage"
	opentelemetry "github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/opentelemetry/stream"
	"github.com/VictoriaMetrics/metrics"
)

// RequestHandler processes Opentelemetry insert requests
func RequestHandler(path string, w http.ResponseWriter, r *http.Request) bool {
	switch path {
	case "/api/v1/push":
		return handleInsert(r, w)
	default:
		return false
	}
}

func handleInsert(r *http.Request, w http.ResponseWriter) bool {
	startTime := time.Now()
	var m *otelMetrics
	contentType := r.Header.Get("Content-Type")
	if contentType == "application/json" {
		m = jsonMetrics
	} else {
		m = protobufMetrics
	}
	isGzipped := r.Header.Get("Content-Encoding") == "gzip"
	m.requestsTotal.Inc()
	cp, err := insertutils.GetCommonParams(r)
	if err != nil {
		httpserver.Errorf(w, r, "cannot parse common params from request: %s", err)
		return true
	}
	var lmp insertutils.LogMessageProcessor
	n, err := opentelemetry.ParseLogsStream(r.Body, contentType, isGzipped, func(streamFields []string) func(int64, []logstorage.Field) {
		cp.StreamFields = streamFields
		lmp = cp.NewLogMessageProcessor()
		return lmp.AddRow
	})
	if lmp != nil {
		lmp.MustClose()
	}
	if err != nil {
		httpserver.Errorf(w, r, "cannot parse Opentelemetry request: %s", err)
		return true
	}

	m.ingestedTotal.Add(n)

	m.requestsDuration.UpdateDuration(startTime)

	return true
}

type otelMetrics struct {
	requestsTotal    *metrics.Counter
	ingestedTotal    *metrics.Counter
	requestsDuration *metrics.Histogram
}

func newMetrics(format string) *otelMetrics {
	return &otelMetrics{
		requestsTotal:    metrics.NewCounter(fmt.Sprintf(`vl_http_requests_total{path="/insert/opentelemetry/api/v1/push",format="%s"}`, format)),
		ingestedTotal:    metrics.NewCounter(fmt.Sprintf(`vl_rows_ingested_total{type="opentelemetry",format="%s"}`, format)),
		requestsDuration: metrics.NewHistogram(fmt.Sprintf(`vl_http_request_duration_seconds{path="/insert/opentelemetry/api/v1/push",format="%s"}`, format)),
	}
}

var (
	jsonMetrics     = newMetrics("json")
	protobufMetrics = newMetrics("protobuf")
)
