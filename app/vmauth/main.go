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
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httputil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/ioutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/netutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/procutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/pushmetrics"
)

var (
	httpListenAddrs = flagutil.NewArrayString("httpListenAddr", "TCP address to listen for incoming http requests. "+
		"By default, serves internal API and proxy requests. "+
		" See also -tls, -httpListenAddr.useProxyProtocol and -httpInternalListenAddr.")
	httpInternalListenAddr = flagutil.NewArrayString("httpInternalListenAddr", "TCP address to listen for incoming internal API http requests. Such as /health, /-/reload, /debug/pprof, etc. "+
		"If flag is set, vmauth no longer serves internal API at -httpListenAddr.")
	useProxyProtocol = flagutil.NewArrayBool("httpListenAddr.useProxyProtocol", "Whether to use proxy protocol for connections accepted at the corresponding -httpListenAddr . "+
		"See https://www.haproxy.org/download/1.8/doc/proxy-protocol.txt . "+
		"With enabled proxy protocol http server cannot serve regular /metrics endpoint. Use -pushmetrics.url for metrics pushing")
	maxIdleConnsPerBackend = flag.Int("maxIdleConnsPerBackend", 100, "The maximum number of idle connections vmauth can open per each backend host")
	idleConnTimeout        = flag.Duration("idleConnTimeout", 50*time.Second, "The timeout for HTTP keep-alive connections to backend services. "+
		"It is recommended setting this value to values smaller than -http.idleConnTimeout set at backend services")
	responseTimeout = flag.Duration("responseTimeout", 5*time.Minute, "The timeout for receiving a response from backend")

	requestBufferSize = flagutil.NewBytes("requestBufferSize", 32*1024, "The size of the buffer for reading the request body before proxying the request to backends. "+
		"This allows reducing the comsumption of backend resources when processing requests from clients connected via slow networks. "+
		"Set to 0 to disable request buffering. See https://docs.victoriametrics.com/victoriametrics/vmauth/#request-body-buffering")
	maxRequestBodySizeToRetry = flagutil.NewBytes("maxRequestBodySizeToRetry", 16*1024, "The maximum request body size to buffer in memory for potential retries at other backends. "+
		"Request bodies larger than this size cannot be retried if the backend fails. Zero or negative value disables request body buffering and retries. "+
		"See also -requestBufferSize")

	maxConcurrentRequests = flag.Int("maxConcurrentRequests", 1000, "The maximum number of concurrent requests vmauth can process simultaneously. "+
		"Requests exceeding this limit are queued for up to -maxQueueDuration and then rejected with '429 Too Many Requests' http status code if the limit is still reached. "+
		"This protects vmauth itself from overloading and out-of-memory (OOM) failures. See also -maxConcurrentPerUserRequests "+
		"and https://docs.victoriametrics.com/victoriametrics/vmauth/#concurrency-limiting")
	maxConcurrentPerUserRequests = flag.Int("maxConcurrentPerUserRequests", 100, "The maximum number of concurrent requests vmauth can process per each configured user. "+
		"Requests exceeding this limit are queued for up to -maxQueueDuration and then rejected with '429 Too Many Requests' http status code if the limit is still reached. "+
		"This provides fairness and isolation between users, preventing a single user from consuming all the available resources. "+
		"It works in conjunction with -maxConcurrentRequests, which sets the global limit across all users. "+
		"This default can be overridden for individual users via max_concurrent_requests option in per-user config. "+
		"See https://docs.victoriametrics.com/victoriametrics/vmauth/#concurrency-limiting")
	maxQueueDuration = flag.Duration("maxQueueDuration", 10*time.Second, "The maximum duration to wait before rejecting incoming requests if concurrency limit "+
		"specified via -maxConcurrentRequests or -maxConcurrentPerUserRequests command-line flags is reached. "+
		"Requests are rejected with '429 Too Many Requests' http status code if the limit is still reached after the -maxQueueDuration duration. "+
		"This allows graceful handling of short spikes in concurrent requests. See https://docs.victoriametrics.com/victoriametrics/vmauth/#concurrency-limiting")

	reloadAuthKey        = flagutil.NewPassword("reloadAuthKey", "Auth key for /-/reload http endpoint. It must be passed via authKey query arg. It overrides -httpAuth.*")
	logInvalidAuthTokens = flag.Bool("logInvalidAuthTokens", false, "Whether to log requests with invalid auth tokens. "+
		`Such requests are always counted at vmauth_http_request_errors_total{reason="invalid_auth_token"} metric, which is exposed at /metrics page`)
	failTimeout = flag.Duration("failTimeout", 3*time.Second, "Sets a delay period for load balancing to skip a malfunctioning backend")

	backendTLSInsecureSkipVerify = flag.Bool("backend.tlsInsecureSkipVerify", false, "Whether to skip TLS verification when connecting to backends over HTTPS. "+
		"See https://docs.victoriametrics.com/victoriametrics/vmauth/#backend-tls-setup")
	backendTLSCAFile = flag.String("backend.TLSCAFile", "", "Optional path to TLS root CA file, which is used for TLS verification when connecting to backends over HTTPS. "+
		"See https://docs.victoriametrics.com/victoriametrics/vmauth/#backend-tls-setup")
	backendTLSCertFile = flag.String("backend.TLSCertFile", "", "Optional path to TLS client certificate file, which must be sent to HTTPS backend. "+
		"See https://docs.victoriametrics.com/victoriametrics/vmauth/#backend-tls-setup")
	backendTLSKeyFile = flag.String("backend.TLSKeyFile", "", "Optional path to TLS client key file, which must be sent to HTTPS backend. "+
		"See https://docs.victoriametrics.com/victoriametrics/vmauth/#backend-tls-setup")
	backendTLSServerName = flag.String("backend.TLSServerName", "", "Optional TLS ServerName, which must be sent to HTTPS backend. "+
		"See https://docs.victoriametrics.com/victoriametrics/vmauth/#backend-tls-setup")
	dryRun                   = flag.Bool("dryRun", false, "Whether to check only config files without running vmauth. The auth configuration file is validated. The -auth.config flag must be specified.")
	removeXFFHTTPHeaderValue = flag.Bool(`removeXFFHTTPHeaderValue`, false, "Whether to remove the X-Forwarded-For HTTP header value from client requests before forwarding them to the backend. "+
		"Recommended when vmauth is exposed to the internet.")
)

func main() {
	// Write flags and help message to stdout, since it is easier to grep or pipe.
	flag.CommandLine.SetOutput(os.Stdout)
	flag.Usage = usage
	envflag.Parse()
	buildinfo.Init()
	logger.Init()

	if *dryRun {
		if len(*authConfigPath) == 0 {
			logger.Fatalf("missing required `-auth.config` command-line flag")
		}
		if _, err := reloadAuthConfig(); err != nil {
			logger.Fatalf("failed to parse %q: %s", *authConfigPath, err)
		}
		return
	}

	listenAddrs := *httpListenAddrs
	if len(listenAddrs) == 0 {
		listenAddrs = []string{":8427"}
	}
	logger.Infof("starting vmauth at %q...", listenAddrs)
	startTime := time.Now()
	initAuthConfig()

	disableInternalRoutes := len(*httpInternalListenAddr) > 0
	rh := requestHandlerWithInternalRoutes
	if disableInternalRoutes {
		rh = requestHandler
	}

	go httpserver.Serve(listenAddrs, rh, httpserver.ServeOptions{
		UseProxyProtocol: useProxyProtocol,
		// built-in routes will be exposed at *httpInternalListenAddr
		DisableBuiltinRoutes: disableInternalRoutes,
	})

	if len(*httpInternalListenAddr) > 0 {
		go httpserver.Serve(*httpInternalListenAddr, internalRequestHandler, httpserver.ServeOptions{})
	}
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

func internalRequestHandler(w http.ResponseWriter, r *http.Request) bool {
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
	return false
}

func requestHandlerWithInternalRoutes(w http.ResponseWriter, r *http.Request) bool {
	if internalRequestHandler(w, r) {
		return true
	}
	return requestHandler(w, r)
}

func requestHandler(w http.ResponseWriter, r *http.Request) bool {
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
		uu := authConfig.Load().UnauthorizedUser
		if uu != nil {
			processUserRequest(w, r, uu)
			return true
		}

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

	ctx, cancel := context.WithTimeout(r.Context(), *maxQueueDuration)
	defer cancel()

	// Acquire global concurrency limit.
	if err := beginConcurrencyLimit(ctx); err != nil {
		handleConcurrencyLimitError(w, r, err)
		return
	}
	defer endConcurrencyLimit()

	// Set read deadline for reading the initial chunk for the request body.
	rc := http.NewResponseController(w)
	deadline, ok := ctx.Deadline()
	if !ok {
		logger.Panicf("BUG: expecting valid deadline for the context")
	}
	if err := rc.SetReadDeadline(deadline); err != nil {
		logger.Panicf("BUG: cannot set read deadline: %s", err)
	}

	// Read the initial chunk for the request body.
	userName := ui.name()
	if userName == "" {
		userName = "unauthorized"
	}
	bb, err := bufferRequestBody(ctx, r.Body, userName)
	if err != nil {
		httpserver.Errorf(w, r, "%s", err)
		return
	}
	r.Body = bb

	// Disable the read deadline for the rest of the request body.
	if err := rc.SetReadDeadline(time.Time{}); err != nil {
		logger.Panicf("BUG: cannot reset read deadline: %s", err)
	}

	// Acquire concurrency limit for the given user.
	if err := ui.beginConcurrencyLimit(ctx); err != nil {
		handleConcurrencyLimitError(w, r, err)
		return
	}
	defer ui.endConcurrencyLimit()

	// Process the request.
	processRequest(w, r, ui)
}

func beginConcurrencyLimit(ctx context.Context) error {
	concurrencyLimitOnce.Do(concurrencyLimitInit)
	select {
	case concurrencyLimitCh <- struct{}{}:
		return nil
	default:
		// The -maxConcurrentRequests are executed. Wait until some of the requests are finished,
		// so the current request could be executed.
		// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/10078
		select {
		case concurrencyLimitCh <- struct{}{}:
			return nil
		case <-ctx.Done():
			err := ctx.Err()
			if errors.Is(err, context.DeadlineExceeded) {
				// The current request couldn't be executed until the request timeout.
				concurrentRequestsLimitReached.Inc()
				return fmt.Errorf("cannot start executing the request during -maxQueueDuration=%s because -maxConcurrentRequests=%d concurrent requests are executed",
					*maxQueueDuration, cap(concurrencyLimitCh))
			}
			return fmt.Errorf("cannot start executing the request because -maxConcurrentRequests=%d concurrent requests are executed: %w", cap(concurrencyLimitCh), err)
		}
	}
}

func endConcurrencyLimit() {
	<-concurrencyLimitCh
}

func bufferRequestBody(ctx context.Context, r io.ReadCloser, userName string) (io.ReadCloser, error) {
	if r == nil {
		// This is a GET request with nil reader.
		return nil, nil
	}

	maxBufSize := max(requestBufferSize.IntN(), maxRequestBodySizeToRetry.IntN())
	if maxBufSize <= 0 {
		return r, nil
	}

	lr := ioutil.GetLimitedReader(r, int64(maxBufSize))
	defer ioutil.PutLimitedReader(lr)

	start := time.Now()
	buf, err := io.ReadAll(lr)
	bufferRequestBodyDuration.UpdateDuration(start)

	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			rejectSlowClientRequests.Inc()

			d := time.Since(start)

			return nil, &httpserver.ErrorWithStatusCode{
				Err: fmt.Errorf("reject request from the user %s because the request body couldn't be read in -maxQueueDuration=%s; read %d bytes in %s",
					userName, *maxQueueDuration, len(buf), d.Truncate(time.Second)),
				StatusCode: http.StatusBadRequest,
			}
		}

		return nil, &httpserver.ErrorWithStatusCode{
			Err:        fmt.Errorf("cannot read request body: %w", err),
			StatusCode: http.StatusBadRequest,
		}
	}

	bb := newBufferedBody(r, buf, maxBufSize)
	return bb, nil
}

func processRequest(w http.ResponseWriter, r *http.Request, ui *UserInfo) {
	u := normalizeURL(r.URL)
	up, hc := ui.getURLPrefixAndHeaders(u, r.Host, r.Header)
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
			var di string
			if ui.DumpRequestOnErrors {
				di = debugInfo(u, r)
			}
			httpserver.Errorf(w, r, "missing route for %q%s", u.String(), di)
			return
		}
		up, hc = ui.DefaultURL, ui.HeadersConf
		isDefault = true
	}

	maxAttempts := up.getBackendsCount()
	for i := 0; i < maxAttempts; i++ {
		bu := up.getBackendURL()
		if bu == nil {
			break
		}
		targetURL := bu.url
		if isDefault {
			// Don't change path and add request_path query param for default route.
			query := targetURL.Query()
			query.Set("request_path", u.String())
			targetURL.RawQuery = query.Encode()
		} else {
			// Update path for regular routes.
			targetURL = mergeURLs(targetURL, u, up.dropSrcPathPrefixParts, up.mergeQueryArgs)
		}

		wasLocalRetry := false
	again:
		ok, needLocalRetry := tryProcessingRequest(w, r, targetURL, hc, up.retryStatusCodes, ui, bu)
		if needLocalRetry && !wasLocalRetry {
			wasLocalRetry = true
			goto again
		}

		bu.put()
		if ok {
			return
		}

		bu.setBroken()
		ui.backendErrors.Inc()
	}
	err := &httpserver.ErrorWithStatusCode{
		Err:        fmt.Errorf("all the %d backends for the user %q are unavailable", up.getBackendsCount(), ui.name()),
		StatusCode: http.StatusBadGateway,
	}
	httpserver.Errorf(w, r, "%s", err)
	ui.requestErrors.Inc()
}

func tryProcessingRequest(w http.ResponseWriter, r *http.Request, targetURL *url.URL, hc HeadersConf, retryStatusCodes []int, ui *UserInfo, bu *backendURL) (bool, bool) {
	ui.backendRequests.Inc()
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

	bb, bbOK := req.Body.(*bufferedBody)
	canRetry := !bbOK || bb.canRetry()

	res, err := ui.rt.RoundTrip(req)

	if errors.Is(r.Context().Err(), context.Canceled) {
		// Do not retry canceled requests.
		clientCanceledRequests.Inc()
		return true, false
	}

	if err != nil {
		if !canRetry {
			// Request body cannot be re-sent to another backend. Return the error to the client then.
			err = &httpserver.ErrorWithStatusCode{
				Err:        fmt.Errorf("cannot proxy the request to %s: %w", targetURL, err),
				StatusCode: http.StatusServiceUnavailable,
			}
			httpserver.Errorf(w, r, "%s", err)
			ui.backendErrors.Inc()
			ui.requestErrors.Inc()
			bu.setBroken()
			return true, false
		}
		if netutil.IsTrivialNetworkError(err) {
			// Retry request at the same backend on trivial network errors, such as proxy idle timeout misconfiguration or socket close by OS
			if bbOK {
				bb.resetReader()
			}
			return false, true
		}

		// Retry the request at another backend
		remoteAddr := httpserver.GetQuotedRemoteAddr(r)
		requestURI := httpserver.GetRequestURI(r)
		logger.Warnf("remoteAddr: %s; requestURI: %s; request to %s failed: %s, retrying the request at another backend", remoteAddr, requestURI, targetURL, err)
		if bbOK {
			bb.resetReader()
		}
		return false, false
	}
	if slices.Contains(retryStatusCodes, res.StatusCode) {
		if !canRetry {
			// If we get an error from the retry_status_codes list, but cannot execute retry,
			// we consider such a request an error as well.
			err := &httpserver.ErrorWithStatusCode{
				Err: fmt.Errorf("got response status code=%d from %s, but cannot retry the request at another backend, because the request body has been already consumed",
					res.StatusCode, targetURL),
				StatusCode: http.StatusServiceUnavailable,
			}
			httpserver.Errorf(w, r, "%s", err)
			ui.backendErrors.Inc()
			ui.requestErrors.Inc()
			return true, false
		}

		// Retry requests at other backends if it matches retryStatusCodes.
		// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4893
		remoteAddr := httpserver.GetQuotedRemoteAddr(r)
		requestURI := httpserver.GetRequestURI(r)
		logger.Warnf("remoteAddr: %s; requestURI: %s; request to %s failed, retrying the request at another backend because response status code=%d belongs to retry_status_codes=%d",
			remoteAddr, requestURI, targetURL, res.StatusCode, retryStatusCodes)
		if bbOK {
			bb.resetReader()
		}
		return false, false
	}
	removeHopHeaders(res.Header)
	copyHeader(w.Header(), res.Header)
	updateHeadersByConfig(w.Header(), hc.ResponseHeaders)
	w.WriteHeader(res.StatusCode)

	err = copyStreamToClient(w, res.Body)
	_ = res.Body.Close()

	if errors.Is(r.Context().Err(), context.Canceled) {
		// Do not retry canceled requests.
		clientCanceledRequests.Inc()
		return true, false
	}

	if err != nil && !netutil.IsTrivialNetworkError(err) {
		remoteAddr := httpserver.GetQuotedRemoteAddr(r)
		requestURI := httpserver.GetRequestURI(r)
		logger.Warnf("remoteAddr: %s; requestURI: %s; error when proxying response body from %s: %s", remoteAddr, requestURI, targetURL, err)
		ui.requestErrors.Inc()
		return true, false
	}
	return true, false
}

func copyStreamToClient(client io.Writer, backend io.Reader) error {
	copyBuf := copyBufPool.Get()
	copyBuf.B = bytesutil.ResizeNoCopyNoOverallocate(copyBuf.B, 16*1024)
	defer copyBufPool.Put(copyBuf)
	buf := copyBuf.B

	flusher, ok := client.(http.Flusher)
	if !ok {
		logger.Panicf("BUG: client must implement net/http.Flusher interface; got %T", client)
	}

	for {
		n, backendErr := backend.Read(buf)
		if n > 0 {
			data := buf[:n]
			n, clientErr := client.Write(data)
			if clientErr != nil {
				return fmt.Errorf("cannot write data to client: %w", clientErr)
			}
			if n != len(data) {
				logger.Panicf("BUG: unexpected number of bytes written returned by client.Write; got %d; want %d", n, len(data))
			}
			// Flush the read data from the backend to the client as fast as possible
			// in order to reduce delays for data propagation.
			// See https://github.com/VictoriaMetrics/VictoriaLogs/issues/667
			flusher.Flush()
		}
		if backendErr != nil {
			if backendErr == io.EOF {
				return nil
			}
			return fmt.Errorf("cannot read data from backend: %w", backendErr)
		}
	}
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
		if len(prior) > 0 && !*removeXFFHTTPHeaderValue {
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
	clientCanceledRequests   = metrics.NewCounter(`vmauth_http_request_errors_total{reason="client_canceled"}`)
	rejectSlowClientRequests = metrics.NewCounter(`vmauth_http_request_errors_total{reason="reject_slow_client"}`)

	bufferRequestBodyDuration = metrics.NewSummary(`vmauth_buffer_request_body_duration_seconds`)
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

	tr := httputil.NewTransport(false, "vmauth_backend")
	tr.ResponseHeaderTimeout = *responseTimeout
	// Automatic compression must be disabled in order to fix https://github.com/VictoriaMetrics/VictoriaMetrics/issues/535
	tr.DisableCompression = true
	tr.IdleConnTimeout = *idleConnTimeout
	tr.MaxIdleConnsPerHost = *maxIdleConnsPerBackend
	if tr.MaxIdleConns != 0 && tr.MaxIdleConns < tr.MaxIdleConnsPerHost {
		tr.MaxIdleConns = tr.MaxIdleConnsPerHost
	}

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

See the docs at https://docs.victoriametrics.com/victoriametrics/vmauth/ .
`
	flagutil.Usage(s)
}

func handleMissingAuthorizationError(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
	http.Error(w, "missing 'Authorization' request header", http.StatusUnauthorized)
}

func handleConcurrencyLimitError(w http.ResponseWriter, r *http.Request, err error) {
	if errors.Is(r.Context().Err(), context.Canceled) {
		// Do not return any response for the request canceled by the client,
		// since the connection to the client is already closed.
		clientCanceledRequests.Inc()
		return
	}

	w.Header().Add("Retry-After", "10")
	err = &httpserver.ErrorWithStatusCode{
		Err:        err,
		StatusCode: http.StatusTooManyRequests,
	}
	httpserver.Errorf(w, r, "%s", err)
}

// bufferedBody serves two purposes:
//  1. Enables request retries when the body size does not exceed maxBodySize
//     by fully buffering the body in memory.
//  2. Prevents slow clients from reducing effective server capacity by
//     buffering the request body before acquiring a per-user concurrency slot.
//
// See bufferRequestBody for details on how bufferedBody is used.
type bufferedBody struct {
	// r contains reader for reading the data after buf is read.
	//
	// r is nil if buf contains all the data.
	r io.ReadCloser

	// buf contains the initial buffer read from r.
	buf []byte

	// bufOffset is the offset at buf for already read bytes.
	bufOffset int

	// cannotRetry is set to true after Close() call on non-nil r.
	cannotRetry bool
}

func newBufferedBody(r io.ReadCloser, buf []byte, maxBufSize int) *bufferedBody {
	// Do not use sync.Pool here, since http.RoundTrip may still use request body after return.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/8051

	if len(buf) < maxBufSize {
		// Read the full request body into buf.
		r = nil
	}

	return &bufferedBody{
		r:   r,
		buf: buf,
	}
}

// Read implements io.Reader interface.
func (bb *bufferedBody) Read(p []byte) (int, error) {
	if bb.cannotRetry {
		return 0, fmt.Errorf("cannot read already closed body")
	}
	if bb.bufOffset < len(bb.buf) {
		n := copy(p, bb.buf[bb.bufOffset:])
		bb.bufOffset += n
		return n, nil
	}
	if bb.r == nil {
		return 0, io.EOF
	}
	return bb.r.Read(p)
}

func (bb *bufferedBody) canRetry() bool {
	return bb.r == nil
}

// Close implements io.Closer interface.
func (bb *bufferedBody) Close() error {
	bb.resetReader()
	if bb.r != nil {
		bb.cannotRetry = true
		return bb.r.Close()
	}
	return nil
}

func (bb *bufferedBody) resetReader() {
	bb.bufOffset = 0
}

func debugInfo(u *url.URL, r *http.Request) string {
	s := &strings.Builder{}
	fmt.Fprintf(s, " (host: %q; ", r.Host)
	fmt.Fprintf(s, "path: %q; ", u.Path)
	fmt.Fprintf(s, "args: %q; ", u.Query().Encode())
	fmt.Fprint(s, "headers:")
	_ = r.Header.WriteSubset(s, nil)
	fmt.Fprint(s, ")")
	return s.String()
}
