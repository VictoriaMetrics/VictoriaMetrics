package vlinsert

import (
	"net/http"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vlinsert/elasticsearch"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vlinsert/jsonline"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vlinsert/loki"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vlinsert/opentelemetry"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vlinsert/syslog"
)

// Init initializes vlinsert
func Init() {
	syslog.MustInit()
}

// Stop stops vlinsert
func Stop() {
	syslog.MustStop()
}

// RequestHandler handles insert requests for VictoriaLogs
func RequestHandler(w http.ResponseWriter, r *http.Request) bool {
	path := r.URL.Path
	if !strings.HasPrefix(path, "/insert/") {
		// Skip requests, which do not start with /insert/, since these aren't our requests.
		return false
	}
	path = strings.TrimPrefix(path, "/insert")
	path = strings.ReplaceAll(path, "//", "/")

	if path == "/jsonline" {
		jsonline.RequestHandler(w, r)
		return true
	}
	switch {
	case strings.HasPrefix(path, "/elasticsearch/"):
		path = strings.TrimPrefix(path, "/elasticsearch")
		return elasticsearch.RequestHandler(path, w, r)
	case strings.HasPrefix(path, "/loki/"):
		path = strings.TrimPrefix(path, "/loki")
		return loki.RequestHandler(path, w, r)
	case strings.HasPrefix(path, "/opentelemetry/"):
		path = strings.TrimPrefix(path, "/opentelemetry")
		return opentelemetry.RequestHandler(path, w, r)
	default:
		return false
	}
}
