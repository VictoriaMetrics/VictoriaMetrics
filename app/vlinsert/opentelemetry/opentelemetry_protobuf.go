package opentelemetry

import (
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vlinsert/insertutils"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vlstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/writeconcurrencylimiter"
	"github.com/VictoriaMetrics/metrics"
)

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
	n, err := pushProtobufRequest(data, cp)
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
	requestsProtobufTotal     = metrics.NewCounter(`vl_http_requests_total{path="/insert/opentelemetry/api/v1/push",format="protobuf"}`)
	rowsIngestedProtobufTotal = metrics.NewCounter(`vl_rows_ingested_total{type="opentelemetry",format="protobuf"}`)
	requestProtobufDuration   = metrics.NewHistogram(`vl_http_request_duration_seconds{path="/insert/opentelemetry/api/v1/push",format="protobuf"}`)
)

func pushProtobufRequest(data []byte, cp *insertutils.CommonParams) (int, error) {
	var req ExportLogsServiceRequest
	if err := req.UnmarshalProtobuf(data); err != nil {
		return 0, fmt.Errorf("cannot unmarshal request from %d bytes: %w", len(data), err)
	}
	return req.pushFields(cp), nil
}
