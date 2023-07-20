package vlinsert

import (
	"net/http"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vlinsert/elasticsearch"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vlinsert/jsonline"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vlinsert/loki"
)

// Init initializes vlinsert
func Init() {
}

// Stop stops vlinsert
func Stop() {
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
		return jsonline.RequestHandler(w, r)
	}
	switch {
	case strings.HasPrefix(path, "/elasticsearch/"):
		path = strings.TrimPrefix(path, "/elasticsearch")
		return elasticsearch.RequestHandler(path, w, r)
	case strings.HasPrefix(path, "/loki/"):
		path = strings.TrimPrefix(path, "/loki")
		return loki.RequestHandler(path, w, r)
	default:
		return false
	}
}
