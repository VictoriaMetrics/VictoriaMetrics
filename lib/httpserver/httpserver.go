package httpserver

import (
	"bufio"
	"compress/gzip"
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/pprof"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/netutil"
	"github.com/VictoriaMetrics/metrics"
)

var (
	disableResponseCompression = flag.Bool("http.disableResponseCompression", false, "Disable compression of HTTP responses for saving CPU resources. By default compression is enabled to save network bandwidth")

	servers     = make(map[string]*http.Server)
	serversLock sync.Mutex
)

// RequestHandler must serve the given request r and write response to w.
//
// RequestHandler must return true if the request has been served (successfully or not).
//
// RequestHandler must return false if it cannot serve the given request.
// In such cases the caller must serve the request.
type RequestHandler func(w http.ResponseWriter, r *http.Request) bool

// Serve starts an http server on the given addr with the given rh.
//
// By default all the responses are transparently compressed, since Google
// charges a lot for the egress traffic. The compression may be disabled
// by calling DisableResponseCompression before writing the first byte to w.
//
// The compression is also disabled if -http.disableResponseCompression flag is set.
func Serve(addr string, rh RequestHandler) {
	logger.Infof("starting http server at http://%s/", addr)
	logger.Infof("pprof handlers are exposed at http://%s/debug/pprof/", addr)
	ln, err := netutil.NewTCPListener("http", addr)
	if err != nil {
		logger.Panicf("FATAL: cannot start http server at %s: %s", addr, err)
	}
	setNetworkTimeouts(ln)
	serveWithListener(addr, ln, rh)
}

func setNetworkTimeouts(ln *netutil.TCPListener) {
	// Set network-level read and write timeouts to reasonable values
	// in order to protect from DoS or broken networks.
	// Application-level timeouts must be set by the authors of request handlers.
	//
	// The read timeout limits the life of idle connection
	ln.ReadTimeout = time.Minute
	ln.WriteTimeout = time.Minute
}

func serveWithListener(addr string, ln net.Listener, rh RequestHandler) {
	s := &http.Server{
		Handler: gzipHandler(rh),

		// Disable http/2
		TLSNextProto: make(map[string]func(*http.Server, *tls.Conn, http.Handler)),

		// Do not set ReadTimeout and WriteTimeout here.
		// Network-level timeouts are set in setNetworkTimeouts.
		// Application-level timeouts must be set in the app.

		// Do not set IdleTimeout, since it is equivalent to read timeout
		// set in setNetworkTimeouts.

		ErrorLog: logger.StdErrorLogger(),
	}
	serversLock.Lock()
	servers[addr] = s
	serversLock.Unlock()
	if err := s.Serve(ln); err != nil {
		if err == http.ErrServerClosed {
			// The server gracefully closed.
			return
		}
		logger.Panicf("FATAL: cannot serve http at %s: %s", addr, err)
	}
}

// Stop stops the http server on the given addr, which has been started
// via Serve func.
func Stop(addr string) error {
	serversLock.Lock()
	s := servers[addr]
	delete(servers, addr)
	serversLock.Unlock()
	if s == nil {
		logger.Panicf("BUG: there is no http server at %q", addr)
	}
	ctx, cancelFunc := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelFunc()
	if err := s.Shutdown(ctx); err != nil {
		return fmt.Errorf("cannot gracefully shutdown http server at %q: %s", addr, err)
	}
	return nil
}

func gzipHandler(rh RequestHandler) http.HandlerFunc {
	hf := func(w http.ResponseWriter, r *http.Request) {
		w = maybeGzipResponseWriter(w, r)
		handlerWrapper(w, r, rh)
		if zrw, ok := w.(*gzipResponseWriter); ok {
			if err := zrw.Close(); err != nil && !isTrivialNetworkError(err) {
				logger.Errorf("gzipResponseWriter.Close: %s", err)
			}
		}
	}
	return http.HandlerFunc(hf)
}

var metricsHandlerDuration = metrics.NewHistogram(`vm_http_request_duration_seconds{path="/metrics"}`)

func handlerWrapper(w http.ResponseWriter, r *http.Request, rh RequestHandler) {
	requestsTotal.Inc()
	switch r.URL.Path {
	case "/health":
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte("OK"))
		return
	case "/metrics":
		startTime := time.Now()
		metricsRequests.Inc()
		w.Header().Set("Content-Type", "text/plain")
		writePrometheusMetrics(w)
		metricsHandlerDuration.UpdateDuration(startTime)
		return
	case "/favicon.ico":
		faviconRequests.Inc()
		w.WriteHeader(http.StatusNoContent)
		return
	default:
		if strings.HasPrefix(r.URL.Path, "/debug/pprof/") {
			pprofRequests.Inc()
			DisableResponseCompression(w)
			pprofHandler(r.URL.Path[len("/debug/pprof/"):], w, r)
			return
		}
		if rh(w, r) {
			return
		}

		Errorf(w, "unsupported path requested: %q", r.URL.Path)
		unsupportedRequestErrors.Inc()
		return
	}
}

func maybeGzipResponseWriter(w http.ResponseWriter, r *http.Request) http.ResponseWriter {
	if *disableResponseCompression {
		return w
	}
	ae := r.Header.Get("Accept-Encoding")
	if ae == "" {
		return w
	}
	ae = strings.ToLower(ae)
	n := strings.Index(ae, "gzip")
	if n < 0 {
		return w
	}
	h := w.Header()
	h.Set("Content-Encoding", "gzip")
	zw := getGzipWriter(w)
	bw := getBufioWriter(zw)
	zrw := &gzipResponseWriter{
		ResponseWriter: w,
		zw:             zw,
		bw:             bw,
	}
	return zrw
}

// DisableResponseCompression disables response compression on w.
//
// The function must be called before the first w.Write* call.
func DisableResponseCompression(w http.ResponseWriter) {
	w.Header().Del("Content-Encoding")
}

// EnableCORS enables https://developer.mozilla.org/en-US/docs/Web/HTTP/CORS
// on the response.
func EnableCORS(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
}

func getGzipWriter(w io.Writer) *gzip.Writer {
	v := gzipWriterPool.Get()
	if v == nil {
		zw, err := gzip.NewWriterLevel(w, 1)
		if err != nil {
			logger.Panicf("BUG: cannot create gzip writer: %s", err)
		}
		return zw
	}
	zw := v.(*gzip.Writer)
	zw.Reset(w)
	return zw
}

func putGzipWriter(zw *gzip.Writer) {
	gzipWriterPool.Put(zw)
}

var gzipWriterPool sync.Pool

type gzipResponseWriter struct {
	http.ResponseWriter
	zw         *gzip.Writer
	bw         *bufio.Writer
	statusCode int

	firstWriteDone     bool
	disableCompression bool
}

func (zrw *gzipResponseWriter) Write(p []byte) (int, error) {
	if !zrw.firstWriteDone {
		h := zrw.Header()
		if h.Get("Content-Encoding") != "gzip" {
			// The request handler disabled gzip encoding.
			// Send uncompressed response body.
			zrw.disableCompression = true
		} else if h.Get("Content-Type") == "" {
			// Disable auto-detection of content-type, since it
			// is incorrectly detected after the compression.
			h.Set("Content-Type", "text/html")
		}
		zrw.firstWriteDone = true
	}
	if zrw.statusCode == 0 {
		zrw.WriteHeader(http.StatusOK)
	}
	if zrw.disableCompression {
		return zrw.ResponseWriter.Write(p)
	}
	return zrw.bw.Write(p)
}

func (zrw *gzipResponseWriter) WriteHeader(statusCode int) {
	if zrw.statusCode != 0 {
		return
	}
	if statusCode == http.StatusNoContent {
		DisableResponseCompression(zrw.ResponseWriter)
	}
	zrw.ResponseWriter.WriteHeader(statusCode)
	zrw.statusCode = statusCode
}

// Implements http.Flusher
func (zrw *gzipResponseWriter) Flush() {
	if err := zrw.bw.Flush(); err != nil && !isTrivialNetworkError(err) {
		logger.Errorf("gzipResponseWriter.Flush (buffer): %s", err)
	}
	if err := zrw.zw.Flush(); err != nil && !isTrivialNetworkError(err) {
		logger.Errorf("gzipResponseWriter.Flush (gzip): %s", err)
	}
	if fw, ok := zrw.ResponseWriter.(http.Flusher); ok {
		fw.Flush()
	}
}

func (zrw *gzipResponseWriter) Close() error {
	if !zrw.firstWriteDone {
		zrw.Header().Del("Content-Encoding")
		return nil
	}
	zrw.Flush()
	err := zrw.zw.Close()
	putGzipWriter(zrw.zw)
	zrw.zw = nil
	putBufioWriter(zrw.bw)
	zrw.bw = nil
	return err
}

func getBufioWriter(w io.Writer) *bufio.Writer {
	v := bufioWriterPool.Get()
	if v == nil {
		return bufio.NewWriterSize(w, 16*1024)
	}
	bw := v.(*bufio.Writer)
	bw.Reset(w)
	return bw
}

func putBufioWriter(bw *bufio.Writer) {
	bufioWriterPool.Put(bw)
}

var bufioWriterPool sync.Pool

func pprofHandler(profileName string, w http.ResponseWriter, r *http.Request) {
	// This switch has been stolen from init func at https://golang.org/src/net/http/pprof/pprof.go
	switch profileName {
	case "cmdline":
		pprofCmdlineRequests.Inc()
		pprof.Cmdline(w, r)
	case "profile":
		pprofProfileRequests.Inc()
		pprof.Profile(w, r)
	case "symbol":
		pprofSymbolRequests.Inc()
		pprof.Symbol(w, r)
	case "trace":
		pprofTraceRequests.Inc()
		pprof.Trace(w, r)
	case "mutex":
		pprofMutexRequests.Inc()
		seconds, _ := strconv.Atoi(r.FormValue("seconds"))
		if seconds <= 0 {
			seconds = 10
		}
		prev := runtime.SetMutexProfileFraction(10)
		time.Sleep(time.Duration(seconds) * time.Second)
		pprof.Index(w, r)
		runtime.SetMutexProfileFraction(prev)
	default:
		pprofDefaultRequests.Inc()
		pprof.Index(w, r)
	}
}

var (
	metricsRequests      = metrics.NewCounter(`vm_http_requests_total{path="/metrics"}`)
	pprofRequests        = metrics.NewCounter(`vm_http_requests_total{path="/debug/pprof/"}`)
	pprofCmdlineRequests = metrics.NewCounter(`vm_http_requests_total{path="/debug/pprof/cmdline"}`)
	pprofProfileRequests = metrics.NewCounter(`vm_http_requests_total{path="/debug/pprof/profile"}`)
	pprofSymbolRequests  = metrics.NewCounter(`vm_http_requests_total{path="/debug/pprof/symbol"}`)
	pprofTraceRequests   = metrics.NewCounter(`vm_http_requests_total{path="/debug/pprof/trace"}`)
	pprofMutexRequests   = metrics.NewCounter(`vm_http_requests_total{path="/debug/pprof/mutex"}`)
	pprofDefaultRequests = metrics.NewCounter(`vm_http_requests_total{path="/debug/pprof/default"}`)
	faviconRequests      = metrics.NewCounter(`vm_http_requests_total{path="/favicon.ico"}`)

	unsupportedRequestErrors = metrics.NewCounter(`vm_http_request_errors_total{path="*", reason="unsupported"}`)

	requestsTotal = metrics.NewCounter(`vm_http_requests_all_total`)
)

// Errorf writes formatted error message to w and to logger.
func Errorf(w http.ResponseWriter, format string, args ...interface{}) {
	errStr := fmt.Sprintf(format, args...)
	logger.Errorf("%s", errStr)

	// Extract statusCode from args
	statusCode := http.StatusBadRequest
	for _, arg := range args {
		if esc, ok := arg.(*ErrorWithStatusCode); ok {
			statusCode = esc.StatusCode
			break
		}
	}
	http.Error(w, errStr, statusCode)
}

// ErrorWithStatusCode is error with HTTP status code.
//
// The given StatusCode is sent to client when the error is passed to Errorf.
type ErrorWithStatusCode struct {
	Err        error
	StatusCode int
}

// Error implements error interface.
func (e *ErrorWithStatusCode) Error() string {
	return e.Err.Error()
}

func isTrivialNetworkError(err error) bool {
	s := err.Error()
	if strings.Contains(s, "broken pipe") || strings.Contains(s, "reset by peer") {
		return true
	}
	return false
}
