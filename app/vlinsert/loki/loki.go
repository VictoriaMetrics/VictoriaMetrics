package loki

import (
	"net/http"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vlinsert/insertutils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logstorage"
)

// RequestHandler processes Loki insert requests
func RequestHandler(path string, w http.ResponseWriter, r *http.Request) bool {
	switch path {
	case "/api/v1/push":
		handleInsert(r, w)
		return true
	case "/ready":
		// See https://grafana.com/docs/loki/latest/api/#identify-ready-loki-instance
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ready"))
		return true
	default:
		return false
	}
}

// See https://grafana.com/docs/loki/latest/api/#push-log-entries-to-loki
func handleInsert(r *http.Request, w http.ResponseWriter) {
	contentType := r.Header.Get("Content-Type")
	switch contentType {
	case "application/json":
		handleJSON(r, w)
	default:
		// Protobuf request body should be handled by default according to https://grafana.com/docs/loki/latest/api/#push-log-entries-to-loki
		handleProtobuf(r, w)
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
			tenantID, err := logstorage.ParseTenantID(org)
			if err != nil {
				return nil, err
			}
			cp.TenantID = tenantID
		}

	}

	return cp, nil
}
