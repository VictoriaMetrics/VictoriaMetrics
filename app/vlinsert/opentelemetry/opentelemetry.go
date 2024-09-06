package opentelemetry

import (
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vlinsert/insertutils"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vlstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/opentelemetry/pb"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/slicesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/writeconcurrencylimiter"
	"github.com/VictoriaMetrics/metrics"
)

// RequestHandler processes Opentelemetry insert requests
func RequestHandler(path string, w http.ResponseWriter, r *http.Request) bool {
	switch path {
	// use the same path as opentelemetry collector
	// https://opentelemetry.io/docs/specs/otlp/#otlphttp-request
	case "/v1/logs":
		if r.Header.Get("Content-Type") == "application/json" {
			httpserver.Errorf(w, r, "json encoding isn't supported for opentelemetry format. Use protobuf encoding")
			return true
		}
		handleProtobuf(r, w)
		return true
	default:
		return false
	}
}

func handleProtobuf(r *http.Request, w http.ResponseWriter) {
	startTime := time.Now()
	requestsProtobufTotal.Inc()
	reader := r.Body
	if r.Header.Get("Content-Encoding") == "gzip" {
		zr, err := common.GetGzipReader(reader)
		if err != nil {
			httpserver.Errorf(w, r, "cannot initialize gzip reader: %s", err)
			return
		}
		defer common.PutGzipReader(zr)
		reader = zr
	}

	wcr := writeconcurrencylimiter.GetReader(reader)
	data, err := io.ReadAll(wcr)
	writeconcurrencylimiter.PutReader(wcr)
	if err != nil {
		httpserver.Errorf(w, r, "cannot read request body: %s", err)
		return
	}

	cp, err := insertutils.GetCommonParams(r)
	if err != nil {
		httpserver.Errorf(w, r, "cannot parse common params from request: %s", err)
		return
	}
	if err := vlstorage.CanWriteData(); err != nil {
		httpserver.Errorf(w, r, "%s", err)
		return
	}

	lmp := cp.NewLogMessageProcessor()
	n, err := pushProtobufRequest(data, lmp)
	lmp.MustClose()
	if err != nil {
		httpserver.Errorf(w, r, "cannot parse OpenTelemetry protobuf request: %s", err)
		return
	}

	rowsIngestedProtobufTotal.Add(n)

	// update requestProtobufDuration only for successfully parsed requests
	// There is no need in updating requestProtobufDuration for request errors,
	// since their timings are usually much smaller than the timing for successful request parsing.
	requestProtobufDuration.UpdateDuration(startTime)
}

var (
	rowsIngestedProtobufTotal = metrics.NewCounter(`vl_rows_ingested_total{type="opentelemetry",format="protobuf"}`)

	requestsProtobufTotal = metrics.NewCounter(`vl_http_requests_total{path="/insert/opentelemetry/v1/logs",format="protobuf"}`)
	errorsTotal           = metrics.NewCounter(`vl_http_errors_total{path="/insert/opentelemetry/v1/logs",format="protobuf"}`)

	requestProtobufDuration = metrics.NewHistogram(`vl_http_request_duration_seconds{path="/insert/opentelemetry/v1/logs",format="protobuf"}`)
)

func pushProtobufRequest(data []byte, lmp insertutils.LogMessageProcessor) (int, error) {
	var req pb.ExportLogsServiceRequest
	if err := req.UnmarshalProtobuf(data); err != nil {
		errorsTotal.Inc()
		return 0, fmt.Errorf("cannot unmarshal request from %d bytes: %w", len(data), err)
	}

	var rowsIngested int
	var commonFields []logstorage.Field
	for _, rl := range req.ResourceLogs {
		attributes := rl.Resource.Attributes
		commonFields = slicesutil.SetLength(commonFields, len(attributes))
		for i, attr := range attributes {
			commonFields[i].Name = attr.Key
			commonFields[i].Value = attr.Value.FormatString()
		}
		commonFieldsLen := len(commonFields)
		for _, sc := range rl.ScopeLogs {
			var scopeIngested int
			commonFields, scopeIngested = pushFieldsFromScopeLogs(&sc, commonFields[:commonFieldsLen], lmp)
			rowsIngested += scopeIngested
		}
	}

	return rowsIngested, nil
}

func pushFieldsFromScopeLogs(sc *pb.ScopeLogs, commonFields []logstorage.Field, lmp insertutils.LogMessageProcessor) ([]logstorage.Field, int) {
	fields := commonFields
	for _, lr := range sc.LogRecords {
		fields = fields[:len(commonFields)]
		fields = append(fields, logstorage.Field{
			Name:  "_msg",
			Value: lr.Body.FormatString(),
		})
		for _, attr := range lr.Attributes {
			fields = append(fields, logstorage.Field{
				Name:  attr.Key,
				Value: attr.Value.FormatString(),
			})
		}
		fields = append(fields, logstorage.Field{
			Name:  "severity",
			Value: lr.FormatSeverity(),
		})

		lmp.AddRow(lr.ExtractTimestampNano(), fields)
	}
	return fields, len(sc.LogRecords)
}
