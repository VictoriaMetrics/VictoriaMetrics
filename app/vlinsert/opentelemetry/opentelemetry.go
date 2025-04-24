package opentelemetry

import (
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vlinsert/opentelemetry/logs"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vlinsert/opentelemetry/traces"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"net/http"
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
		logs.HandleProtobuf(r, w)
		return true
	// use the same path as opentelemetry collector
	// https://opentelemetry.io/docs/specs/otlp/#otlphttp-request
	case "/v1/traces":
		if r.Header.Get("Content-Type") == "application/json" {
			httpserver.Errorf(w, r, "json encoding isn't supported for opentelemetry format. Use protobuf encoding")
			return true
		}
		traces.HandleProtobuf(r, w)
		return true
	default:
		return false
	}
}
