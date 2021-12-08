package httpserver

import (
	"bufio"
	"context"
	"crypto/tls"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/pprof"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/netutil"
	"github.com/VictoriaMetrics/metrics"
	"github.com/klauspost/compress/gzip"
	"github.com/valyala/fastrand"
)

var (
	tlsEnable   = flag.Bool("tls", false, "Whether to enable TLS (aka HTTPS) for incoming requests. -tlsCertFile and -tlsKeyFile must be set if -tls is set")
	tlsCertFile = flag.String("tlsCertFile", "", "Path to file with TLS certificate. Used only if -tls is set. Prefer ECDSA certs instead of RSA certs as RSA certs are slower")
	tlsKeyFile  = flag.String("tlsKeyFile", "", "Path to file with TLS key. Used only if -tls is set")

	pathPrefix = flag.String("http.pathPrefix", "", "An optional prefix to add to all the paths handled by http server. For example, if '-http.pathPrefix=/foo/bar' is set, "+
		"then all the http requests will be handled on '/foo/bar/*' paths. This may be useful for proxied requests. "+
		"See https://www.robustperception.io/using-external-urls-and-proxies-with-prometheus")

	disableResponseCompression  = flag.Bool("http.disableResponseCompression", false, "Disable compression of HTTP responses to save CPU resources. By default compression is enabled to save network bandwidth")
	maxGracefulShutdownDuration = flag.Duration("http.maxGracefulShutdownDuration", 7*time.Second, `The maximum duration for a graceful shutdown of the HTTP server. A highly loaded server may require increased value for a graceful shutdown`)
	shutdownDelay               = flag.Duration("http.shutdownDelay", 0, `Optional delay before http server shutdown. During this delay, the server returns non-OK responses from /health page, so load balancers can route new requests to other servers`)
	idleConnTimeout             = flag.Duration("http.idleConnTimeout", time.Minute, "Timeout for incoming idle http connections")
	connTimeout                 = flag.Duration("http.connTimeout", 2*time.Minute, `Incoming http connections are closed after the configured timeout. This may help to spread the incoming load among a cluster of services behind a load balancer. Please note that the real timeout may be bigger by up to 10% as a protection against the thundering herd problem`)
)

var (
	servers     = make(map[string]*server)
	serversLock sync.Mutex
)

type server struct {
	shutdownDelayDeadline int64
	s                     *http.Server
}

// RequestHandler must serve the given request r and write response to w.
//
// RequestHandler must return true if the request has been served (successfully or not).
//
// RequestHandler must return false if it cannot serve the given request.
// In such cases the caller must serve the request.
type RequestHandler func(w http.ResponseWriter, r *http.Request) bool

// Serve starts an http server on the given addr with the given optional rh.
//
// By default all the responses are transparently compressed, since Google
// charges a lot for the egress traffic. The compression may be disabled
// by calling DisableResponseCompression before writing the first byte to w.
//
// The compression is also disabled if -http.disableResponseCompression flag is set.
func Serve(addr string, rh RequestHandler) {
	if rh == nil {
		rh = func(w http.ResponseWriter, r *http.Request) bool {
			return false
		}
	}
	scheme := "http"
	if *tlsEnable {
		scheme = "https"
	}
	hostAddr := addr
	if strings.HasPrefix(hostAddr, ":") {
		hostAddr = "127.0.0.1" + hostAddr
	}
	logger.Infof("starting http server at %s://%s/", scheme, hostAddr)
	logger.Infof("pprof handlers are exposed at %s://%s/debug/pprof/", scheme, hostAddr)
	lnTmp, err := netutil.NewTCPListener(scheme, addr)
	if err != nil {
		logger.Fatalf("cannot start http server at %s: %s", addr, err)
	}
	ln := net.Listener(lnTmp)

	if *tlsEnable {
		cert, err := tls.LoadX509KeyPair(*tlsCertFile, *tlsKeyFile)
		if err != nil {
			logger.Fatalf("cannot load TLS cert from tlsCertFile=%q, tlsKeyFile=%q: %s", *tlsCertFile, *tlsKeyFile, err)
		}
		cfg := &tls.Config{
			Certificates:             []tls.Certificate{cert},
			MinVersion:               tls.VersionTLS12,
			PreferServerCipherSuites: true,
		}
		ln = tls.NewListener(ln, cfg)
	}
	serveWithListener(addr, ln, rh)
}

func serveWithListener(addr string, ln net.Listener, rh RequestHandler) {
	var s server
	s.s = &http.Server{
		Handler: gzipHandler(&s, rh),

		// Disable http/2, since it doesn't give any advantages for VictoriaMetrics services.
		TLSNextProto: make(map[string]func(*http.Server, *tls.Conn, http.Handler)),

		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       *idleConnTimeout,

		// Do not set ReadTimeout and WriteTimeout here,
		// since these timeouts must be controlled by request handlers.

		ErrorLog: logger.StdErrorLogger(),

		ConnContext: func(ctx context.Context, c net.Conn) context.Context {
			timeoutSec := connTimeout.Seconds()
			// Add a jitter for connection timeout in order to prevent Thundering herd problem
			// when all the connections are established at the same time.
			// See https://en.wikipedia.org/wiki/Thundering_herd_problem
			jitterSec := fastrand.Uint32n(uint32(timeoutSec / 10))
			deadline := fasttime.UnixTimestamp() + uint64(timeoutSec) + uint64(jitterSec)
			return context.WithValue(ctx, connDeadlineTimeKey, &deadline)
		},
	}
	serversLock.Lock()
	servers[addr] = &s
	serversLock.Unlock()
	if err := s.s.Serve(ln); err != nil {
		if err == http.ErrServerClosed {
			// The server gracefully closed.
			return
		}
		logger.Panicf("FATAL: cannot serve http at %s: %s", addr, err)
	}
}

func whetherToCloseConn(r *http.Request) bool {
	ctx := r.Context()
	v := ctx.Value(connDeadlineTimeKey)
	deadline, ok := v.(*uint64)
	return ok && fasttime.UnixTimestamp() > *deadline
}

var connDeadlineTimeKey = interface{}("connDeadlineSecs")

// Stop stops the http server on the given addr, which has been started
// via Serve func.
func Stop(addr string) error {
	serversLock.Lock()
	s := servers[addr]
	delete(servers, addr)
	serversLock.Unlock()
	if s == nil {
		err := fmt.Errorf("BUG: there is no http server at %q", addr)
		logger.Panicf("%s", err)
		// The return is needed for golangci-lint: SA5011(related information): this check suggests that the pointer can be nil
		return err
	}

	deadline := time.Now().Add(*shutdownDelay).UnixNano()
	atomic.StoreInt64(&s.shutdownDelayDeadline, deadline)
	if *shutdownDelay > 0 {
		// Sleep for a while until load balancer in front of the server
		// notifies that "/health" endpoint returns non-OK responses.
		// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/463 .
		logger.Infof("Waiting for %.3fs before shutdown of http server %q, so load balancers could re-route requests to other servers", shutdownDelay.Seconds(), addr)
		time.Sleep(*shutdownDelay)
		logger.Infof("Starting shutdown for http server %q", addr)
	}

	ctx, cancel := context.WithTimeout(context.Background(), *maxGracefulShutdownDuration)
	defer cancel()
	if err := s.s.Shutdown(ctx); err != nil {
		return fmt.Errorf("cannot gracefully shutdown http server at %q in %.3fs; "+
			"probably, `-http.maxGracefulShutdownDuration` command-line flag value must be increased; error: %s", addr, maxGracefulShutdownDuration.Seconds(), err)
	}
	return nil
}

func gzipHandler(s *server, rh RequestHandler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w = maybeGzipResponseWriter(w, r)
		handlerWrapper(s, w, r, rh)
		if zrw, ok := w.(*gzipResponseWriter); ok {
			if err := zrw.Close(); err != nil && !isTrivialNetworkError(err) {
				logger.Warnf("gzipResponseWriter.Close: %s", err)
			}
		}
	}
}

var metricsHandlerDuration = metrics.NewHistogram(`vm_http_request_duration_seconds{path="/metrics"}`)
var connTimeoutClosedConns = metrics.NewCounter(`vm_http_conn_timeout_closed_conns_total`)

var hostname = func() string {
	h, err := os.Hostname()
	if err != nil {
		// Cannot use logger.Errorf, since it isn't initialized yet.
		// So use log.Printf instead.
		log.Printf("ERROR: cannot determine hostname: %s", err)
		return "unknown"
	}
	return h
}()

func handlerWrapper(s *server, w http.ResponseWriter, r *http.Request, rh RequestHandler) {
	// All the VictoriaMetrics code assumes that panic stops the process.
	// Unfortunately, the standard net/http.Server recovers from panics in request handlers,
	// so VictoriaMetrics state can become inconsistent after the recovered panic.
	// The following recover() code works around this by explicitly stopping the process after logging the panic.
	// See https://github.com/golang/go/issues/16542#issuecomment-246549902 for details.
	defer func() {
		if err := recover(); err != nil {
			buf := make([]byte, 1<<20)
			n := runtime.Stack(buf, false)
			fmt.Fprintf(os.Stderr, "panic: %v\n\n%s", err, buf[:n])
			os.Exit(1)
		}
	}()

	w.Header().Add("X-Server-Hostname", hostname)
	requestsTotal.Inc()
	if whetherToCloseConn(r) {
		connTimeoutClosedConns.Inc()
		w.Header().Set("Connection", "close")
	}
	path, err := getCanonicalPath(r.URL.Path)
	if err != nil {
		Errorf(w, r, "cannot get canonical path: %s", err)
		unsupportedRequestErrors.Inc()
		return
	}
	r.URL.Path = path
	switch r.URL.Path {
	case "/health":
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		deadline := atomic.LoadInt64(&s.shutdownDelayDeadline)
		if deadline <= 0 {
			w.Write([]byte("OK"))
			return
		}
		// Return non-OK response during grace period before shutting down the server.
		// Load balancers must notify these responses and re-route new requests to other servers.
		// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/463 .
		d := time.Until(time.Unix(0, deadline))
		if d < 0 {
			d = 0
		}
		errMsg := fmt.Sprintf("The server is in delayed shutdown mode, which will end in %.3fs", d.Seconds())
		http.Error(w, errMsg, http.StatusServiceUnavailable)
		return
	case "/ping":
		// This is needed for compatibility with InfluxDB agents.
		// See https://docs.influxdata.com/influxdb/v1.7/tools/api/#ping-http-endpoint
		status := http.StatusNoContent
		if verbose := r.FormValue("verbose"); verbose == "true" {
			status = http.StatusOK
		}
		w.WriteHeader(status)
		return
	case "/favicon.ico":
		faviconRequests.Inc()
		w.WriteHeader(http.StatusNoContent)
		return
	case "/metrics":
		metricsRequests.Inc()
		startTime := time.Now()
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		WritePrometheusMetrics(w)
		metricsHandlerDuration.UpdateDuration(startTime)
		return
	case "/flags":
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		flagutil.WriteFlags(w)
		return
	case "/-/healthy":
		// This is needed for Prometheus compatibility
		// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1833
		fmt.Fprintf(w, "VictoriaMetrics is Healthy.\n")
		return
	case "/-/ready":
		// This is needed for Prometheus compatibility
		// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1833
		fmt.Fprintf(w, "VictoriaMetrics is Ready.\n")
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

		Errorf(w, r, "unsupported path requested: %q", r.URL.Path)
		unsupportedRequestErrors.Inc()
		return
	}
}

func getCanonicalPath(path string) (string, error) {
	if len(*pathPrefix) == 0 || path == "/" {
		return path, nil
	}
	if *pathPrefix == path {
		return "/", nil
	}
	prefix := *pathPrefix
	if !strings.HasSuffix(prefix, "/") {
		prefix = prefix + "/"
	}
	if !strings.HasPrefix(path, prefix) {
		return "", fmt.Errorf("missing `-pathPrefix=%q` in the requested path: %q", *pathPrefix, path)
	}
	path = path[len(prefix)-1:]
	return path, nil
}

func maybeGzipResponseWriter(w http.ResponseWriter, r *http.Request) http.ResponseWriter {
	if *disableResponseCompression {
		return w
	}
	if r.Header.Get("Connection") == "Upgrade" {
		return w
	}
	ae := r.Header.Get("Accept-Encoding")
	if ae == "" {
		return w
	}
	ae = strings.ToLower(ae)
	n := strings.Index(ae, "gzip")
	if n < 0 {
		// Do not apply gzip encoding to the response.
		return w
	}
	// Apply gzip encoding to the response.
	zw := getGzipWriter(w)
	bw := getBufioWriter(zw)
	zrw := &gzipResponseWriter{
		rw: w,
		zw: zw,
		bw: bw,
	}
	return zrw
}

// DisableResponseCompression disables response compression on w.
//
// The function must be called before the first w.Write* call.
func DisableResponseCompression(w http.ResponseWriter) {
	zrw, ok := w.(*gzipResponseWriter)
	if !ok {
		return
	}
	if zrw.firstWriteDone {
		logger.Panicf("BUG: DisableResponseCompression must be called before sending the response")
	}
	zrw.disableCompression = true
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
	rw         http.ResponseWriter
	zw         *gzip.Writer
	bw         *bufio.Writer
	statusCode int

	firstWriteDone     bool
	disableCompression bool
}

// Implements http.ResponseWriter.Header method.
func (zrw *gzipResponseWriter) Header() http.Header {
	return zrw.rw.Header()
}

// Implements http.ResponseWriter.Write method.
func (zrw *gzipResponseWriter) Write(p []byte) (int, error) {
	if !zrw.firstWriteDone {
		h := zrw.Header()
		if zrw.statusCode == http.StatusNoContent {
			zrw.disableCompression = true
		}
		if h.Get("Content-Encoding") != "" {
			zrw.disableCompression = true
		}
		if !zrw.disableCompression {
			h.Set("Content-Encoding", "gzip")
			h.Del("Content-Length")
			if h.Get("Content-Type") == "" {
				// Disable auto-detection of content-type, since it
				// is incorrectly detected after the compression.
				h.Set("Content-Type", "text/html; charset=utf-8")
			}
		}
		zrw.writeHeader()
		zrw.firstWriteDone = true
	}
	if zrw.disableCompression {
		return zrw.rw.Write(p)
	}
	return zrw.bw.Write(p)
}

// Implements http.ResponseWriter.WriteHeader method.
func (zrw *gzipResponseWriter) WriteHeader(statusCode int) {
	zrw.statusCode = statusCode
}

func (zrw *gzipResponseWriter) writeHeader() {
	if zrw.statusCode == 0 {
		zrw.statusCode = http.StatusOK
	}
	zrw.rw.WriteHeader(zrw.statusCode)
}

// Implements http.Flusher
func (zrw *gzipResponseWriter) Flush() {
	if !zrw.firstWriteDone {
		_, _ = zrw.Write(nil)
	}
	if !zrw.disableCompression {
		if err := zrw.bw.Flush(); err != nil && !isTrivialNetworkError(err) {
			logger.Warnf("gzipResponseWriter.Flush (buffer): %s", err)
		}
		if err := zrw.zw.Flush(); err != nil && !isTrivialNetworkError(err) {
			logger.Warnf("gzipResponseWriter.Flush (gzip): %s", err)
		}
	}
	if fw, ok := zrw.rw.(http.Flusher); ok {
		fw.Flush()
	}
}

func (zrw *gzipResponseWriter) Close() error {
	if !zrw.firstWriteDone {
		_, _ = zrw.Write(nil)
	}
	zrw.Flush()
	var err error
	if !zrw.disableCompression {
		err = zrw.zw.Close()
	}
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

// GetQuotedRemoteAddr returns quoted remote address.
func GetQuotedRemoteAddr(r *http.Request) string {
	remoteAddr := strconv.Quote(r.RemoteAddr) // quote remoteAddr and X-Forwarded-For, since they may contain untrusted input
	if addr := r.Header.Get("X-Forwarded-For"); addr != "" {
		remoteAddr += ", X-Forwarded-For: " + strconv.Quote(addr)
	}
	return remoteAddr
}

// Errorf writes formatted error message to w and to logger.
func Errorf(w http.ResponseWriter, r *http.Request, format string, args ...interface{}) {
	errStr := fmt.Sprintf(format, args...)
	remoteAddr := GetQuotedRemoteAddr(r)
	requestURI := GetRequestURI(r)
	errStr = fmt.Sprintf("remoteAddr: %s; requestURI: %s; %s", remoteAddr, requestURI, errStr)
	logger.WarnfSkipframes(1, "%s", errStr)

	// Extract statusCode from args
	statusCode := http.StatusBadRequest
	var esc *ErrorWithStatusCode
	for _, arg := range args {
		if err, ok := arg.(error); ok && errors.As(err, &esc) {
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

// Unwrap returns e.Err.
//
// This is used by standard errors package. See https://golang.org/pkg/errors
func (e *ErrorWithStatusCode) Unwrap() error {
	return e.Err
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

// IsTLS indicates is tls enabled or not
func IsTLS() bool {
	return *tlsEnable
}

// GetPathPrefix - returns http server path prefix.
func GetPathPrefix() string {
	return *pathPrefix
}

// WriteAPIHelp writes pathList to w in HTML format.
func WriteAPIHelp(w io.Writer, pathList [][2]string) {
	for _, p := range pathList {
		p, doc := p[0], p[1]
		fmt.Fprintf(w, "<a href=%q>%s</a> - %s<br/>", p, p, doc)
	}
}

// GetRequestURI returns requestURI for r.
func GetRequestURI(r *http.Request) string {
	requestURI := r.RequestURI
	if r.Method != "POST" {
		return requestURI
	}
	_ = r.ParseForm()
	queryArgs := r.PostForm.Encode()
	if len(queryArgs) == 0 {
		return requestURI
	}
	delimiter := "?"
	if strings.Contains(requestURI, delimiter) {
		delimiter = "&"
	}
	return requestURI + delimiter + queryArgs
}
