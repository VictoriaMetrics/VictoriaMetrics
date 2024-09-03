package opentelemetry

import (
	"encoding/json"
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

func handleJSON(r *http.Request, w http.ResponseWriter) {
	startTime := time.Now()
	requestsJSONTotal.Inc()
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
	n, err := pushJSONRequest(data, cp)
	if err != nil {
		httpserver.Errorf(w, r, "cannot parse OpenTelemetry json request: %s", err)
		return
	}

	rowsIngestedJSONTotal.Add(n)

	// update requestJSONDuration only for successfully parsed requests
	// There is no need in updating requestJSONDuration for request errors,
	// since their timings are usually much smaller than the timing for successful request parsing.
	requestJSONDuration.UpdateDuration(startTime)
}

var (
	requestsJSONTotal     = metrics.NewCounter(`vl_http_requests_total{path="/insert/opentelemetry/api/v1/push",format="json"}`)
	rowsIngestedJSONTotal = metrics.NewCounter(`vl_rows_ingested_total{type="opentelemetry",format="json"}`)
	requestJSONDuration   = metrics.NewHistogram(`vl_http_request_duration_seconds{path="/insert/opentelemetry/api/v1/push",format="json"}`)
)

func pushJSONRequest(data []byte, cp *insertutils.CommonParams) (int, error) {
	var req ExportLogsServiceRequest
	if err := json.Unmarshal(data, &req); err != nil {
		return 0, fmt.Errorf("cannot unmarshal request from %d bytes: %w", len(data), err)
	}
	return req.pushFields(cp), nil
}
