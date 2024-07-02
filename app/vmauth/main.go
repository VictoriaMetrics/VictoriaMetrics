package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/textproto"
	"net/url"
	"os"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/VictoriaMetrics/metrics"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/buildinfo"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/envflag"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/netutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/procutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/pushmetrics"
)

var (
	httpListenAddrs  = flagutil.NewArrayString("httpListenAddr", "TCP address to listen for incoming http requests. See also -tls and -httpListenAddr.useProxyProtocol")
	useProxyProtocol = flagutil.NewArrayBool("httpListenAddr.useProxyProtocol", "Whether to use proxy protocol for connections accepted at the corresponding -httpListenAddr . "+
		"See https://www.haproxy.org/download/1.8/doc/proxy-protocol.txt . "+
		"With enabled proxy protocol http server cannot serve regular /metrics endpoint. Use -pushmetrics.url for metrics pushing")
	maxIdleConnsPerBackend = flag.Int("maxIdleConnsPerBackend", 100, "The maximum number of idle connections vmauth can open per each backend host. "+
		"See also -maxConcurrentRequests")
	idleConnTimeout = flag.Duration("idleConnTimeout", 50*time.Second, `Defines a duration for idle (keep-alive connections) to exist.
    Consider setting this value less than "-http.idleConnTimeout". It must prevent possible "write: broken pipe" and "read: connection reset by peer" errors.`)
	responseTimeout       = flag.Duration("responseTimeout", 5*time.Minute, "The timeout for receiving a response from backend")
	maxConcurrentRequests = flag.Int("maxConcurrentRequests", 1000, "The maximum number of concurrent requests vmauth can process. Other requests are rejected with "+
		"'429 Too Many Requests' http status code. See also -maxConcurrentPerUserRequests and -maxIdleConnsPerBackend command-line options")
	maxConcurrentPerUserRequests = flag.Int("maxConcurrentPerUserRequests", 300, "The maximum number of concurrent requests vmauth can process per each configured user. "+
		"Other requests are rejected with '429 Too Many Requests' http status code. See also -maxConcurrentRequests command-line option and max_concurrent_requests option "+
		"in per-user config")
	reloadAuthKey        = flagutil.NewPassword("reloadAuthKey", "Auth key for /-/reload http endpoint. It must be passed via authKey query arg. It overrides httpAuth.* settings.")
	logInvalidAuthTokens = flag.Bool("logInvalidAuthTokens", false, "Whether to log requests with invalid auth tokens. "+
		`Such requests are always counted at vmauth_http_request_errors_total{reason="invalid_auth_token"} metric, which is exposed at /metrics page`)
	failTimeout               = flag.Duration("failTimeout", 3*time.Second, "Sets a delay period for load balancing to skip a malfunctioning backend")
	maxRequestBodySizeToRetry = flagutil.NewBytes("maxRequestBodySizeToRetry", 16*1024, "The maximum request body size, which can be cached and re-tried at other backends. "+
		"Bigger values may require more memory. Negative or zero values disable request body caching and retries.")
	backendTLSInsecureSkipVerify = flag.Bool("backend.tlsInsecureSkipVerify", false, "Whether to skip TLS verification when connecting to backends over HTTPS. "+
		"See https://docs.victoriametrics.com/vmauth/#backend-tls-setup")
	backendTLSCAFile = flag.String("backend.TLSCAFile", "", "Optional path to TLS root CA file, which is used for TLS verification when connecting to backends over HTTPS. "+
		"See https://docs.victoriametrics.com/vmauth/#backend-tls-setup")
	backendTLSCertFile = flag.String("backend.TLSCertFile", "", "Optional path to TLS client certificate file, which must be sent to HTTPS backend. "+
		"See https://docs.victoriametrics.com/vmauth/#backend-tls-setup")
	backendTLSKeyFile = flag.String("backend.TLSKeyFile", "", "Optional path to TLS client key file, which must be sent to HTTPS backend. "+
		"See https://docs.victoriametrics.com/vmauth/#backend-tls-setup")
	backendTLSServerName = flag.String("backend.TLSServerName", "", "Optional TLS ServerName, which must be sent to HTTPS backend. "+
		"See https://docs.victoriametrics.com/vmauth/#backend-tls-setup")
)

func main() {
	// Write flags and help message to stdout, since it is easier to grep or pipe.
	flag.CommandLine.SetOutput(os.Stdout)
	flag.Usage = usage
	envflag.Parse()
	buildinfo.Init()
	logger.Init()

	listenAddrs := *httpListenAddrs
	if len(listenAddrs) == 0 {
		listenAddrs = []string{":8427"}
	}
	logger.Infof("starting vmauth at %q...", listenAddrs)
	startTime := time.Now()
	initAuthConfig()
	go httpserver.Serve(listenAddrs, useProxyProtocol, requestHandler)
	logger.Infof("started vmauth in %.3f seconds", time.Since(startTime).Seconds())

	pushmetrics.Init()
	sig := procutil.WaitForSigterm()
	logger.Infof("received signal %s", sig)
	pushmetrics.Stop()

	startTime = time.Now()
	logger.Infof("gracefully shutting down webservice at %q", listenAddrs)
	if err := httpserver.Stop(listenAddrs); err != nil {
		logger.Fatalf("cannot stop the webservice: %s", err)
	}
	logger.Infof("successfully shut down the webservice in %.3f seconds", time.Since(startTime).Seconds())
	stopAuthConfig()
	logger.Infof("successfully stopped vmauth in %.3f seconds", time.Since(startTime).Seconds())
}

func requestHandler(w http.ResponseWriter, r *http.Request) bool {
	switch r.URL.Path {
	case "/-/reload":
		if !httpserver.CheckAuthFlag(w, r, reloadAuthKey.Get(), "reloadAuthKey") {
			return true
		}
		configReloadRequests.Inc()
		procutil.SelfSIGHUP()
		w.WriteHeader(http.StatusOK)
		return true
	}

	ats := getAuthTokensFromRequest(r)
	if len(ats) == 0 {
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

	ui := getUserInfoByAuthTokens(ats)
	if ui == nil {
		invalidAuthTokenRequests.Inc()
		if *logInvalidAuthTokens {
			err := fmt.Errorf("cannot authorize request with auth tokens %q", ats)
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

func getUserInfoByAuthTokens(ats []string) *UserInfo {
	ac := *authUsers.Load()
	for _, at := range ats {
		ui := ac[at]
		if ui != nil {
			return ui
		}
	}
	return nil
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
	up, hc := ui.getURLPrefixAndHeaders(u, r.Header)
	isDefault := false
	if up == nil {
		if ui.DefaultURL == nil {
			// Authorization should be requested for http requests without credentials
			// to a route that is not in the configuration for unauthorized user.
			// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/5236
			if ui.BearerToken == "" && ui.Username == "" && len(*authUsers.Load()) > 0 {
				w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
				http.Error(w, "missing `Authorization` request header", http.StatusUnauthorized)
				return
			}
			missingRouteRequests.Inc()
			httpserver.Errorf(w, r, "missing route for %q", u.String())
			return
		}
		up, hc = ui.DefaultURL, ui.HeadersConf
		isDefault = true
	}
	// caching makes sense only for positive non zero size
	if maxRequestBodySizeToRetry.IntN() > 0 {
		rtb := getReadTrackingBody(r.Body, int(r.ContentLength))
		defer putReadTrackingBody(rtb)
		r.Body = rtb
	}
	maxAttempts := up.getBackendsCount()
	for i := 0; i < maxAttempts; i++ {
		bu := up.getBackendURL()
		targetURL := bu.url
		// Don't change path and add request_path query param for default route.
		if isDefault {
			query := targetURL.Query()
			query.Set("request_path", u.String())
			targetURL.RawQuery = query.Encode()
		} else { // Update path for regular routes.
			targetURL = mergeURLs(targetURL, u, up.dropSrcPathPrefixParts)
		}
		ok := tryProcessingRequest(w, r, targetURL, hc, up.retryStatusCodes, ui)
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
	ui.backendErrors.Inc()
}

func tryProcessingRequest(w http.ResponseWriter, r *http.Request, targetURL *url.URL, hc HeadersConf, retryStatusCodes []int, ui *UserInfo) bool {
	// This code has been copied from net/http/httputil/reverseproxy.go
	req := sanitizeRequestHeaders(r)
	req.URL = targetURL

	if req.URL.Scheme == "https" || ui.overrideHostHeader {
		// Override req.Host only for https requests, since https server verifies hostnames during TLS handshake,
		// so it expects the targetURL.Host in the request.
		// There is no need in overriding the req.Host for http requests, since it is expected that backend server
		// may properly process queries with the original req.Host.
		req.Host = targetURL.Host
	}
	updateHeadersByConfig(req.Header, hc.RequestHeaders)
	var trivialRetries int
	rtb, rtbOK := req.Body.(*readTrackingBody)
again:
	res, err := ui.rt.RoundTrip(req)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			// Do not retry canceled or timed out requests
			remoteAddr := httpserver.GetQuotedRemoteAddr(r)
			requestURI := httpserver.GetRequestURI(r)
			logger.Warnf("remoteAddr: %s; requestURI: %s; error when proxying response body from %s: %s", remoteAddr, requestURI, targetURL, err)
			if errors.Is(err, context.DeadlineExceeded) {
				// Timed out request must be counted as errors, since this usually means that the backend is slow.
				ui.backendErrors.Inc()
			}
			return true
		}
		if !rtbOK || !rtb.canRetry() {
			// Request body cannot be re-sent to another backend. Return the error to the client then.
			err = &httpserver.ErrorWithStatusCode{
				Err:        fmt.Errorf("cannot proxy the request to %s: %w", targetURL, err),
				StatusCode: http.StatusServiceUnavailable,
			}
			httpserver.Errorf(w, r, "%s", err)
			ui.backendErrors.Inc()
			return true
		}
		// one time retry trivial network errors, such as proxy idle timeout misconfiguration
		// or socket close by OS
		if (netutil.IsTrivialNetworkError(err) || errors.Is(err, io.EOF)) && trivialRetries < 1 {
			trivialRetries++
			goto again
		}
		// Retry the request if its body wasn't read yet. This usually means that the backend isn't reachable.
		remoteAddr := httpserver.GetQuotedRemoteAddr(r)
		// NOTE: do not use httpserver.GetRequestURI
		// it explicitly reads request body, which may fail retries.
		logger.Warnf("remoteAddr: %s; requestURI: %s; retrying the request to %s because of response error: %s", remoteAddr, req.URL, targetURL, err)
		return false
	}
	if slices.Contains(retryStatusCodes, res.StatusCode) {
		_ = res.Body.Close()
		if !rtbOK || !rtb.canRetry() {
			// If we get an error from the retry_status_codes list, but cannot execute retry,
			// we consider such a request an error as well.
			err := &httpserver.ErrorWithStatusCode{
				Err: fmt.Errorf("got response status code=%d from %s, but cannot retry the request on another backend, because the request has been already consumed",
					res.StatusCode, targetURL),
				StatusCode: http.StatusServiceUnavailable,
			}
			httpserver.Errorf(w, r, "%s", err)
			ui.backendErrors.Inc()
			return true
		}
		// Retry requests at other backends if it matches retryStatusCodes.
		// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4893
		remoteAddr := httpserver.GetQuotedRemoteAddr(r)
		// NOTE: do not use httpserver.GetRequestURI
		// it explicitly reads request body, which may fail retries.
		logger.Warnf("remoteAddr: %s; requestURI: %s; retrying the request to %s because response status code=%d belongs to retry_status_codes=%d",
			remoteAddr, req.URL, targetURL, res.StatusCode, retryStatusCodes)
		return false
	}
	removeHopHeaders(res.Header)
	copyHeader(w.Header(), res.Header)
	updateHeadersByConfig(w.Header(), hc.ResponseHeaders)
	w.WriteHeader(res.StatusCode)

	copyBuf := copyBufPool.Get()
	copyBuf.B = bytesutil.ResizeNoCopyNoOverallocate(copyBuf.B, 16*1024)
	_, err = io.CopyBuffer(w, res.Body, copyBuf.B)
	copyBufPool.Put(copyBuf)
	_ = res.Body.Close()
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

func updateHeadersByConfig(headers http.Header, config []*Header) {
	for _, h := range config {
		if h.Value == "" {
			headers.Del(h.Name)
		} else {
			headers.Set(h.Name, h.Value)
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

func newRoundTripper(caFileOpt, certFileOpt, keyFileOpt, serverNameOpt string, insecureSkipVerifyP *bool) (http.RoundTripper, error) {
	caFile := *backendTLSCAFile
	if caFileOpt != "" {
		caFile = caFileOpt
	}
	certFile := *backendTLSCertFile
	if certFileOpt != "" {
		certFile = certFileOpt
	}
	keyFile := *backendTLSKeyFile
	if keyFileOpt != "" {
		keyFile = keyFileOpt
	}
	serverName := *backendTLSServerName
	if serverNameOpt != "" {
		serverName = serverNameOpt
	}
	insecureSkipVerify := *backendTLSInsecureSkipVerify
	if p := insecureSkipVerifyP; p != nil {
		insecureSkipVerify = *p
	}
	opts := &promauth.Options{
		TLSConfig: &promauth.TLSConfig{
			CAFile:             caFile,
			CertFile:           certFile,
			KeyFile:            keyFile,
			ServerName:         serverName,
			InsecureSkipVerify: insecureSkipVerify,
		},
	}
	cfg, err := opts.NewConfig()
	if err != nil {
		return nil, fmt.Errorf("cannot initialize promauth.Config: %w", err)
	}

	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.ResponseHeaderTimeout = *responseTimeout
	// Automatic compression must be disabled in order to fix https://github.com/VictoriaMetrics/VictoriaMetrics/issues/535
	tr.DisableCompression = true
	tr.IdleConnTimeout = *idleConnTimeout
	tr.MaxIdleConnsPerHost = *maxIdleConnsPerBackend
	if tr.MaxIdleConns != 0 && tr.MaxIdleConns < tr.MaxIdleConnsPerHost {
		tr.MaxIdleConns = tr.MaxIdleConnsPerHost
	}
	tr.DialContext = netutil.DialMaybeSRV

	rt := cfg.NewRoundTripper(tr)
	return rt, nil
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

See the docs at https://docs.victoriametrics.com/vmauth/ .
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
	// r contains reader for initial data reading
	r io.ReadCloser

	// buf is a buffer for data read from r. Buf size is limited by maxRequestBodySizeToRetry.
	// If more than maxRequestBodySizeToRetry is read from r, then cannotRetry is set to true.
	buf []byte

	// cannotRetry is set to true when more than maxRequestBodySizeToRetry are read from r.
	// In this case the read data cannot fit buf, so it cannot be re-read from buf.
	cannotRetry bool

	// bufComplete is set to true when buf contains complete request body read from r.
	bufComplete bool

	// offset is an offset at buf for the next data read if needReadBuf is set to true.
	offset int
}

// Read implements io.Reader interface
// tracks body reading requests
func (rtb *readTrackingBody) Read(p []byte) (int, error) {
	if rtb.offset < len(rtb.buf) {
		if rtb.cannotRetry {
			return 0, fmt.Errorf("cannot retry reading data from buf")
		}
		nb := copy(p, rtb.buf[rtb.offset:])
		rtb.offset += nb
		if rtb.bufComplete {
			if rtb.offset == len(rtb.buf) {
				return nb, io.EOF
			}
			return nb, nil
		}
		if nb < len(p) {
			nr, err := rtb.readFromStream(p[nb:])
			return nb + nr, err
		}
		return nb, nil
	}
	if rtb.bufComplete {
		return 0, io.EOF
	}
	return rtb.readFromStream(p)
}

func (rtb *readTrackingBody) readFromStream(p []byte) (int, error) {
	if rtb.r == nil {
		return 0, fmt.Errorf("cannot read data after closing the reader")
	}
	n, err := rtb.r.Read(p)
	if rtb.cannotRetry {
		return n, err
	}
	if rtb.offset+n > maxRequestBodySizeToRetry.IntN() {
		rtb.cannotRetry = true
	}
	if n > 0 {
		rtb.offset += n
		rtb.buf = append(rtb.buf, p[:n]...)
	}
	if err != nil {
		if err == io.EOF {
			rtb.bufComplete = true
			return n, err
		}
		rtb.cannotRetry = true
		return n, err
	}
	return n, nil
}

func (rtb *readTrackingBody) canRetry() bool {
	return !rtb.cannotRetry
}

// Close implements io.Closer interface.
func (rtb *readTrackingBody) Close() error {
	rtb.offset = 0

	// Close rtb.r only if the request body is completely read or if it is too big.
	// http.Roundtrip performs body.Close call even without any Read calls,
	// so this hack allows us to reuse request body.
	if rtb.bufComplete || rtb.cannotRetry {
		if rtb.r == nil {
			return nil
		}
		err := rtb.r.Close()
		rtb.r = nil
		return err
	}

	return nil
}

var readTrackingBodyPool sync.Pool

func getReadTrackingBody(origin io.ReadCloser, b int) *readTrackingBody {
	bufSize := 1024
	if b > 0 && b < maxRequestBodySizeToRetry.IntN() {
		bufSize = b
	}
	v := readTrackingBodyPool.Get()
	if v == nil {
		v = &readTrackingBody{
			buf: make([]byte, 0, bufSize),
		}
	}
	rtb := v.(*readTrackingBody)
	rtb.r = origin
	if bufSize > cap(rtb.buf) {
		rtb.buf = make([]byte, 0, bufSize)
	}

	return rtb
}

func putReadTrackingBody(rtb *readTrackingBody) {
	if rtb.r != nil {
		_ = rtb.r.Close()
	}
	rtb.r = nil
	rtb.buf = rtb.buf[:0]
	rtb.offset = 0
	rtb.cannotRetry = false
	rtb.bufComplete = false

	readTrackingBodyPool.Put(rtb)
}
