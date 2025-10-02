package httpserver

import (
	"context"
	"crypto/tls"
	_ "embed"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/pprof"
	"net/url"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/VictoriaMetrics/metrics"
	"github.com/klauspost/compress/gzhttp"
	"github.com/valyala/fastrand"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/appmetrics"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/netutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/stringsutil"
)

var (
	tlsEnable = flagutil.NewArrayBool("tls", "Whether to enable TLS for incoming HTTP requests at the given -httpListenAddr (aka https). -tlsCertFile and -tlsKeyFile must be set if -tls is set. "+
		"See also -mtls")
	tlsCertFile = flagutil.NewArrayString("tlsCertFile", "Path to file with TLS certificate for the corresponding -httpListenAddr if -tls is set. "+
		"Prefer ECDSA certs instead of RSA certs as RSA certs are slower. The provided certificate file is automatically re-read every second, so it can be dynamically updated. "+
		"See also -tlsAutocertHosts")
	tlsKeyFile = flagutil.NewArrayString("tlsKeyFile", "Path to file with TLS key for the corresponding -httpListenAddr if -tls is set. "+
		"The provided key file is automatically re-read every second, so it can be dynamically updated. See also -tlsAutocertHosts")
	tlsCipherSuites = flagutil.NewArrayString("tlsCipherSuites", "Optional list of TLS cipher suites for incoming requests over HTTPS if -tls is set. See the list of supported cipher suites at https://pkg.go.dev/crypto/tls#pkg-constants")
	tlsMinVersion   = flagutil.NewArrayString("tlsMinVersion", "Optional minimum TLS version to use for the corresponding -httpListenAddr if -tls is set. "+
		"Supported values: TLS10, TLS11, TLS12, TLS13")

	pathPrefix = flag.String("http.pathPrefix", "", "An optional prefix to add to all the paths handled by http server. For example, if '-http.pathPrefix=/foo/bar' is set, "+
		"then all the http requests will be handled on '/foo/bar/*' paths. This may be useful for proxied requests. "+
		"See https://www.robustperception.io/using-external-urls-and-proxies-with-prometheus")
	httpAuthUsername = flag.String("httpAuth.username", "", "Username for HTTP server's Basic Auth. The authentication is disabled if empty. See also -httpAuth.password")
	httpAuthPassword = flagutil.NewPassword("httpAuth.password", "Password for HTTP server's Basic Auth. The authentication is disabled if -httpAuth.username is empty")
	metricsAuthKey   = flagutil.NewPassword("metricsAuthKey", "Auth key for /metrics endpoint. It must be passed via authKey query arg. It overrides -httpAuth.*")
	flagsAuthKey     = flagutil.NewPassword("flagsAuthKey", "Auth key for /flags endpoint. It must be passed via authKey query arg. It overrides -httpAuth.*")
	pprofAuthKey     = flagutil.NewPassword("pprofAuthKey", "Auth key for /debug/pprof/* endpoints. It must be passed via authKey query arg. It overrides -httpAuth.*")

	disableKeepAlive            = flag.Bool("http.disableKeepAlive", false, "Whether to disable HTTP keep-alive for incoming connections at -httpListenAddr")
	disableResponseCompression  = flag.Bool("http.disableResponseCompression", false, "Disable compression of HTTP responses to save CPU resources. By default, compression is enabled to save network bandwidth")
	maxGracefulShutdownDuration = flag.Duration("http.maxGracefulShutdownDuration", 7*time.Second, `The maximum duration for a graceful shutdown of the HTTP server. A highly loaded server may require increased value for a graceful shutdown`)
	shutdownDelay               = flag.Duration("http.shutdownDelay", 0, `Optional delay before http server shutdown. During this delay, the server returns non-OK responses from /health page, so load balancers can route new requests to other servers`)
	idleConnTimeout             = flag.Duration("http.idleConnTimeout", time.Minute, "Timeout for incoming idle http connections")
	connTimeout                 = flag.Duration("http.connTimeout", 2*time.Minute, "Incoming connections to -httpListenAddr are closed after the configured timeout. "+
		"This may help evenly spreading load among a cluster of services behind TCP-level load balancer. Zero value disables closing of incoming connections")

	headerHSTS         = flag.String("http.header.hsts", "", "Value for 'Strict-Transport-Security' header, recommended: 'max-age=31536000; includeSubDomains'")
	headerFrameOptions = flag.String("http.header.frameOptions", "", "Value for 'X-Frame-Options' header")
	headerCSP          = flag.String("http.header.csp", "", `Value for 'Content-Security-Policy' header, recommended: "default-src 'self'"`)

	disableCORS = flag.Bool("http.disableCORS", false, `Disable CORS for all origins (*)`)
)

var (
	servers     = make(map[string]*server)
	serversLock sync.Mutex
)

type server struct {
	shutdownDelayDeadline atomic.Int64
	s                     *http.Server
}

// RequestHandler must serve the given request r and write response to w.
//
// RequestHandler must return true if the request has been served (successfully or not).
//
// RequestHandler must return false if it cannot serve the given request.
// In such cases the caller must serve the request.
type RequestHandler func(w http.ResponseWriter, r *http.Request) bool

// ServeOptions defines optional parameters for http server
type ServeOptions struct {
	// UseProxyProtocol if is set to true for the corresponding addr, then the incoming connections are accepted via proxy protocol.
	// See https://www.haproxy.org/download/1.8/doc/proxy-protocol.txt
	UseProxyProtocol *flagutil.ArrayBool
	// DisableBuiltinRoutes whether not to serve built-in routes for the given server, such as:
	// /health, /debug/pprof and few others
	// In addition basic auth check and authKey checks will be disabled for the given addr
	//
	// Mostly required by http proxy servers, which performs own authorization and requests routing
	DisableBuiltinRoutes bool
}

// Serve starts an http server on the given addrs with the given optional rh.
//
// By default all the responses are transparently compressed, since egress traffic is usually expensive.
func Serve(addrs []string, rh RequestHandler, opts ServeOptions) {
	if rh == nil {
		rh = func(_ http.ResponseWriter, _ *http.Request) bool {
			return false
		}
	}
	for idx, addr := range addrs {
		if addr == "" {
			continue
		}
		go serve(addr, rh, idx, opts)
	}
}

func serve(addr string, rh RequestHandler, idx int, opts ServeOptions) {
	scheme := "http"
	if tlsEnable.GetOptionalArg(idx) {
		scheme = "https"
	}
	useProxyProto := false
	if opts.UseProxyProtocol != nil {
		useProxyProto = opts.UseProxyProtocol.GetOptionalArg(idx)
	}

	var tlsConfig *tls.Config
	if tlsEnable.GetOptionalArg(idx) {
		certFile := tlsCertFile.GetOptionalArg(idx)
		keyFile := tlsKeyFile.GetOptionalArg(idx)
		minVersion := tlsMinVersion.GetOptionalArg(idx)
		tc, err := netutil.GetServerTLSConfig(certFile, keyFile, minVersion, *tlsCipherSuites)
		if err != nil {
			logger.Fatalf("cannot load TLS cert from -tlsCertFile=%q, -tlsKeyFile=%q, -tlsMinVersion=%q, -tlsCipherSuites=%q: %s", certFile, keyFile, minVersion, *tlsCipherSuites, err)
		}
		tlsConfig = tc
	}
	ln, err := netutil.NewTCPListener(scheme, addr, useProxyProto, tlsConfig)
	if err != nil {
		logger.Fatalf("cannot start http server at %s: %s", addr, err)
	}
	logger.Infof("started server at %s://%s/", scheme, ln.Addr())
	if !opts.DisableBuiltinRoutes {
		logger.Infof("pprof handlers are exposed at %s://%s/debug/pprof/", scheme, ln.Addr())
	}

	serveWithListener(addr, ln, rh, opts.DisableBuiltinRoutes)
}

func serveWithListener(addr string, ln net.Listener, rh RequestHandler, disableBuiltinRoutes bool) {
	var s server

	s.s = &http.Server{

		// Disable http/2, since it doesn't give any advantages for VictoriaMetrics services.
		TLSNextProto: make(map[string]func(*http.Server, *tls.Conn, http.Handler)),

		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       *idleConnTimeout,

		// Do not set ReadTimeout and WriteTimeout here,
		// since these timeouts must be controlled by request handlers.

		ErrorLog: logger.StdErrorLogger(),
	}
	s.s.SetKeepAlivesEnabled(!*disableKeepAlive)
	if *connTimeout > 0 {
		s.s.ConnContext = func(ctx context.Context, _ net.Conn) context.Context {
			timeoutSec := connTimeout.Seconds()
			// Add a jitter for connection timeout in order to prevent Thundering herd problem
			// when all the connections are established at the same time.
			// See https://en.wikipedia.org/wiki/Thundering_herd_problem
			jitterSec := fastrand.Uint32n(uint32(timeoutSec / 10))
			deadline := fasttime.UnixTimestamp() + uint64(timeoutSec) + uint64(jitterSec)
			return context.WithValue(ctx, connDeadlineTimeKey, &deadline)
		}
	}
	rhw := rh
	if !disableBuiltinRoutes {
		rhw = func(w http.ResponseWriter, r *http.Request) bool {
			return builtinRoutesHandler(&s, r, w, rh)
		}
	}
	h := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handlerWrapper(w, r, rhw)
	})
	if !*disableResponseCompression {
		h = gzipHandlerWrapper(h)
	}
	s.s.Handler = h

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
	if *connTimeout <= 0 {
		return false
	}
	ctx := r.Context()
	v := ctx.Value(connDeadlineTimeKey)
	deadline, ok := v.(*uint64)
	return ok && fasttime.UnixTimestamp() > *deadline
}

var connDeadlineTimeKey = any("connDeadlineSecs")

// Stop stops the http server on the given addrs, which has been started via Serve func.
func Stop(addrs []string) error {
	var errGlobalLock sync.Mutex
	var errGlobal error

	var wg sync.WaitGroup
	for _, addr := range addrs {
		if addr == "" {
			continue
		}
		wg.Add(1)
		go func(addr string) {
			if err := stop(addr); err != nil {
				errGlobalLock.Lock()
				errGlobal = err
				errGlobalLock.Unlock()
			}
			wg.Done()
		}(addr)
	}
	wg.Wait()

	return errGlobal
}

func stop(addr string) error {
	serversLock.Lock()
	s := servers[addr]
	delete(servers, addr)
	serversLock.Unlock()
	if s == nil {
		err := fmt.Errorf("BUG: there is no server at %q", addr)
		logger.Panicf("%s", err)
		// The return is needed for golangci-lint: SA5011(related information): this check suggests that the pointer can be nil
		return err
	}

	deadline := time.Now().Add(*shutdownDelay).UnixNano()
	s.shutdownDelayDeadline.Store(deadline)
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

var gzipHandlerWrapper = func() func(http.Handler) http.HandlerFunc {
	hw, err := gzhttp.NewWrapper(gzhttp.CompressionLevel(1))
	if err != nil {
		panic(fmt.Errorf("BUG: cannot initialize gzip http wrapper: %w", err))
	}
	return hw
}()

var (
	metricsHandlerDuration = metrics.NewHistogram(`vm_http_request_duration_seconds{path="/metrics"}`)
	connTimeoutClosedConns = metrics.NewCounter(`vm_http_conn_timeout_closed_conns_total`)
)

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

func handlerWrapper(w http.ResponseWriter, r *http.Request, rh RequestHandler) {
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

	h := w.Header()
	if *headerHSTS != "" {
		h.Add("Strict-Transport-Security", *headerHSTS)
	}
	if *headerFrameOptions != "" {
		h.Add("X-Frame-Options", *headerFrameOptions)
	}
	if *headerCSP != "" {
		h.Add("Content-Security-Policy", *headerCSP)
	}
	h.Add("X-Server-Hostname", hostname)
	requestsTotal.Inc()
	if whetherToCloseConn(r) {
		connTimeoutClosedConns.Inc()
		h.Set("Connection", "close")
	}
	path := r.URL.Path

	prefix := GetPathPrefix()
	if prefix != "" {
		// Trim -http.pathPrefix from path
		prefixNoTrailingSlash := strings.TrimSuffix(prefix, "/")
		if path == prefixNoTrailingSlash {
			// Redirect to url with / at the end.
			// This is needed for proper handling of relative urls in web browsers.
			// Intentionally ignore query args, since it is expected that the requested url
			// is composed by a human, so it doesn't contain query args.
			Redirect(w, prefix)
			return
		}
		if !strings.HasPrefix(path, prefix) {
			Errorf(w, r, "missing -http.pathPrefix=%q in the requested path %q", *pathPrefix, path)
			unsupportedRequestErrors.Inc()
			return
		}
		path = path[len(prefix)-1:]
		r.URL.Path = path
	}

	w = &responseWriterWithAbort{
		ResponseWriter: w,
	}
	if rh(w, r) {
		return
	}

	Errorf(w, r, "unsupported path requested: %q", r.URL.Path)
	unsupportedRequestErrors.Inc()
}

func builtinRoutesHandler(s *server, r *http.Request, w http.ResponseWriter, rh RequestHandler) bool {

	h := w.Header()

	path := r.URL.Path
	if strings.HasSuffix(path, "/favicon.ico") {
		w.Header().Set("Cache-Control", "max-age=3600")
		faviconRequests.Inc()
		w.Write(faviconData)
		return true
	}

	switch r.URL.Path {
	case "/health":
		h.Set("Content-Type", "text/plain; charset=utf-8")
		deadline := s.shutdownDelayDeadline.Load()
		if deadline <= 0 {
			w.Write([]byte("OK"))
			return true
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
		return true
	case "/ping":
		// This is needed for compatibility with InfluxDB agents.
		// See https://docs.influxdata.com/influxdb/v1.7/tools/api/#ping-http-endpoint
		status := http.StatusNoContent
		if verbose := r.FormValue("verbose"); verbose == "true" {
			status = http.StatusOK
		}
		w.WriteHeader(status)
		return true
	case "/metrics":
		metricsRequests.Inc()
		if !CheckAuthFlag(w, r, metricsAuthKey) {
			return true
		}
		startTime := time.Now()
		h.Set("Content-Type", "text/plain; charset=utf-8")
		appmetrics.WritePrometheusMetrics(w)
		metricsHandlerDuration.UpdateDuration(startTime)
		return true
	case "/flags":
		if !CheckAuthFlag(w, r, flagsAuthKey) {
			return true
		}
		h.Set("Content-Type", "text/plain; charset=utf-8")
		flagutil.WriteFlags(w)
		return true
	case "/-/healthy":
		// This is needed for Prometheus compatibility
		// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1833
		fmt.Fprintf(w, "VictoriaMetrics is Healthy.\n")
		return true
	case "/-/ready":
		// This is needed for Prometheus compatibility
		// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1833
		fmt.Fprintf(w, "VictoriaMetrics is Ready.\n")
		return true
	case "/robots.txt":
		// This prevents search engines from indexing contents
		// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4128
		fmt.Fprintf(w, "User-agent: *\nDisallow: /\n")
		return true
	default:
		if strings.HasPrefix(r.URL.Path, "/debug/pprof/") {
			pprofRequests.Inc()
			if !CheckAuthFlag(w, r, pprofAuthKey) {
				return true
			}
			pprofHandler(r.URL.Path[len("/debug/pprof/"):], w, r)
			return true
		}

		if !isProtectedByAuthFlag(r.URL.Path) && !CheckBasicAuth(w, r) {
			return true
		}
	}
	return rh(w, r)
}

func isProtectedByAuthFlag(path string) bool {
	// These paths must explicitly call CheckAuthFlag().
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/6329
	return strings.HasSuffix(path, "/config") || strings.HasSuffix(path, "/reload") ||
		strings.HasSuffix(path, "/resetRollupResultCache") || strings.HasSuffix(path, "/delSeries") || strings.HasSuffix(path, "/delete_series") ||
		strings.HasSuffix(path, "/force_merge") || strings.HasSuffix(path, "/force_flush") || strings.HasSuffix(path, "/snapshot") ||
		strings.HasPrefix(path, "/snapshot/") || strings.HasSuffix(path, "/admin/status/metric_names_stats/reset")
}

// CheckAuthFlag checks whether the given authKey is set and valid
//
// Falls back to checkBasicAuth if authKey is not set
func CheckAuthFlag(w http.ResponseWriter, r *http.Request, expectedKey *flagutil.Password) bool {
	expectedValue := expectedKey.Get()
	if expectedValue == "" {
		return CheckBasicAuth(w, r)
	}
	if len(r.FormValue("authKey")) == 0 {
		authKeyRequestErrors.Inc()
		http.Error(w, fmt.Sprintf("Expected to receive non-empty authKey when -%s is set", expectedKey.Name()), http.StatusUnauthorized)
		return false
	}
	if r.FormValue("authKey") != expectedValue {
		authKeyRequestErrors.Inc()
		http.Error(w, fmt.Sprintf("The provided authKey doesn't match -%s", expectedKey.Name()), http.StatusUnauthorized)
		return false
	}
	return true
}

// CheckBasicAuth validates credentials provided in request if httpAuth.* flags are set
// returns true if credentials are valid or httpAuth.* flags are not set
func CheckBasicAuth(w http.ResponseWriter, r *http.Request) bool {
	if len(*httpAuthUsername) == 0 {
		// HTTP Basic Auth is disabled.
		return true
	}
	username, password, ok := r.BasicAuth()
	if ok {
		if username == *httpAuthUsername && password == httpAuthPassword.Get() {
			return true
		}
		authBasicRequestErrors.Inc()
	}

	w.Header().Set("WWW-Authenticate", `Basic realm="VictoriaMetrics"`)
	http.Error(w, "", http.StatusUnauthorized)
	return false
}

// EnableCORS enables https://developer.mozilla.org/en-US/docs/Web/HTTP/CORS
// on the response.
func EnableCORS(w http.ResponseWriter, _ *http.Request) {
	if *disableCORS {
		// see https://github.com/VictoriaMetrics/VictoriaMetrics/issues/8680
		return
	}
	w.Header().Set("Access-Control-Allow-Origin", "*")
}

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
	faviconRequests      = metrics.NewCounter(`vm_http_requests_total{path="*/favicon.ico"}`)

	authBasicRequestErrors   = metrics.NewCounter(`vm_http_request_errors_total{path="*", reason="wrong_basic_auth"}`)
	authKeyRequestErrors     = metrics.NewCounter(`vm_http_request_errors_total{path="*", reason="wrong_auth_key"}`)
	unsupportedRequestErrors = metrics.NewCounter(`vm_http_request_errors_total{path="*", reason="unsupported"}`)

	requestsTotal = metrics.NewCounter(`vm_http_requests_all_total`)
)

//go:embed favicon.ico
var faviconData []byte

// GetQuotedRemoteAddr returns quoted remote address.
func GetQuotedRemoteAddr(r *http.Request) string {
	remoteAddr := r.RemoteAddr
	if addr := r.Header.Get("X-Forwarded-For"); addr != "" {
		remoteAddr += ", X-Forwarded-For: " + addr
	}
	// quote remoteAddr and X-Forwarded-For, since they may contain untrusted input
	return stringsutil.JSONString(remoteAddr)
}

type responseWriterWithAbort struct {
	http.ResponseWriter

	sentHeaders bool
	aborted     bool
}

func (rwa *responseWriterWithAbort) Write(data []byte) (int, error) {
	if rwa.aborted {
		return 0, fmt.Errorf("response connection is aborted")
	}
	if !rwa.sentHeaders {
		rwa.sentHeaders = true
	}
	return rwa.ResponseWriter.Write(data)
}

func (rwa *responseWriterWithAbort) WriteHeader(statusCode int) {
	if rwa.aborted {
		logger.WarnfSkipframes(1, "cannot write response headers with statusCode=%d, since the response connection has been aborted", statusCode)
		return
	}
	if rwa.sentHeaders {
		logger.WarnfSkipframes(1, "cannot write response headers with statusCode=%d, since they were already sent", statusCode)
		return
	}
	rwa.ResponseWriter.WriteHeader(statusCode)
	rwa.sentHeaders = true
}

// Flush implements net/http.Flusher interface
func (rwa *responseWriterWithAbort) Flush() {
	if rwa.aborted {
		return
	}
	if !rwa.sentHeaders {
		rwa.sentHeaders = true
	}
	flusher, ok := rwa.ResponseWriter.(http.Flusher)
	if !ok {
		logger.Panicf("BUG: it is expected http.ResponseWriter (%T) supports http.Flusher interface", rwa.ResponseWriter)
	}
	flusher.Flush()
}

// abort aborts the client connection associated with rwa.
//
// The last http chunk in the response stream is intentionally written incorrectly,
// so the client, which reads the response, could notice this error.
func (rwa *responseWriterWithAbort) abort() {
	if !rwa.sentHeaders {
		logger.Panicf("BUG: abort can be called only after http response headers are sent")
	}
	if rwa.aborted {
		// Nothing to do. The connection has been already aborted.
		return
	}
	hj, ok := rwa.ResponseWriter.(http.Hijacker)
	if !ok {
		logger.Panicf("BUG: ResponseWriter must implement http.Hijacker interface")
	}
	conn, bw, err := hj.Hijack()
	if err != nil {
		logger.WarnfSkipframes(2, "cannot hijack response connection: %s", err)
		return
	}

	// Just write an error message into the client connection as is without http chunked encoding.
	// This is needed in order to notify the client about the aborted connection.
	_, _ = bw.WriteString("\nthe connection has been aborted; see the last line in the response and/or in the server log for the reason\n")
	_ = bw.Flush()

	// Forcibly close the client connection in order to break http keep-alive at client side.
	_ = conn.Close()

	rwa.aborted = true
}

// Errorf writes formatted error message to w and to logger.
func Errorf(w http.ResponseWriter, r *http.Request, format string, args ...any) {
	errStr := fmt.Sprintf(format, args...)
	logHTTPError(r, errStr)

	// Extract statusCode from args
	statusCode := http.StatusBadRequest
	var esc *ErrorWithStatusCode
	for _, arg := range args {
		if err, ok := arg.(error); ok && errors.As(err, &esc) {
			statusCode = esc.StatusCode
			break
		}
	}

	if rwa, ok := w.(*responseWriterWithAbort); ok && rwa.sentHeaders {
		// HTTP status code has been already sent to client, so it cannot be sent again.
		// Just write errStr to the response and abort the client connection, so the client could notice the error.
		fmt.Fprintf(w, "\n%s\n", errStr)
		rwa.abort()
		return
	}
	http.Error(w, errStr, statusCode)
}

// logHTTPError logs the errStr with the client remote address and the request URI obtained from r.
func logHTTPError(r *http.Request, errStr string) {
	remoteAddr := GetQuotedRemoteAddr(r)
	requestURI := GetRequestURI(r)
	errStr = fmt.Sprintf("remoteAddr: %s; requestURI: %s; %s", remoteAddr, requestURI, errStr)
	logger.WarnfSkipframes(2, "%s", errStr)
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

// IsTLS indicates is tls enabled or not for -httpListenAddr at the given idx.
func IsTLS(idx int) bool {
	return tlsEnable.GetOptionalArg(idx)
}

// GetPathPrefix - returns http server path prefix.
func GetPathPrefix() string {
	prefix := *pathPrefix
	if prefix == "" {
		return ""
	}
	if !strings.HasPrefix(prefix, "/") {
		prefix = "/" + prefix
	}
	if !strings.HasSuffix(prefix, "/") {
		prefix += "/"
	}
	return prefix
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
	if r.Method != http.MethodPost {
		return requestURI
	}
	_ = r.ParseForm()
	if len(r.PostForm) == 0 {
		return requestURI
	}
	// code copied from url.Query.Encode
	var queryArgs strings.Builder
	for k := range r.PostForm {
		vs := r.PostForm[k]
		// mask authKey as well-known secret
		if k == "authKey" {
			vs = []string{"secret"}
		}
		keyEscaped := url.QueryEscape(k)
		for _, v := range vs {
			if queryArgs.Len() > 0 {
				queryArgs.WriteByte('&')
			}
			queryArgs.WriteString(keyEscaped)
			queryArgs.WriteByte('=')
			queryArgs.WriteString(url.QueryEscape(v))
		}
	}
	delimiter := "?"
	if strings.Contains(requestURI, delimiter) {
		delimiter = "&"
	}
	return requestURI + delimiter + queryArgs.String()
}

// Redirect redirects to the given url.
func Redirect(w http.ResponseWriter, url string) {
	// Do not use http.Redirect, since it breaks relative redirects
	// if the http.Request.URL contains unexpected url.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2918
	w.Header().Set("Location", url)
	// Use http.StatusFound instead of http.StatusMovedPermanently,
	// since browsers can cache incorrect redirects returned with StatusMovedPermanently.
	// This may require browser cache cleaning after the incorrect redirect is fixed.
	w.WriteHeader(http.StatusFound)
}

// LogError logs the errStr with the context from req.
func LogError(req *http.Request, errStr string) {
	uri := GetRequestURI(req)
	remoteAddr := GetQuotedRemoteAddr(req)
	logger.Errorf("uri: %s, remote address: %q: %s", uri, remoteAddr, errStr)
}
