package vlinsert

import (
	"net/http"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vlinsert/elasticsearch"
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
		return false
	}
	path = strings.TrimPrefix(path, "/insert")
	path = strings.ReplaceAll(path, "//", "/")

	switch {
	case strings.HasPrefix(path, "/elasticsearch/"):
		path = strings.TrimPrefix(path, "/elasticsearch")
		return elasticsearch.RequestHandler(path, w, r)
	default:
		return false
	}
}
