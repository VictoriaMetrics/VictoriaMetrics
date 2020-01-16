package opentsdbhttp

import (
	"context"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/netutil"
	"github.com/VictoriaMetrics/metrics"
)

var (
	writeRequests = metrics.NewCounter(`vm_http_requests_total{path="/api/put", protocol="opentsdb-http"}`)
	writeErrors   = metrics.NewCounter(`vm_http_request_errors_total{path="/api/put", protocol="opentsdb-http"}`)
)

// Server represents HTTP OpenTSDB server.
type Server struct {
	s  *http.Server
	ln net.Listener
	wg sync.WaitGroup
}

// MustStart starts HTTP OpenTSDB server on the given addr.
//
// MustStop must be called on the returned server when it is no longer needed.
func MustStart(addr string) *Server {
	logger.Infof("starting HTTP OpenTSDB server at %q", addr)
	lnTCP, err := netutil.NewTCPListener("opentsdbhttp", addr)
	if err != nil {
		logger.Fatalf("cannot start HTTP OpenTSDB collector at %q: %s", addr, err)
	}
	return MustServe(lnTCP)
}

// MustServe serves OpenTSDB HTTP put requests from ln.
//
// MustStop must be called on the returned server when it is no longer needed.
func MustServe(ln net.Listener) *Server {
	h := newRequestHandler()
	hs := &http.Server{
		Handler:      h,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
	s := &Server{
		s:  hs,
		ln: ln,
	}
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		err := s.s.Serve(s.ln)
		if err == http.ErrServerClosed {
			return
		}
		if err != nil {
			logger.Fatalf("error serving HTTP OpenTSDB at %q: %s", s.ln.Addr(), err)
		}
	}()
	return s
}

// Wait waits until the server is stopped with MustStop.
func (s *Server) Wait() {
	s.wg.Wait()
}

// MustStop stops HTTP OpenTSDB server.
func (s *Server) MustStop() {
	logger.Infof("stopping HTTP OpenTSDB server at %q...", s.ln.Addr())
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.s.Shutdown(ctx); err != nil {
		logger.Fatalf("cannot close HTTP OpenTSDB server at %q: %s", s.ln.Addr(), err)
	}
	s.wg.Wait()
	logger.Infof("OpenTSDB HTTP server at %q has been stopped", s.ln.Addr())
}

func newRequestHandler() http.Handler {
	rh := func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/put":
			writeRequests.Inc()
			if err := insertHandler(r); err != nil {
				writeErrors.Inc()
				httpserver.Errorf(w, "error in %q: %s", r.URL.Path, err)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		default:
			httpserver.Errorf(w, "unexpected path requested on HTTP OpenTSDB server: %q", r.URL.Path)
		}
	}
	return http.HandlerFunc(rh)
}
