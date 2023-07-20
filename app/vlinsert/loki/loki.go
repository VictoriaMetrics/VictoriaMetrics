package loki

import (
	"net/http"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vlinsert/insertutils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logstorage"
	"github.com/VictoriaMetrics/metrics"
)

const msgField = "_msg"

var (
	lokiRequestsTotal = metrics.NewCounter(`vl_http_requests_total{path="/insert/loki/api/v1/push"}`)
)

// RequestHandler processes ElasticSearch insert requests
func RequestHandler(path string, w http.ResponseWriter, r *http.Request) bool {
	switch path {
	case "/api/v1/push":
		contentType := r.Header.Get("Content-Type")
		lokiRequestsTotal.Inc()
		switch contentType {
		case "application/x-protobuf":
			return handleProtobuf(r, w)
		case "application/json", "gzip":
			return handleJSON(r, w)
		default:
			logger.Warnf("unsupported Content-Type=%q for %q request; skipping it", contentType, path)
			return false
		}
	default:
		return false
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
