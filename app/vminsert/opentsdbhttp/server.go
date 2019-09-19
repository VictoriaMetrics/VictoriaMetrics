package opentsdbhttp

import (
	"context"
	"net/http"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/metrics"
)

var (
	writeRequests = metrics.NewCounter(`vm_http_requests_total{path="/api/put", protocol="opentsdb-http"}`)
	writeErrors   = metrics.NewCounter(`vm_http_request_errors_total{path="/api/put", protocol="opentsdb-http"}`)
)

var (
	httpServer     *http.Server
	httpAddr       string
	maxRequestSize int64
)

// Serve starts HTTP OpenTSDB server on the given addr.
func Serve(addr string, maxReqSize int64) {
	logger.Infof("starting HTTP OpenTSDB server at %q", addr)
	httpAddr = addr
	maxRequestSize = maxReqSize
	httpServer = &http.Server{
		Addr:         addr,
		Handler:      http.HandlerFunc(requestHandler),
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
	go func() {
		err := httpServer.ListenAndServe()
		if err == http.ErrServerClosed {
			return
		}
		if err != nil {
			logger.Fatalf("error serving HTTP OpenTSDB: %s", err)
		}
	}()
}

// requestHandler handles HTTP OpenTSDB insert request.
func requestHandler(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/api/put":
		writeRequests.Inc()
		if err := insertHandler(r, maxRequestSize); err != nil {
			writeErrors.Inc()
			httpserver.Errorf(w, "error in %q: %s", r.URL.Path, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		httpserver.Errorf(w, "unexpected path requested on HTTP OpenTSDB server: %q", r.URL.Path)
	}
}

// Stop stops HTTP OpenTSDB server.
func Stop() {
	logger.Infof("stopping HTTP OpenTSDB server at %q...", httpAddr)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(ctx); err != nil {
		logger.Fatalf("cannot close HTTP OpenTSDB server: %s", err)
	}
}
