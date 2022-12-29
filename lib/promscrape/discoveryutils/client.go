package discoveryutils

import (
	"compress/gzip"
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/proxy"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/timerpool"
	"github.com/VictoriaMetrics/metrics"
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

	// DefaultClientWriteTimeout is the maximum duration for waiting the request to be sent to GetAPI* and GetBlockingAPI*
	DefaultClientWriteTimeout = 10 * time.Second
)

func concurrencyLimitChInit() {
	concurrencyLimitCh = make(chan struct{}, *maxConcurrency)
}

// GetHTTPClient returns default client for http API requests.
func GetHTTPClient() *http.Client {
	return defaultClient
}

// Client is http client, which talks to the given apiServer.
type Client struct {
	// client is used for short requests.
	client *HTTPClient

	// blockingClient is used for long-polling requests.
	blockingClient *HTTPClient

	apiServer string

	dialAddr string

	setHTTPHeaders      func(req *http.Request)
	setHTTPProxyHeaders func(req *http.Request)

	clientCtx    context.Context
	clientCancel context.CancelFunc
}

// HTTPClient is a wrapper around http.Client with timeouts.
type HTTPClient struct {
	client       *http.Client
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

func addMissingPort(addr string, isTLS bool) string {
	if strings.Contains(addr, ":") {
		return addr
	}
	if isTLS {
		return addr + ":443"
	}
	return addr + ":80"
}

// NewClient returns new Client for the given args.
func NewClient(apiServer string, ac *promauth.Config, proxyURL *proxy.URL, proxyAC *promauth.Config) (*Client, error) {
	u, err := url.Parse(apiServer)
	if err != nil {
		return nil, fmt.Errorf("cannot parse provided url %q: %w", apiServer, err)
	}

	// special case for unix socket connection
	var dialFunc func(addr string) (net.Conn, error)
	if string(u.Scheme) == "unix" {
		dialAddr := u.Path
		apiServer = "http://"
		dialFunc = func(_ string) (net.Conn, error) {
			return net.Dial("unix", dialAddr)
		}
	}

	dialAddr := u.Host
	isTLS := string(u.Scheme) == "https"
	var tlsCfg *tls.Config
	if isTLS {
		tlsCfg = ac.NewTLSConfig()
	}

	setHTTPProxyHeaders := func(req *http.Request) {}

	dialAddr = addMissingPort(dialAddr, isTLS)
	if dialFunc == nil {
		var err error
		dialFunc, err = proxyURL.NewDialFunc(proxyAC)
		if err != nil {
			return nil, err
		}
		if proxyAC != nil {
			setHTTPProxyHeaders = func(req *http.Request) {
				proxyURL.SetHeaders(proxyAC, req)
			}
		}
	}

	hcTransport := &http.Transport{
		TLSClientConfig:       tlsCfg,
		MaxConnsPerHost:       2 * *maxConcurrency,
		ResponseHeaderTimeout: *maxWaitTime,
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return dialFunc(dialAddr)
		},
	}

	hc := &http.Client{
		Timeout:   DefaultClientReadTimeout,
		Transport: hcTransport,
	}

	blockingTransport := &http.Transport{
		TLSClientConfig: tlsCfg,
		MaxConnsPerHost: 64 * 1024,
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return dialFunc(dialAddr)
		},
	}
	blockingClient := &http.Client{
		Timeout:   BlockingClientReadTimeout,
		Transport: blockingTransport,
	}

	setHTTPHeaders := func(req *http.Request) {}
	if ac != nil {
		setHTTPHeaders = func(req *http.Request) { ac.SetHeaders(req, true) }
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Client{
		client:              &HTTPClient{client: hc, ReadTimeout: DefaultClientReadTimeout, WriteTimeout: DefaultClientWriteTimeout},
		blockingClient:      &HTTPClient{client: blockingClient, ReadTimeout: BlockingClientReadTimeout, WriteTimeout: DefaultClientWriteTimeout},
		apiServer:           apiServer,
		dialAddr:            dialAddr,
		setHTTPHeaders:      setHTTPHeaders,
		setHTTPProxyHeaders: setHTTPProxyHeaders,
		clientCtx:           ctx,
		clientCancel:        cancel,
	}, nil
}

// Addr returns the address the client connects to.
func (c *Client) Addr() string {
	return c.dialAddr
}

// GetAPIResponseWithReqParams returns response for given absolute path with optional callback for request.
// modifyRequestParams should never reference data from request.
func (c *Client) GetAPIResponseWithReqParams(path string, modifyRequestParams func(request *http.Request)) ([]byte, error) {
	return c.getAPIResponse(path, modifyRequestParams)
}

// GetAPIResponse returns response for the given absolute path.
func (c *Client) GetAPIResponse(path string) ([]byte, error) {
	return c.getAPIResponse(path, nil)
}

// GetAPIResponse returns response for the given absolute path with optional callback for request.
func (c *Client) getAPIResponse(path string, modifyRequest func(request *http.Request)) ([]byte, error) {
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
	}
	defer func() { <-concurrencyLimitCh }()
	return c.getAPIResponseWithParamsAndClient(c.client, path, modifyRequest, nil)
}

// GetBlockingAPIResponse returns response for given absolute path with blocking client and optional callback for api response,
// inspectResponse - should never reference data from response.
func (c *Client) GetBlockingAPIResponse(path string, inspectResponse func(resp *http.Response)) ([]byte, error) {
	return c.getAPIResponseWithParamsAndClient(c.blockingClient, path, nil, inspectResponse)
}

// getAPIResponseWithParamsAndClient returns response for the given absolute path with optional callback for request and for response.
func (c *Client) getAPIResponseWithParamsAndClient(client *HTTPClient, path string, modifyRequest func(req *http.Request), inspectResponse func(resp *http.Response)) ([]byte, error) {
	requestURL := c.apiServer + path
	u, err := url.Parse(requestURL)
	if err != nil {
		return nil, fmt.Errorf("cannot parse %q: %w", requestURL, err)
	}
	u.Host = c.dialAddr

	deadline := time.Now().Add(client.WriteTimeout)
	ctx, cancel := context.WithDeadline(c.clientCtx, deadline)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("cannot create request for %q: %w", requestURL, err)
	}

	req.Header.Set("Host", c.dialAddr)
	req.Header.Set("Accept-Encoding", "gzip")
	c.setHTTPHeaders(req)
	c.setHTTPProxyHeaders(req)
	if modifyRequest != nil {
		modifyRequest(req)
	}

	resp, err := doRequestWithPossibleRetry(client, req)
	if err != nil {
		return nil, fmt.Errorf("cannot fetch %q: %w", requestURL, err)
	}

	reader := resp.Body
	if resp.Header.Get("Content-Encoding") == "gzip" {
		reader, err = gzip.NewReader(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("cannot create gzip reader for %q: %w", requestURL, err)
		}
	}

	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("cannot ungzip response from %q: %w", requestURL, err)
	}
	_ = resp.Body.Close()

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
}

// DoRequestWithPossibleRetry performs the given req at client and stores the response at resp.
func DoRequestWithPossibleRetry(hc *HTTPClient, req *http.Request, requestCounter, retryCounter *metrics.Counter) (*http.Response, error) {
	sleepTime := time.Second
	requestCounter.Inc()
	deadline, ok := req.Context().Deadline()
	if !ok {
		deadline = time.Now().Add(hc.WriteTimeout)
	}

	for {
		resp, err := hc.client.Do(req)
		if err == nil {
			statusCode := resp.StatusCode
			if statusCode != http.StatusTooManyRequests {
				return resp, nil
			}
		} else if err != net.ErrClosed && !strings.Contains(err.Error(), "broken pipe") {
			return nil, err
		}
		// Retry request after exponentially increased sleep.
		maxSleepTime := time.Until(deadline)
		if sleepTime > maxSleepTime {
			return nil, fmt.Errorf("the server closes all the connection attempts: %w", err)
		}
		sleepTime += sleepTime
		if sleepTime > maxSleepTime {
			sleepTime = maxSleepTime
		}
		time.Sleep(sleepTime)
		retryCounter.Inc()
	}
}

func doRequestWithPossibleRetry(hc *HTTPClient, req *http.Request) (*http.Response, error) {
	return DoRequestWithPossibleRetry(hc, req, discoveryRequests, discoveryRetries)
}

var (
	discoveryRequests = metrics.NewCounter(`vm_promscrape_discovery_requests_total`)
	discoveryRetries  = metrics.NewCounter(`vm_promscrape_discovery_retries_total`)
)
