package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
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

	"github.com/VictoriaMetrics/metrics"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/buildinfo"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/envflag"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httpserver"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/netutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/procutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/pushmetrics"
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
	failTimeout               = flag.Duration("failTimeout", 3*time.Second, "Sets a delay period for load balancing to skip a malfunctioning backend")
	maxRequestBodySizeToRetry = flagutil.NewBytes("maxRequestBodySizeToRetry", 16*1024, "The maximum request body size, which can be cached and re-tried at other backends. "+
		"Bigger values may require more memory")
	backendTLSInsecureSkipVerify = flag.Bool("backend.tlsInsecureSkipVerify", false, "Whether to skip TLS verification when connecting to backends over HTTPS. "+
		"See https://docs.victoriametrics.com/vmauth.html#backend-tls-setup")
	backendTLSCAFile = flag.String("backend.TLSCAFile", "", "Optional path to TLS root CA file, which is used for TLS verification when connecting to backends over HTTPS. "+
		"See https://docs.victoriametrics.com/vmauth.html#backend-tls-setup")
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
	up, hc, retryStatusCodes, dropSrcPathPrefixParts := ui.getURLPrefixAndHeaders(u)
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
		up, hc, retryStatusCodes = ui.DefaultURL, ui.HeadersConf, ui.RetryStatusCodes
		isDefault = true
	}
	maxAttempts := up.getBackendsCount()
	if maxAttempts > 1 {
		r.Body = &readTrackingBody{
			r: r.Body,
		}
	}
	for i := 0; i < maxAttempts; i++ {
		bu := up.getLeastLoadedBackendURL()
		targetURL := bu.url
		// Don't change path and add request_path query param for default route.
		if isDefault {
			query := targetURL.Query()
			query.Set("request_path", u.Path)
			targetURL.RawQuery = query.Encode()
		} else { // Update path for regular routes.
			targetURL = mergeURLs(targetURL, u, dropSrcPathPrefixParts)
		}
		ok := tryProcessingRequest(w, r, targetURL, hc, retryStatusCodes, ui.httpTransport)
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

func tryProcessingRequest(w http.ResponseWriter, r *http.Request, targetURL *url.URL, hc HeadersConf, retryStatusCodes []int, transport *http.Transport) bool {
	// This code has been copied from net/http/httputil/reverseproxy.go
	req := sanitizeRequestHeaders(r)
	req.URL = targetURL
	req.Host = targetURL.Host
	updateHeadersByConfig(req.Header, hc.RequestHeaders)
	res, err := transport.RoundTrip(req)
	rtb, rtbOK := req.Body.(*readTrackingBody)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			// Do not retry canceled or timed out requests
			remoteAddr := httpserver.GetQuotedRemoteAddr(r)
			requestURI := httpserver.GetRequestURI(r)
			logger.Warnf("remoteAddr: %s; requestURI: %s; error when proxying response body from %s: %s", remoteAddr, requestURI, targetURL, err)
			return true
		}
		if !rtbOK || !rtb.canRetry() {
			// Request body cannot be re-sent to another backend. Return the error to the client then.
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
		// it explicitly reads request body, which may fail retries.
		logger.Warnf("remoteAddr: %s; requestURI: %s; retrying the request to %s because of response error: %s", remoteAddr, req.URL, targetURL, err)
		return false
	}
	if (rtbOK && rtb.canRetry()) && hasInt(retryStatusCodes, res.StatusCode) {
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
	if err != nil && !netutil.IsTrivialNetworkError(err) {
		remoteAddr := httpserver.GetQuotedRemoteAddr(r)
		requestURI := httpserver.GetRequestURI(r)
		logger.Warnf("remoteAddr: %s; requestURI: %s; error when proxying response body from %s: %s", remoteAddr, requestURI, targetURL, err)
		return true
	}
	return true
}

func hasInt(a []int, n int) bool {
	for _, x := range a {
		if x == n {
			return true
		}
	}
	return false
}

var copyBufPool bytesutil.ByteBufferPool

func copyHeader(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}

func updateHeadersByConfig(headers http.Header, config []Header) {
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

func getTransport(insecureSkipVerifyP *bool, caFile string) (*http.Transport, error) {
	if insecureSkipVerifyP == nil {
		insecureSkipVerifyP = backendTLSInsecureSkipVerify
	}
	insecureSkipVerify := *insecureSkipVerifyP
	if caFile == "" {
		caFile = *backendTLSCAFile
	}

	bb := bbPool.Get()
	defer bbPool.Put(bb)

	bb.B = appendTransportKey(bb.B[:0], insecureSkipVerify, caFile)

	transportMapLock.Lock()
	defer transportMapLock.Unlock()

	tr := transportMap[string(bb.B)]
	if tr == nil {
		trLocal, err := newTransport(insecureSkipVerify, caFile)
		if err != nil {
			return nil, err
		}
		transportMap[string(bb.B)] = trLocal
		tr = trLocal
	}

	return tr, nil
}

var transportMap = make(map[string]*http.Transport)
var transportMapLock sync.Mutex

func appendTransportKey(dst []byte, insecureSkipVerify bool, caFile string) []byte {
	dst = encoding.MarshalBool(dst, insecureSkipVerify)
	dst = encoding.MarshalBytes(dst, bytesutil.ToUnsafeBytes(caFile))
	return dst
}

var bbPool bytesutil.ByteBufferPool

func newTransport(insecureSkipVerify bool, caFile string) (*http.Transport, error) {
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.ResponseHeaderTimeout = *responseTimeout
	// Automatic compression must be disabled in order to fix https://github.com/VictoriaMetrics/VictoriaMetrics/issues/535
	tr.DisableCompression = true
	tr.MaxIdleConnsPerHost = *maxIdleConnsPerBackend
	if tr.MaxIdleConns != 0 && tr.MaxIdleConns < tr.MaxIdleConnsPerHost {
		tr.MaxIdleConns = tr.MaxIdleConnsPerHost
	}
	tlsCfg := tr.TLSClientConfig
	if tlsCfg == nil {
		tlsCfg = &tls.Config{}
		tr.TLSClientConfig = tlsCfg
	}
	if insecureSkipVerify || caFile != "" {
		tlsCfg.ClientSessionCache = tls.NewLRUClientSessionCache(0)
		tlsCfg.InsecureSkipVerify = insecureSkipVerify
		if caFile != "" {
			data, err := fs.ReadFileOrHTTP(caFile)
			if err != nil {
				return nil, fmt.Errorf("cannot read tls_ca_file: %w", err)
			}
			rootCA := x509.NewCertPool()
			if !rootCA.AppendCertsFromPEM(data) {
				return nil, fmt.Errorf("cannot parse data read from tls_ca_file %q", caFile)
			}
			tlsCfg.RootCAs = rootCA
		}
	}
	return tr, nil
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

	// needReadBuf is set to true when Read() must be performed from buf instead of r.
	needReadBuf bool

	// offset is an offset at buf for the next data read if needReadBuf is set to true.
	offset int
}

// Read implements io.Reader interface
// tracks body reading requests
func (rtb *readTrackingBody) Read(p []byte) (int, error) {
	if rtb.needReadBuf {
		if rtb.offset >= len(rtb.buf) {
			return 0, io.EOF
		}
		n := copy(p, rtb.buf[rtb.offset:])
		rtb.offset += n
		return n, nil
	}

	if rtb.r == nil {
		return 0, fmt.Errorf("cannot read data after closing the reader")
	}

	n, err := rtb.r.Read(p)
	if rtb.cannotRetry {
		return n, err
	}
	if len(rtb.buf)+n > maxRequestBodySizeToRetry.IntN() {
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
	if len(rtb.buf) > 0 && !rtb.needReadBuf {
		return false
	}
	return true
}

// Close implements io.Closer interface.
func (rtb *readTrackingBody) Close() error {
	rtb.offset = 0
	if rtb.bufComplete {
		rtb.needReadBuf = true
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
