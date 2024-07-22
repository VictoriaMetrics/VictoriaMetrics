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
	idleConnTimeout = flag.Duration("idleConnTimeout", 50*time.Second, "The timeout for HTTP keep-alive connections to backend services. "+
		"It is recommended setting this value to values smaller than -http.idleConnTimeout set at backend services")
	responseTimeout       = flag.Duration("responseTimeout", 5*time.Minute, "The timeout for receiving a response from backend")
	maxConcurrentRequests = flag.Int("maxConcurrentRequests", 1000, "The maximum number of concurrent requests vmauth can process. Other requests are rejected with "+
		"'429 Too Many Requests' http status code. See also -maxConcurrentPerUserRequests and -maxIdleConnsPerBackend command-line options")
	maxConcurrentPerUserRequests = flag.Int("maxConcurrentPerUserRequests", 300, "The maximum number of concurrent requests vmauth can process per each configured user. "+
		"Other requests are rejected with '429 Too Many Requests' http status code. See also -maxConcurrentRequests command-line option and max_concurrent_requests option "+
		"in per-user config")
	reloadAuthKey        = flagutil.NewPassword("reloadAuthKey", "Auth key for /-/reload http endpoint. It must be passed via authKey query arg. It overrides -httpAuth.*")
	logInvalidAuthTokens = flag.Bool("logInvalidAuthTokens", false, "Whether to log requests with invalid auth tokens. "+
		`Such requests are always counted at vmauth_http_request_errors_total{reason="invalid_auth_token"} metric, which is exposed at /metrics page`)
	failTimeout               = flag.Duration("failTimeout", 3*time.Second, "Sets a delay period for load balancing to skip a malfunctioning backend")
	maxRequestBodySizeToRetry = flagutil.NewBytes("maxRequestBodySizeToRetry", 16*1024, "The maximum request body size, which can be cached and re-tried at other backends. "+
		"Bigger values may require more memory. Zero or negative value disables caching of request body. This may be useful when proxying data ingestion requests")
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
		if !httpserver.CheckAuthFlag(w, r, reloadAuthKey) {
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

		handleMissingAuthorizationError(w)
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
				handleMissingAuthorizationError(w)
				return
			}
			missingRouteRequests.Inc()
			httpserver.Errorf(w, r, "missing route for %s", u.String())
			return
		}
		up, hc = ui.DefaultURL, ui.HeadersConf
		isDefault = true
	}

	rtb := getReadTrackingBody(r.Body, maxRequestBodySizeToRetry.IntN())
	defer putReadTrackingBody(rtb)
	r.Body = rtb

	maxAttempts := up.getBackendsCount()
	for i := 0; i < maxAttempts; i++ {
		bu := up.getBackendURL()
		if bu == nil {
			break
		}
		targetURL := bu.url
		// Don't change path and add request_path query param for default route.
		if isDefault {
			query := targetURL.Query()
			query.Set("request_path", u.String())
			targetURL.RawQuery = query.Encode()
		} else { // Update path for regular routes.
			targetURL = mergeURLs(targetURL, u, up.dropSrcPathPrefixParts)
		}

		wasLocalRetry := false
	again:
		ok, needLocalRetry := tryProcessingRequest(w, r, targetURL, hc, up.retryStatusCodes, ui)
		if needLocalRetry && !wasLocalRetry {
			wasLocalRetry = true
			goto again
		}

		bu.put()
		if ok {
			return
		}
		bu.setBroken()
	}
	err := &httpserver.ErrorWithStatusCode{
		Err:        fmt.Errorf("all the %d backends for the user %q are unavailable", up.getBackendsCount(), ui.name()),
		StatusCode: http.StatusServiceUnavailable,
	}
	httpserver.Errorf(w, r, "%s", err)
	ui.backendErrors.Inc()
}

func tryProcessingRequest(w http.ResponseWriter, r *http.Request, targetURL *url.URL, hc HeadersConf, retryStatusCodes []int, ui *UserInfo) (bool, bool) {
	req := sanitizeRequestHeaders(r)

	req.URL = targetURL
	req.Header.Set("User-Agent", "vmauth")
	updateHeadersByConfig(req.Header, hc.RequestHeaders)
	if hc.KeepOriginalHost == nil || !*hc.KeepOriginalHost {
		if host := getHostHeader(hc.RequestHeaders); host != "" {
			req.Host = host
		} else {
			req.Host = targetURL.Host
		}
	}

	rtb, rtbOK := req.Body.(*readTrackingBody)
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
			return true, false
		}
		if !rtbOK || !rtb.canRetry() {
			// Request body cannot be re-sent to another backend. Return the error to the client then.
			err = &httpserver.ErrorWithStatusCode{
				Err:        fmt.Errorf("cannot proxy the request to %s: %w", targetURL, err),
				StatusCode: http.StatusServiceUnavailable,
			}
			httpserver.Errorf(w, r, "%s", err)
			ui.backendErrors.Inc()
			return true, false
		}
		if netutil.IsTrivialNetworkError(err) {
			// Retry request at the same backend on trivial network errors, such as proxy idle timeout misconfiguration or socket close by OS
			return false, true
		}

		// Retry the request if its body wasn't read yet. This usually means that the backend isn't reachable.
		remoteAddr := httpserver.GetQuotedRemoteAddr(r)
		// NOTE: do not use httpserver.GetRequestURI
		// it explicitly reads request body, which may fail retries.
		logger.Warnf("remoteAddr: %s; requestURI: %s; retrying the request to %s because of response error: %s", remoteAddr, req.URL, targetURL, err)
		return false, false
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
			return true, false
		}
		// Retry requests at other backends if it matches retryStatusCodes.
		// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4893
		remoteAddr := httpserver.GetQuotedRemoteAddr(r)
		// NOTE: do not use httpserver.GetRequestURI
		// it explicitly reads request body, which may fail retries.
		logger.Warnf("remoteAddr: %s; requestURI: %s; retrying the request to %s because response status code=%d belongs to retry_status_codes=%d",
			remoteAddr, req.URL, targetURL, res.StatusCode, retryStatusCodes)
		return false, false
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
		return true, false
	}
	return true, false
}

var copyBufPool bytesutil.ByteBufferPool

func copyHeader(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}

func getHostHeader(headers []*Header) string {
	for _, h := range headers {
		if h.Name == "Host" {
			return h.Value
		}
	}
	return ""
}

func updateHeadersByConfig(dst http.Header, src []*Header) {
	for _, h := range src {
		if h.Value == "" {
			dst.Del(h.Name)
		} else {
			dst.Set(h.Name, h.Value)
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
	tr.DialContext = netutil.NewStatDialFunc("vmauth_backend")

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

func handleMissingAuthorizationError(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
	http.Error(w, "missing 'Authorization' request header", http.StatusUnauthorized)
}

func handleConcurrencyLimitError(w http.ResponseWriter, r *http.Request, err error) {
	w.Header().Add("Retry-After", "10")
	err = &httpserver.ErrorWithStatusCode{
		Err:        err,
		StatusCode: http.StatusTooManyRequests,
	}
	httpserver.Errorf(w, r, "%s", err)
}

// readTrackingBody must be obtained via getReadTrackingBody()
type readTrackingBody struct {
	// maxBodySize is the maximum body size to cache in buf.
	//
	// Bigger bodies cannot be retried.
	maxBodySize int

	// r contains reader for initial data reading
	r io.ReadCloser

	// buf is a buffer for data read from r. Buf size is limited by maxBodySize.
	// If more than maxBodySize is read from r, then cannotRetry is set to true.
	buf []byte

	// readBuf points to the cached data at buf, which must be read in the next call to Read().
	readBuf []byte

	// cannotRetry is set to true when more than maxBodySize bytes are read from r.
	// In this case the read data cannot fit buf, so it cannot be re-read from buf.
	cannotRetry bool

	// bufComplete is set to true when buf contains complete request body read from r.
	bufComplete bool
}

func (rtb *readTrackingBody) reset() {
	rtb.maxBodySize = 0
	rtb.r = nil
	rtb.buf = rtb.buf[:0]
	rtb.readBuf = nil
	rtb.cannotRetry = false
	rtb.bufComplete = false
}

func getReadTrackingBody(r io.ReadCloser, maxBodySize int) *readTrackingBody {
	v := readTrackingBodyPool.Get()
	if v == nil {
		v = &readTrackingBody{}
	}
	rtb := v.(*readTrackingBody)

	if maxBodySize < 0 {
		maxBodySize = 0
	}
	rtb.maxBodySize = maxBodySize

	if r == nil {
		// This is GET request without request body
		r = (*zeroReader)(nil)
	}
	rtb.r = r
	return rtb
}

type zeroReader struct{}

func (r *zeroReader) Read(_ []byte) (int, error) {
	return 0, io.EOF
}
func (r *zeroReader) Close() error {
	return nil
}

func putReadTrackingBody(rtb *readTrackingBody) {
	rtb.reset()
	readTrackingBodyPool.Put(rtb)
}

var readTrackingBodyPool sync.Pool

// Read implements io.Reader interface.
func (rtb *readTrackingBody) Read(p []byte) (int, error) {
	if len(rtb.readBuf) > 0 {
		n := copy(p, rtb.readBuf)
		rtb.readBuf = rtb.readBuf[n:]
		return n, nil
	}

	if rtb.r == nil {
		if rtb.bufComplete {
			return 0, io.EOF
		}
		return 0, fmt.Errorf("cannot read client request body after closing client reader")
	}

	n, err := rtb.r.Read(p)
	if rtb.cannotRetry {
		return n, err
	}

	if len(rtb.buf)+n > rtb.maxBodySize {
		rtb.cannotRetry = true
		return n, err
	}
	rtb.buf = append(rtb.buf, p[:n]...)
	if err == io.EOF {
		rtb.bufComplete = true
	}
	return n, err
}

func (rtb *readTrackingBody) canRetry() bool {
	if rtb.cannotRetry {
		return false
	}
	if rtb.bufComplete {
		return true
	}
	return rtb.r != nil
}

// Close implements io.Closer interface.
func (rtb *readTrackingBody) Close() error {
	if !rtb.cannotRetry {
		rtb.readBuf = rtb.buf
	} else {
		rtb.readBuf = nil
	}

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
