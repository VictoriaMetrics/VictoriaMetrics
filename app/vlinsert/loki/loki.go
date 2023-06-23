package loki

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

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
		tenantID, err := getTenantIDFromRequest(r)
		if err != nil {
			return nil, err
		}
		cp.TenantID = tenantID
	}

	return cp, nil
}

// Parses TenantID from request based on Loki X-Scope-OrgID header
func getTenantIDFromRequest(r *http.Request) (logstorage.TenantID, error) {
	var tenantID logstorage.TenantID

	org := r.Header.Get("X-Scope-OrgID")
	if org == "" {
		// Return empty tenantID
		return logstorage.TenantID{}, nil
	}

	colon := strings.Index(org, ":")
	if colon < 0 {
		account, err := getUint32FromString(org)
		if err != nil {
			return tenantID, fmt.Errorf("cannot parse X-Scope-OrgID=%q: %w", org, err)
		}
		tenantID.AccountID = account

		return tenantID, nil
	}

	account, err := getUint32FromString(org[:colon])
	if err != nil {
		return tenantID, fmt.Errorf("cannot parse X-Scope-OrgID=%q: %w", org, err)
	}
	tenantID.AccountID = account

	project, err := getUint32FromString(org[colon+1:])
	if err != nil {
		return tenantID, fmt.Errorf("cannot parse X-Scope-OrgID=%q: %w", org, err)
	}
	tenantID.ProjectID = project

	return tenantID, nil
}

func getUint32FromString(s string) (uint32, error) {
	if len(s) == 0 {
		return 0, nil
	}
	n, err := strconv.ParseUint(s, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("cannot parse %q as uint32: %w", s, err)
	}
	return uint32(n), nil
}
