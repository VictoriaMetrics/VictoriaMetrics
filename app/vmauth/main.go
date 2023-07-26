package main

import (
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/textproto"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/buildinfo"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/envflag"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/netutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/procutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/pushmetrics"
	"github.com/VictoriaMetrics/metrics"
)

var (
	httpListenAddr   = flag.String("httpListenAddr", ":8427", "TCP address to listen for http connections. See also -httpListenAddr.useProxyProtocol")
	useProxyProtocol = flag.Bool("httpListenAddr.useProxyProtocol", false, "Whether to use proxy protocol for connections accepted at -httpListenAddr . "+
		"See https://www.haproxy.org/download/1.8/doc/proxy-protocol.txt . "+
		"With enabled proxy protocol http server cannot serve regular /metrics endpoint. Use -pushmetrics.url for metrics pushing")
	maxIdleConnsPerBackend = flag.Int("maxIdleConnsPerBackend", 100, "The maximum number of idle connections vmauth can open per each backend host. "+
		"See also -maxConcurrentRequests")
	responseTimeout       = flag.Duration("responseTimeout", 5*time.Minute, "The timeout for receiving a response from backend")
	maxConcurrentRequests = flag.Int("maxConcurrentRequests", 1000, "The maximum number of concurrent requests vmauth can process. Other requests are rejected with "+
		"'429 Too Many Requests' http status code. See also -maxConcurrentPerUserRequests and -maxIdleConnsPerBackend command-line options")
	maxConcurrentPerUserRequests = flag.Int("maxConcurrentPerUserRequests", 300, "The maximum number of concurrent requests vmauth can process per each configured user. "+
		"Other requests are rejected with '429 Too Many Requests' http status code. See also -maxConcurrentRequests command-line option and max_concurrent_requests option "+
		"in per-user config")
	reloadAuthKey        = flag.String("reloadAuthKey", "", "Auth key for /-/reload http endpoint. It must be passed as authKey=...")
	logInvalidAuthTokens = flag.Bool("logInvalidAuthTokens", false, "Whether to log requests with invalid auth tokens. "+
		`Such requests are always counted at vmauth_http_request_errors_total{reason="invalid_auth_token"} metric, which is exposed at /metrics page`)
)

func main() {
	// Write flags and help message to stdout, since it is easier to grep or pipe.
	flag.CommandLine.SetOutput(os.Stdout)
	flag.Usage = usage
	envflag.Parse()
	buildinfo.Init()
	logger.Init()
	pushmetrics.Init()

	logger.Infof("starting vmauth at %q...", *httpListenAddr)
	startTime := time.Now()
	initAuthConfig()
	go httpserver.Serve(*httpListenAddr, *useProxyProtocol, requestHandler)
	logger.Infof("started vmauth in %.3f seconds", time.Since(startTime).Seconds())

	sig := procutil.WaitForSigterm()
	logger.Infof("received signal %s", sig)

	startTime = time.Now()
	logger.Infof("gracefully shutting down webservice at %q", *httpListenAddr)
	if err := httpserver.Stop(*httpListenAddr); err != nil {
		logger.Fatalf("cannot stop the webservice: %s", err)
	}
	logger.Infof("successfully shut down the webservice in %.3f seconds", time.Since(startTime).Seconds())
	stopAuthConfig()
	logger.Infof("successfully stopped vmauth in %.3f seconds", time.Since(startTime).Seconds())
}

func requestHandler(w http.ResponseWriter, r *http.Request) bool {
	switch r.URL.Path {
	case "/-/reload":
		if !httpserver.CheckAuthFlag(w, r, *reloadAuthKey, "reloadAuthKey") {
			return true
		}
		configReloadRequests.Inc()
		procutil.SelfSIGHUP()
		w.WriteHeader(http.StatusOK)
		return true
	}
	authToken := r.Header.Get("Authorization")
	if authToken == "" {
		// Process requests for unauthorized users
		ui := authConfig.Load().UnauthorizedUser
		if ui != nil {
			processUserRequest(w, r, ui)
			return true
		}

		w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
		http.Error(w, "missing `Authorization` request header", http.StatusUnauthorized)
		return true
	}
	if strings.HasPrefix(authToken, "Token ") {
		// Handle InfluxDB's proprietary token authentication scheme as a bearer token authentication
		// See https://docs.influxdata.com/influxdb/v2.0/api/
		authToken = strings.Replace(authToken, "Token", "Bearer", 1)
	}

	ac := *authUsers.Load()
	ui := ac[authToken]
	if ui == nil {
		invalidAuthTokenRequests.Inc()
		if *logInvalidAuthTokens {
			err := fmt.Errorf("cannot find the provided auth token %q in config", authToken)
			err = &httpserver.ErrorWithStatusCode{
				Err:        err,
				StatusCode: http.StatusUnauthorized,
			}
			httpserver.Errorf(w, r, "%s", err)
		} else {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
		}
		return true
	}

	processUserRequest(w, r, ui)
	return true
}

func processUserRequest(w http.ResponseWriter, r *http.Request, ui *UserInfo) {
	startTime := time.Now()
	defer ui.requestsDuration.UpdateDuration(startTime)

	ui.requests.Inc()

	// Limit the concurrency of requests to backends
	concurrencyLimitOnce.Do(concurrencyLimitInit)
	select {
	case concurrencyLimitCh <- struct{}{}:
		if err := ui.beginConcurrencyLimit(); err != nil {
			handleConcurrencyLimitError(w, r, err)
			<-concurrencyLimitCh
			return
		}
	default:
		concurrentRequestsLimitReached.Inc()
		err := fmt.Errorf("cannot serve more than -maxConcurrentRequests=%d concurrent requests", cap(concurrencyLimitCh))
		handleConcurrencyLimitError(w, r, err)
		return
	}
	processRequest(w, r, ui)
	ui.endConcurrencyLimit()
	<-concurrencyLimitCh
}

func processRequest(w http.ResponseWriter, r *http.Request, ui *UserInfo) {
	u := normalizeURL(r.URL)
	up, headers := ui.getURLPrefixAndHeaders(u)
	isDefault := false
	if up == nil {
		missingRouteRequests.Inc()
		if ui.DefaultURL == nil {
			httpserver.Errorf(w, r, "missing route for %q", u.String())
			return
		}
		up, headers = ui.DefaultURL, ui.Headers
		isDefault = true
	}
	r.Body = &readTrackingBody{
		r: r.Body,
	}

	maxAttempts := up.getBackendsCount()
	for i := 0; i < maxAttempts; i++ {
		bu := up.getLeastLoadedBackendURL()
		targetURL := bu.url
		// Don't change path and add request_path query param for default route.
		if isDefault {
			query := targetURL.Query()
			query.Set("request_path", u.Path)
			targetURL.RawQuery = query.Encode()
		} else { // Update path for regular routes.
			targetURL = mergeURLs(targetURL, u)
		}
		ok := tryProcessingRequest(w, r, targetURL, headers)
		bu.put()
		if ok {
			return
		}
		bu.setBroken()
	}
	err := &httpserver.ErrorWithStatusCode{
		Err:        fmt.Errorf("all the backends for the user %q are unavailable", ui.name()),
		StatusCode: http.StatusServiceUnavailable,
	}
	httpserver.Errorf(w, r, "%s", err)
}

func tryProcessingRequest(w http.ResponseWriter, r *http.Request, targetURL *url.URL, headers []Header) bool {
	// This code has been copied from net/http/httputil/reverseproxy.go
	req := sanitizeRequestHeaders(r)
	req.URL = targetURL
	for _, h := range headers {
		req.Header.Set(h.Name, h.Value)
	}
	transportOnce.Do(transportInit)
	res, err := transport.RoundTrip(req)
	if err != nil {
		rtb := req.Body.(*readTrackingBody)
		if rtb.readStarted {
			// Request body has been already read, so it is impossible to retry the request.
			// Return the error to the client then.
			err = &httpserver.ErrorWithStatusCode{
				Err:        fmt.Errorf("cannot proxy the request to %q: %w", targetURL, err),
				StatusCode: http.StatusServiceUnavailable,
			}
			httpserver.Errorf(w, r, "%s", err)
			return true
		}
		// Retry the request if its body wasn't read yet. This usually means that the backend isn't reachable.
		remoteAddr := httpserver.GetQuotedRemoteAddr(r)
		// NOTE: do not use httpserver.GetRequestURI
		// it explicitly reads request body and fails retries.
		logger.Warnf("remoteAddr: %s; requestURI: %s; error when proxying the request to %q: %s", remoteAddr, req.URL, targetURL, err)
		return false
	}
	removeHopHeaders(res.Header)
	copyHeader(w.Header(), res.Header)
	w.WriteHeader(res.StatusCode)

	copyBuf := copyBufPool.Get()
	copyBuf.B = bytesutil.ResizeNoCopyNoOverallocate(copyBuf.B, 16*1024)
	_, err = io.CopyBuffer(w, res.Body, copyBuf.B)
	copyBufPool.Put(copyBuf)
	if err != nil && !netutil.IsTrivialNetworkError(err) {
		remoteAddr := httpserver.GetQuotedRemoteAddr(r)
		requestURI := httpserver.GetRequestURI(r)
		logger.Warnf("remoteAddr: %s; requestURI: %s; error when proxying response body from %s: %s", remoteAddr, requestURI, targetURL, err)
		return true
	}
	return true
}

var copyBufPool bytesutil.ByteBufferPool

func copyHeader(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}

func sanitizeRequestHeaders(r *http.Request) *http.Request {
	// This code has been copied from net/http/httputil/reverseproxy.go
	req := r.Clone(r.Context())
	removeHopHeaders(req.Header)
	if clientIP, _, err := net.SplitHostPort(req.RemoteAddr); err == nil {
		// If we aren't the first proxy retain prior
		// X-Forwarded-For information as a comma+space
		// separated list and fold multiple headers into one.
		prior := req.Header["X-Forwarded-For"]
		if len(prior) > 0 {
			clientIP = strings.Join(prior, ", ") + ", " + clientIP
		}
		req.Header.Set("X-Forwarded-For", clientIP)
	}
	return req
}

func removeHopHeaders(h http.Header) {
	// remove hop-by-hop headers listed in the "Connection" header of h.
	// See RFC 7230, section 6.1
	for _, f := range h["Connection"] {
		for _, sf := range strings.Split(f, ",") {
			if sf = textproto.TrimString(sf); sf != "" {
				h.Del(sf)
			}
		}
	}

	// Remove hop-by-hop headers to the backend. Especially
	// important is "Connection" because we want a persistent
	// connection, regardless of what the client sent to us.
	for _, key := range hopHeaders {
		h.Del(key)
	}
}

// Hop-by-hop headers. These are removed when sent to the backend.
// As of RFC 7230, hop-by-hop headers are required to appear in the
// Connection header field. These are the headers defined by the
// obsoleted RFC 2616 (section 13.5.1) and are used for backward
// compatibility.
var hopHeaders = []string{
	"Connection",
	"Proxy-Connection", // non-standard but still sent by libcurl and rejected by e.g. google
	"Keep-Alive",
	"Proxy-Authenticate",
	"Proxy-Authorization",
	"Te",      // canonicalized version of "TE"
	"Trailer", // not Trailers per URL above; https://www.rfc-editor.org/errata_search.php?eid=4522
	"Transfer-Encoding",
	"Upgrade",
}

var (
	configReloadRequests     = metrics.NewCounter(`vmauth_http_requests_total{path="/-/reload"}`)
	invalidAuthTokenRequests = metrics.NewCounter(`vmauth_http_request_errors_total{reason="invalid_auth_token"}`)
	missingRouteRequests     = metrics.NewCounter(`vmauth_http_request_errors_total{reason="missing_route"}`)
)

var (
	transport     *http.Transport
	transportOnce sync.Once
)

func transportInit() {
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.ResponseHeaderTimeout = *responseTimeout
	// Automatic compression must be disabled in order to fix https://github.com/VictoriaMetrics/VictoriaMetrics/issues/535
	tr.DisableCompression = true
	// Disable HTTP/2.0, since VictoriaMetrics components don't support HTTP/2.0 (because there is no sense in this).
	tr.ForceAttemptHTTP2 = false
	tr.MaxIdleConnsPerHost = *maxIdleConnsPerBackend
	if tr.MaxIdleConns != 0 && tr.MaxIdleConns < tr.MaxIdleConnsPerHost {
		tr.MaxIdleConns = tr.MaxIdleConnsPerHost
	}
	transport = tr
}

var (
	concurrencyLimitCh   chan struct{}
	concurrencyLimitOnce sync.Once
)

func concurrencyLimitInit() {
	concurrencyLimitCh = make(chan struct{}, *maxConcurrentRequests)
	_ = metrics.NewGauge("vmauth_concurrent_requests_capacity", func() float64 {
		return float64(*maxConcurrentRequests)
	})
	_ = metrics.NewGauge("vmauth_concurrent_requests_current", func() float64 {
		return float64(len(concurrencyLimitCh))
	})
}

var concurrentRequestsLimitReached = metrics.NewCounter("vmauth_concurrent_requests_limit_reached_total")

func usage() {
	const s = `
vmauth authenticates and authorizes incoming requests and proxies them to VictoriaMetrics.

See the docs at https://docs.victoriametrics.com/vmauth.html .
`
	flagutil.Usage(s)
}

func handleConcurrencyLimitError(w http.ResponseWriter, r *http.Request, err error) {
	w.Header().Add("Retry-After", "10")
	err = &httpserver.ErrorWithStatusCode{
		Err:        err,
		StatusCode: http.StatusTooManyRequests,
	}
	httpserver.Errorf(w, r, "%s", err)
}

type readTrackingBody struct {
	r           io.ReadCloser
	readStarted bool
}

// Read implements io.Reader interface
// tracks body reading requests
func (rtb *readTrackingBody) Read(p []byte) (int, error) {
	if len(p) > 0 {
		rtb.readStarted = true
	}
	return rtb.r.Read(p)
}

// Close implements io.Closer interface.
func (rtb *readTrackingBody) Close() error {
	// Close rtb.r only if at least a single Read call was performed.
	// http.Roundtrip performs body.Close call even without any Read calls
	// so this hack allows us to reuse request body
	if rtb.readStarted {
		return rtb.r.Close()
	}
	return nil
}
