package discoveryutil

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/VictoriaMetrics/metrics"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/httputil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/netutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/proxy"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/timerpool"
)

var (
	maxConcurrency = flag.Int("promscrape.discovery.concurrency", 100, "The maximum number of concurrent requests to Prometheus autodiscovery API (Consul, Kubernetes, etc.)")
	maxWaitTime    = flag.Duration("promscrape.discovery.concurrentWaitTime", time.Minute, "The maximum duration for waiting to perform API requests "+
		"if more than -promscrape.discovery.concurrency requests are simultaneously performed")
)

var defaultClient = &http.Client{
	Timeout: 30 * time.Second,
}

var (
	concurrencyLimitCh     chan struct{}
	concurrencyLimitChOnce sync.Once
)

const (
	// BlockingClientReadTimeout is the maximum duration for waiting the response from GetBlockingAPI*
	BlockingClientReadTimeout = 10 * time.Minute

	// DefaultClientReadTimeout is the maximum duration for waiting the response from GetAPI*
	DefaultClientReadTimeout = time.Minute
)

// RequestCallback is called on the request before sending the request to the server.
type RequestCallback func(req *http.Request)

// ResponseCallback is called on the response before validating and returning the response to the caller.
type ResponseCallback func(resp *http.Response)

func concurrencyLimitChInit() {
	concurrencyLimitCh = make(chan struct{}, *maxConcurrency)
}

// GetHTTPClient returns default client for http API requests.
func GetHTTPClient() *http.Client {
	return defaultClient
}

// Client is http client, which talks to the given apiServer passed to NewClient().
type Client struct {
	// client is used for short requests.
	client *HTTPClient

	// blockingClient is used for long-polling requests.
	blockingClient *HTTPClient

	apiServer string

	setHTTPHeaders      func(req *http.Request) error
	setHTTPProxyHeaders func(req *http.Request) error

	clientCtx    context.Context
	clientCancel context.CancelFunc
}

// HTTPClient is a wrapper around http.Client with timeouts.
type HTTPClient struct {
	client      *http.Client
	ReadTimeout time.Duration
}

func (hc *HTTPClient) stop() {
	// Close idle connections to server in order to free up resources.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4724
	hc.client.CloseIdleConnections()
}

// NewClient returns new Client for the given args.
func NewClient(apiServer string, ac *promauth.Config, proxyURL *proxy.URL, proxyAC *promauth.Config, httpCfg *promauth.HTTPClientConfig) (*Client, error) {
	u, err := url.Parse(apiServer)
	if err != nil {
		return nil, fmt.Errorf("cannot parse apiServer=%q: %w", apiServer, err)
	}

	dialFunc := netutil.NewStatDialFunc("vm_promscrape_discovery")
	if u.Scheme == "unix" {
		// special case for unix socket connection
		dialAddr := u.Path
		apiServer = "http://unix"
		dialFunc = func(ctx context.Context, _, _ string) (net.Conn, error) {
			return netutil.Dialer.DialContext(ctx, "unix", dialAddr)
		}
	}

	var proxyURLFunc func(*http.Request) (*url.URL, error)
	if pu := proxyURL.GetURL(); pu != nil {
		proxyURLFunc = http.ProxyURL(pu)
	}

	tr := httputil.NewTransport(false, "vm_promscrape_discovery")
	tr.Proxy = proxyURLFunc
	tr.TLSHandshakeTimeout = 10 * time.Second
	tr.MaxIdleConnsPerHost = *maxConcurrency
	tr.ResponseHeaderTimeout = DefaultClientReadTimeout
	tr.DialContext = dialFunc

	client := &http.Client{
		Timeout:   DefaultClientReadTimeout,
		Transport: ac.NewRoundTripper(tr),
	}

	trBlocking := httputil.NewTransport(false, "vm_promscrape_discovery")
	trBlocking.Proxy = proxyURLFunc
	trBlocking.TLSHandshakeTimeout = 10 * time.Second
	trBlocking.MaxIdleConnsPerHost = 1000
	trBlocking.ResponseHeaderTimeout = BlockingClientReadTimeout
	trBlocking.DialContext = dialFunc

	blockingClient := &http.Client{
		Timeout:   BlockingClientReadTimeout,
		Transport: ac.NewRoundTripper(trBlocking),
	}

	setHTTPHeaders := func(_ *http.Request) error { return nil }
	if ac != nil {
		setHTTPHeaders = func(req *http.Request) error {
			return ac.SetHeaders(req, true)
		}
	}
	if httpCfg.FollowRedirects != nil && !*httpCfg.FollowRedirects {
		checkRedirect := func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		}
		client.CheckRedirect = checkRedirect
		blockingClient.CheckRedirect = checkRedirect
	}
	setHTTPProxyHeaders := func(_ *http.Request) error { return nil }
	if proxyAC != nil {
		setHTTPProxyHeaders = func(req *http.Request) error {
			return proxyURL.SetHeaders(proxyAC, req)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())

	c := &Client{
		client: &HTTPClient{
			client:      client,
			ReadTimeout: DefaultClientReadTimeout,
		},
		blockingClient: &HTTPClient{
			client:      blockingClient,
			ReadTimeout: BlockingClientReadTimeout,
		},
		apiServer:           apiServer,
		setHTTPHeaders:      setHTTPHeaders,
		setHTTPProxyHeaders: setHTTPProxyHeaders,
		clientCtx:           ctx,
		clientCancel:        cancel,
	}
	return c, nil
}

// Context returns context for the client requests.
func (c *Client) Context() context.Context {
	return c.clientCtx
}

// GetAPIResponseWithParamsCtx returns response for given absolute path with blocking client and optional callback for api response,
func (c *Client) GetAPIResponseWithParamsCtx(ctx context.Context, path string, modifyRequest RequestCallback, inspectResponse ResponseCallback) ([]byte, error) {
	return c.getAPIResponseWithConcurrencyLimit(ctx, c.client, path, modifyRequest, inspectResponse)
}

// GetAPIResponseWithReqParams returns response for given absolute path with optional callback for request.
func (c *Client) GetAPIResponseWithReqParams(path string, modifyRequest RequestCallback) ([]byte, error) {
	return c.getAPIResponseWithConcurrencyLimit(c.clientCtx, c.client, path, modifyRequest, nil)
}

// GetAPIResponse returns response for the given absolute path.
func (c *Client) GetAPIResponse(path string) ([]byte, error) {
	return c.getAPIResponseWithConcurrencyLimit(c.clientCtx, c.client, path, nil, nil)
}

func (c *Client) getAPIResponseWithConcurrencyLimit(ctx context.Context, client *HTTPClient, path string,
	modifyRequest RequestCallback, inspectResponse ResponseCallback,
) ([]byte, error) {
	// Limit the number of concurrent API requests.
	concurrencyLimitChOnce.Do(concurrencyLimitChInit)
	t := timerpool.Get(*maxWaitTime)
	select {
	case concurrencyLimitCh <- struct{}{}:
		timerpool.Put(t)
	case <-t.C:
		timerpool.Put(t)
		return nil, fmt.Errorf("too many outstanding requests to %q; try increasing -promscrape.discovery.concurrentWaitTime=%s or -promscrape.discovery.concurrency=%d",
			c.apiServer, *maxWaitTime, *maxConcurrency)
	case <-ctx.Done():
		timerpool.Put(t)
		return nil, ctx.Err()
	}
	data, err := c.getAPIResponseWithParamsAndClientCtx(ctx, client, path, modifyRequest, inspectResponse)
	<-concurrencyLimitCh
	return data, err
}

// GetBlockingAPIResponse returns response for given absolute path with blocking client and optional callback for api response,
func (c *Client) GetBlockingAPIResponse(path string, inspectResponse ResponseCallback) ([]byte, error) {
	return c.getAPIResponseWithParamsAndClientCtx(c.clientCtx, c.blockingClient, path, nil, inspectResponse)
}

// GetBlockingAPIResponseCtx returns response for given absolute path with blocking client and optional callback for api response,
func (c *Client) GetBlockingAPIResponseCtx(ctx context.Context, path string, inspectResponse ResponseCallback) ([]byte, error) {
	return c.getAPIResponseWithParamsAndClientCtx(ctx, c.blockingClient, path, nil, inspectResponse)
}

// getAPIResponseWithParamsAndClient returns response for the given absolute path with optional callback for request and for response.
func (c *Client) getAPIResponseWithParamsAndClientCtx(ctx context.Context, client *HTTPClient, path string, modifyRequest RequestCallback, inspectResponse ResponseCallback) ([]byte, error) {
	requestURL := c.apiServer + path
	u, err := url.Parse(requestURL)
	if err != nil {
		return nil, fmt.Errorf("cannot parse %q: %w", requestURL, err)
	}

	deadline := time.Now().Add(client.ReadTimeout)
	ctx, cancel := context.WithDeadline(ctx, deadline)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("cannot create request for %q: %w", requestURL, err)
	}

	if err := c.setHTTPHeaders(req); err != nil {
		return nil, fmt.Errorf("cannot set request headers for %q: %w", requestURL, err)
	}
	if err := c.setHTTPProxyHeaders(req); err != nil {
		return nil, fmt.Errorf("cannot set request proxy headers for %q: %w", requestURL, err)
	}
	if modifyRequest != nil {
		modifyRequest(req)
	}

	resp, err := doRequestWithPossibleRetry(client, req)
	if err != nil {
		return nil, fmt.Errorf("cannot fetch %q: %w", requestURL, err)
	}
	data, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		return nil, fmt.Errorf("cannot read response from %q: %w", requestURL, err)
	}

	if inspectResponse != nil {
		inspectResponse(resp)
	}
	statusCode := resp.StatusCode
	if statusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code returned from %q: %d; expecting %d; response body: %q",
			requestURL, statusCode, http.StatusOK, data)
	}
	return data, nil
}

// APIServer returns the API server address
func (c *Client) APIServer() string {
	return c.apiServer
}

// Stop cancels all in-flight requests
func (c *Client) Stop() {
	c.clientCancel()
	c.client.stop()
	c.blockingClient.stop()
}

func doRequestWithPossibleRetry(hc *HTTPClient, req *http.Request) (*http.Response, error) {
	discoveryRequests.Inc()

	var (
		reqErr error
		resp   *http.Response
	)
	// Return true if the request execution is completed and retry is not required
	attempt := func() bool {
		resp, reqErr = hc.client.Do(req)
		if reqErr != nil {
			if !errors.Is(reqErr, net.ErrClosed) && !strings.Contains(reqErr.Error(), "broken pipe") {
				return true
			}

			return false
		}

		return resp.StatusCode != http.StatusTooManyRequests
	}

	if attempt() {
		return resp, reqErr
	}

	// The first attempt was unsuccessful. Use exponential backoff for further attempts.
	// Perform the second attempt immediately after the first attempt - this should help
	// in cases when the remote side closes the keep-alive connection before the first attempt.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3293
	sleepTime := time.Second
	// It is expected that the deadline is already set to req.Context(), so the loop below
	// should eventually finish if all the attempt() calls are unsuccessful.
	ctx := req.Context()
	for {
		discoveryRetries.Inc()
		if attempt() {
			return resp, reqErr
		}
		sleepTime += sleepTime
		if !SleepCtx(ctx, sleepTime) {
			return resp, reqErr
		}
	}
}

var (
	discoveryRequests = metrics.NewCounter(`vm_promscrape_discovery_requests_total`)
	discoveryRetries  = metrics.NewCounter(`vm_promscrape_discovery_retries_total`)
)

// SleepCtx sleeps for sleepDuration.
//
// It immediately returns false on ctx cancel or deadline, without waiting for sleepDuration.
func SleepCtx(ctx context.Context, sleepDuration time.Duration) bool {
	t := timerpool.Get(sleepDuration)
	defer timerpool.Put(t)
	select {
	case <-ctx.Done():
		return false
	case <-t.C:
		return true
	}
}
