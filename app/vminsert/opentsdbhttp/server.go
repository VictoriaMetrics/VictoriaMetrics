package opentsdbhttp

import (
	"fmt"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/metrics"
	"github.com/valyala/fasthttp"
)

var (
	writeRequests = metrics.NewCounter(`vm_http_requests_total{path="/api/put", protocol="opentsdb-http"}`)
	writeErrors   = metrics.NewCounter(`vm_http_request_errors_total{path="/api/put", protocol="opentsdb-http"}`)
)

var (
	httpServer fasthttp.Server
	httpAddr string
)


// Serve starts OpenTSDB collector on the given addr.
func Serve(addr string, maxSize int64) {
	logger.Infof("starting HTTP OpenTSDB collector at %q", addr)
	httpAddr = addr
	httpServer = fasthttp.Server{
		MaxRequestBodySize: int(maxSize),
		Handler: func(req *fasthttp.RequestCtx) {
			if req.IsPost() && (ob2s(req.Path()) == "/api/put") {
				writeRequests.Inc()
				err := InsertHandler(req)
				if err != nil {
					writeErrors.Inc()
					_, _ = fmt.Fprintf(req, "error in %q: %s", req.URI().Path(), err)
					req.Response.SetStatusCode(fasthttp.StatusInternalServerError)
				} else {
					req.Response.SetStatusCode(fasthttp.StatusOK)
				}
			}
		},
	}


	if err := httpServer.ListenAndServe(addr); err != nil {
		logger.Fatalf("cannot start HTTP OpenTSDB collector at %q: %s", addr, err)
	}
}



// Stop stops the server.
func Stop() {
	logger.Infof("stopping TCP OpenTSDB server at %q...", httpAddr)
	if err := httpServer.Shutdown(); err != nil {
		logger.Errorf("cannot close HTTP OpenTSDB server: %s", err)
	}
}
