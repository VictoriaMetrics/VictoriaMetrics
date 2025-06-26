package loki

import (
	"flag"
	"fmt"
	"net/http"
	"strconv"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vlinsert/insertutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httputil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logstorage"
)

var disableMessageParsing = flag.Bool("loki.disableMessageParsing", false, "Whether to disable automatic parsing of JSON-encoded log fields inside Loki log message into distinct log fields")

// RequestHandler processes Loki insert requests
func RequestHandler(path string, w http.ResponseWriter, r *http.Request) bool {
	switch path {
	case "/insert/loki/api/v1/push":
		handleInsert(r, w)
		return true
	case "/insert/loki/ready":
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

type commonParams struct {
	cp *insertutil.CommonParams

	// Whether to parse JSON inside plaintext log message.
	//
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/8486
	parseMessage bool
}

func getCommonParams(r *http.Request) (*commonParams, error) {
	cp, err := insertutil.GetCommonParams(r)
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

	parseMessage := !*disableMessageParsing
	if rv := httputil.GetRequestValue(r, "disable_message_parsing", "VL-Loki-Disable-Message-Parsing"); rv != "" {
		bv, err := strconv.ParseBool(rv)
		if err != nil {
			return nil, fmt.Errorf("cannot parse dusable_message_parsing=%q: %s", rv, err)
		}
		parseMessage = !bv
	}

	return &commonParams{
		cp:           cp,
		parseMessage: parseMessage,
	}, nil
}
