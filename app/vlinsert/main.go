package vlinsert

import (
	"flag"
	"fmt"
	"net/http"
	"strings"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vlinsert/datadog"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vlinsert/elasticsearch"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vlinsert/internalinsert"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vlinsert/journald"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vlinsert/jsonline"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vlinsert/loki"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vlinsert/opentelemetry"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vlinsert/syslog"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
)

var (
	disableInsert   = flag.Bool("insert.disable", false, "Whether to disable /insert/* HTTP endpoints")
	disableInternal = flag.Bool("internalinsert.disable", false, "Whether to disable /internal/insert HTTP endpoint")
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

	if strings.HasPrefix(path, "/insert/") {
		if *disableInsert {
			httpserver.Errorf(w, r, "requests to /insert/* are disabled with -insert.disable command-line flag")
			return true
		}

		return insertHandler(w, r, path)
	}

	if path == "/internal/insert" {
		if *disableInternal || *disableInsert {
			httpserver.Errorf(w, r, "requests to /internal/insert are disabled with -internalinsert.disable or -insert.disable command-line flag")
			return true
		}
		internalinsert.RequestHandler(w, r)
		return true
	}

	return false
}

func insertHandler(w http.ResponseWriter, r *http.Request, path string) bool {
	path = strings.TrimPrefix(path, "/insert")
	path = strings.ReplaceAll(path, "//", "/")

	switch path {
	case "/jsonline":
		jsonline.RequestHandler(w, r)
		return true
	case "/ready":
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		fmt.Fprintf(w, `{"status":"ok"}`)
		return true
	}
	switch {
	case strings.HasPrefix(path, "/elasticsearch"):
		// some clients may omit trailing slash
		// see https://github.com/VictoriaMetrics/VictoriaMetrics/issues/8353
		path = strings.TrimPrefix(path, "/elasticsearch")
		return elasticsearch.RequestHandler(path, w, r)
	case strings.HasPrefix(path, "/loki/"):
		path = strings.TrimPrefix(path, "/loki")
		return loki.RequestHandler(path, w, r)
	case strings.HasPrefix(path, "/opentelemetry/"):
		path = strings.TrimPrefix(path, "/opentelemetry")
		return opentelemetry.RequestHandler(path, w, r)
	case strings.HasPrefix(path, "/journald/"):
		path = strings.TrimPrefix(path, "/journald")
		return journald.RequestHandler(path, w, r)
	case strings.HasPrefix(path, "/datadog/"):
		path = strings.TrimPrefix(path, "/datadog")
		return datadog.RequestHandler(path, w, r)
	}

	return false
}
