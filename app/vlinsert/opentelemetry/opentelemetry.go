package opentelemetry

import (
	"fmt"
	"net/http"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vlinsert/insertutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vlstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/opentelemetry/pb"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/protoparserutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/slicesutil"
	"github.com/VictoriaMetrics/metrics"
)

var maxRequestSize = flagutil.NewBytes("opentelemetry.maxRequestSize", 64*1024*1024, "The maximum size in bytes of a single OpenTelemetry request")

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

	cp, err := insertutil.GetCommonParams(r)
	if err != nil {
		httpserver.Errorf(w, r, "cannot parse common params from request: %s", err)
		return
	}
	if err := vlstorage.CanWriteData(); err != nil {
		httpserver.Errorf(w, r, "%s", err)
		return
	}

	encoding := r.Header.Get("Content-Encoding")
	err = protoparserutil.ReadUncompressedData(r.Body, encoding, maxRequestSize, func(data []byte) error {
		lmp := cp.NewLogMessageProcessor("opentelelemtry_protobuf", false)
		useDefaultStreamFields := len(cp.StreamFields) == 0
		err := pushProtobufRequest(data, lmp, useDefaultStreamFields)
		lmp.MustClose()
		return err
	})
	if err != nil {
		httpserver.Errorf(w, r, "cannot read OpenTelemetry protocol data: %s", err)
		return
	}

	// update requestProtobufDuration only for successfully parsed requests
	// There is no need in updating requestProtobufDuration for request errors,
	// since their timings are usually much smaller than the timing for successful request parsing.
	requestProtobufDuration.UpdateDuration(startTime)
}

var (
	requestsProtobufTotal = metrics.NewCounter(`vl_http_requests_total{path="/insert/opentelemetry/v1/logs",format="protobuf"}`)
	errorsTotal           = metrics.NewCounter(`vl_http_errors_total{path="/insert/opentelemetry/v1/logs",format="protobuf"}`)

	requestProtobufDuration = metrics.NewHistogram(`vl_http_request_duration_seconds{path="/insert/opentelemetry/v1/logs",format="protobuf"}`)
)

func pushProtobufRequest(data []byte, lmp insertutil.LogMessageProcessor, useDefaultStreamFields bool) error {
	var req pb.ExportLogsServiceRequest
	if err := req.UnmarshalProtobuf(data); err != nil {
		errorsTotal.Inc()
		return fmt.Errorf("cannot unmarshal request from %d bytes: %w", len(data), err)
	}

	var commonFields []logstorage.Field
	for _, rl := range req.ResourceLogs {
		attributes := rl.Resource.Attributes
		commonFields = slicesutil.SetLength(commonFields, len(attributes))
		for i, attr := range attributes {
			commonFields[i].Name = attr.Key
			commonFields[i].Value = attr.Value.FormatString(true)
		}
		commonFieldsLen := len(commonFields)
		for _, sc := range rl.ScopeLogs {
			commonFields = pushFieldsFromScopeLogs(&sc, commonFields[:commonFieldsLen], lmp, useDefaultStreamFields)
		}
	}

	return nil
}

func pushFieldsFromScopeLogs(sc *pb.ScopeLogs, commonFields []logstorage.Field, lmp insertutil.LogMessageProcessor, useDefaultStreamFields bool) []logstorage.Field {
	fields := commonFields
	for _, lr := range sc.LogRecords {
		fields = fields[:len(commonFields)]
		fields = append(fields, logstorage.Field{
			Name:  "_msg",
			Value: lr.Body.FormatString(true),
		})
		for _, attr := range lr.Attributes {
			fields = append(fields, logstorage.Field{
				Name:  attr.Key,
				Value: attr.Value.FormatString(true),
			})
		}
		if len(lr.TraceID) > 0 {
			fields = append(fields, logstorage.Field{
				Name:  "trace_id",
				Value: lr.TraceID,
			})
		}
		if len(lr.SpanID) > 0 {
			fields = append(fields, logstorage.Field{
				Name:  "span_id",
				Value: lr.SpanID,
			})
		}
		fields = append(fields, logstorage.Field{
			Name:  "severity",
			Value: lr.FormatSeverity(),
		})

		var streamFields []logstorage.Field
		if useDefaultStreamFields {
			streamFields = commonFields
		}
		lmp.AddRow(lr.ExtractTimestampNano(), fields, streamFields)
	}
	return fields
}
