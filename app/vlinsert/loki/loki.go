package loki

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vlstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httputils"
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

func getLogRows(r *http.Request) *logstorage.LogRows {
	var streamFields []string
	if sfs := r.FormValue("_stream_fields"); sfs != "" {
		streamFields = strings.Split(sfs, ",")
	}

	// Extract field names, which must be ignored
	var ignoreFields []string
	if ifs := r.FormValue("ignore_fields"); ifs != "" {
		ignoreFields = strings.Split(ifs, ",")
	}
	lr := logstorage.GetLogRows(streamFields, ignoreFields)
	return lr
}

func getLogMessageHandler(r *http.Request, tenantID logstorage.TenantID, lr *logstorage.LogRows) func(timestamp int64, fields []logstorage.Field) {
	isDebug := httputils.GetBool(r, "debug")
	debugRequestURI := ""
	debugRemoteAddr := ""
	if isDebug {
		debugRequestURI = httpserver.GetRequestURI(r)
		debugRemoteAddr = httpserver.GetQuotedRemoteAddr(r)
	}

	return func(timestamp int64, fields []logstorage.Field) {
		lr.MustAdd(tenantID, timestamp, fields)
		if isDebug {
			s := lr.GetRowString(0)
			lr.ResetKeepSettings()
			logger.Infof("remoteAddr=%s; requestURI=%s; ignoring log entry because of `debug` query arg: %s", debugRemoteAddr, debugRequestURI, s)
			return
		}
		if lr.NeedFlush() {
			vlstorage.MustAddRows(lr)
			lr.ResetKeepSettings()
		}
	}
}

func getTenantIDFromRequest(r *http.Request) (logstorage.TenantID, error) {
	var tenantID logstorage.TenantID

	org := r.Header.Get("X-Scope-OrgID")
	if org == "" {
		// Rollback to default multi-tenancy headers
		return logstorage.GetTenantIDFromRequest(r)
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
