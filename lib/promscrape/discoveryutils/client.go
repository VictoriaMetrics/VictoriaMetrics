package discoveryutils

import (
	"crypto/tls"
	"flag"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/netutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/timerpool"
	"github.com/VictoriaMetrics/fasthttp"
)

var (
	maxConcurrency = flag.Int("promscrape.discovery.concurrency", 100, "The maximum number of concurrent requests to Prometheus autodiscovery API (Consul, Kubernetes, etc.)")
	maxWaitTime    = flag.Duration("promscrape.discovery.concurrentWaitTime", time.Minute, "The maximum duration for waiting to perform API requests "+
		"if more than -promscrape.discovery.concurrency requests are simultaneously performed")
)

var defaultClient = &http.Client{
	Timeout: 30 * time.Second,
}

// GetHTTPClient returns default client for http API requests.
func GetHTTPClient() *http.Client {
	return defaultClient
}

// Client is http client, which talks to the given apiServer.
type Client struct {
	hc *fasthttp.HostClient
	// blockingClient is used for performing long-polling requests.
	blockingClient *fasthttp.HostClient
	ac             *promauth.Config
	apiServer      string
	hostPort       string
}

// NewClient returns new Client for the given apiServer and the given ac.
func NewClient(apiServer string, ac *promauth.Config) (*Client, error) {
	var (
		dialFunc fasthttp.DialFunc
		tlsCfg   *tls.Config
		u        fasthttp.URI
	)
	u.Update(apiServer)

	// special case for unix socket connection
	if string(u.Scheme()) == "unix" {
		dialAddr := string(u.Path())
		apiServer = "http://"
		dialFunc = func(_ string) (net.Conn, error) {
			return net.Dial("unix", dialAddr)
		}
	}
	hostPort := string(u.Host())
	isTLS := string(u.Scheme()) == "https"
	if isTLS && ac != nil {
		tlsCfg = ac.NewTLSConfig()
	}
	if !strings.Contains(hostPort, ":") {
		port := "80"
		if isTLS {
			port = "443"
		}
		hostPort = net.JoinHostPort(hostPort, port)
	}
	hc := &fasthttp.HostClient{
		Addr:                hostPort,
		Name:                "vm_promscrape/discovery",
		DialDualStack:       netutil.TCP6Enabled(),
		IsTLS:               isTLS,
		TLSConfig:           tlsCfg,
		ReadTimeout:         time.Minute,
		WriteTimeout:        10 * time.Second,
		MaxResponseBodySize: 300 * 1024 * 1024,
		MaxConns:            2 * *maxConcurrency,
		Dial:                dialFunc,
	}
	wc := &fasthttp.HostClient{
		Addr:                hostPort,
		Name:                "vm_promscrape/discovery",
		DialDualStack:       netutil.TCP6Enabled(),
		IsTLS:               isTLS,
		TLSConfig:           tlsCfg,
		ReadTimeout:         time.Minute * 3,
		WriteTimeout:        10 * time.Second,
		MaxResponseBodySize: 300 * 1024 * 1024,
		MaxConns:            20 * *maxConcurrency,
		Dial:                dialFunc,
	}
	return &Client{
		hc:             hc,
		blockingClient: wc,
		ac:             ac,
		apiServer:      apiServer,
		hostPort:       hostPort,
	}, nil
}

var (
	concurrencyLimitCh     chan struct{}
	concurrencyLimitChOnce sync.Once
)

func concurrencyLimitChInit() {
	concurrencyLimitCh = make(chan struct{}, *maxConcurrency)
}

// APIRequestParams modifies api request with given params.
type APIRequestParams struct {
	FetchFromResponse func(resp *fasthttp.Response)
	SetToRequest      func(req *fasthttp.Request)
}

// GetAPIResponse returns response for the given absolute path.
func (c *Client) GetAPIResponse(path string) ([]byte, error) {
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
	return c.getAPIResponseWithParamsAndClient(c.hc, path, nil)
}

// GetBlockingAPIResponse returns response for given absolute path with blocking client and optional callback for api response,
// inspectResponse - should never reference data from response.
func (c *Client) GetBlockingAPIResponse(path string, inspectResponse func(resp *fasthttp.Response)) ([]byte, error) {
	return c.getAPIResponseWithParamsAndClient(c.blockingClient, path, inspectResponse)
}

// getAPIResponseWithParamsAndClient returns response for the given absolute path with optional callback for response.
func (c *Client) getAPIResponseWithParamsAndClient(client *fasthttp.HostClient, path string, inspectResponse func(resp *fasthttp.Response)) ([]byte, error) {
	requestURL := c.apiServer + path
	var u fasthttp.URI
	u.Update(requestURL)
	var req fasthttp.Request
	req.SetRequestURIBytes(u.RequestURI())
	req.SetHost(c.hostPort)
	req.Header.Set("Accept-Encoding", "gzip")
	if c.ac != nil && c.ac.Authorization != "" {
		req.Header.Set("Authorization", c.ac.Authorization)
	}

	var resp fasthttp.Response
	deadline := time.Now().Add(client.ReadTimeout)
	if err := doRequestWithPossibleRetry(client, &req, &resp, deadline); err != nil {
		return nil, fmt.Errorf("cannot fetch %q: %w", requestURL, err)
	}
	var data []byte
	if ce := resp.Header.Peek("Content-Encoding"); string(ce) == "gzip" {
		dst, err := fasthttp.AppendGunzipBytes(nil, resp.Body())
		if err != nil {
			return nil, fmt.Errorf("cannot ungzip response from %q: %w", requestURL, err)
		}
		data = dst
	} else {
		data = append(data[:0], resp.Body()...)
	}
	if inspectResponse != nil {
		inspectResponse(&resp)
	}
	statusCode := resp.StatusCode()
	if statusCode != fasthttp.StatusOK {
		return nil, fmt.Errorf("unexpected status code returned from %q: %d; expecting %d; response body: %q",
			requestURL, statusCode, fasthttp.StatusOK, data)
	}
	return data, nil
}

func doRequestWithPossibleRetry(hc *fasthttp.HostClient, req *fasthttp.Request, resp *fasthttp.Response, deadline time.Time) error {
	for {
		// Use DoDeadline instead of Do even if hc.ReadTimeout is already set in order to guarantee the given deadline
		// across multiple retries.
		err := hc.DoDeadline(req, resp, deadline)
		if err == nil {
			return nil
		}
		if err != fasthttp.ErrConnectionClosed {
			return err
		}
		// Retry request if the server closes the keep-alive connection unless deadline exceeds.
		if time.Since(deadline) >= 0 {
			return fmt.Errorf("the server closes all the connection attempts: %w", err)
		}
	}
}
