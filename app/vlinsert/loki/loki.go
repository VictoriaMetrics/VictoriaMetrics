package loki

import (
	"net/http"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vlinsert/insertutils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logstorage"
	"github.com/VictoriaMetrics/metrics"
)

var (
	lokiRequestsJSONTotal     = metrics.NewCounter(`vl_http_requests_total{path="/insert/loki/api/v1/push",format="json"}`)
	lokiRequestsProtobufTotal = metrics.NewCounter(`vl_http_requests_total{path="/insert/loki/api/v1/push",format="protobuf"}`)
)

// RequestHandler processes Loki insert requests
//
// See https://grafana.com/docs/loki/latest/api/#push-log-entries-to-loki
func RequestHandler(path string, w http.ResponseWriter, r *http.Request) bool {
	if path != "/api/v1/push" {
		return false
	}
	contentType := r.Header.Get("Content-Type")
	switch contentType {
	case "application/json":
		lokiRequestsJSONTotal.Inc()
		return handleJSON(r, w)
	default:
		// Protobuf request body should be handled by default accoring to https://grafana.com/docs/loki/latest/api/#push-log-entries-to-loki
		lokiRequestsProtobufTotal.Inc()
		return handleProtobuf(r, w)
	}
}

func getCommonParams(r *http.Request) (*insertutils.CommonParams, error) {
	cp, err := insertutils.GetCommonParams(r)
	if err != nil {
		return nil, err
	}

	// If parsed tenant is (0,0) it is likely to be default tenant
	// Try parsing tenant from Loki headers
	if cp.TenantID.AccountID == 0 && cp.TenantID.ProjectID == 0 {
		org := r.Header.Get("X-Scope-OrgID")
		if org != "" {
			tenantID, err := logstorage.GetTenantIDFromString(org)
			if err != nil {
				return nil, err
			}
			cp.TenantID = tenantID
		}

	}

	return cp, nil
}
