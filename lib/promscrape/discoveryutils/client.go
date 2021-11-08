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

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/proxy"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/timerpool"
	"github.com/VictoriaMetrics/fasthttp"
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

// GetHTTPClient returns default client for http API requests.
func GetHTTPClient() *http.Client {
	return defaultClient
}

// Client is http client, which talks to the given apiServer.
type Client struct {
	// hc is used for short requests.
	hc *fasthttp.HostClient

	// blockingClient is used for long-polling requests.
	blockingClient *fasthttp.HostClient

	apiServer string

	hostPort           string
	getAuthHeader      func() string
	getProxyAuthHeader func() string
	sendFullURL        bool
}

// NewClient returns new Client for the given args.
func NewClient(apiServer string, ac *promauth.Config, proxyURL *proxy.URL, proxyAC *promauth.Config) (*Client, error) {
	var u fasthttp.URI
	u.Update(apiServer)

	// special case for unix socket connection
	var dialFunc fasthttp.DialFunc
	if string(u.Scheme()) == "unix" {
		dialAddr := string(u.Path())
		apiServer = "http://"
		dialFunc = func(_ string) (net.Conn, error) {
			return net.Dial("unix", dialAddr)
		}
	}

	hostPort := string(u.Host())
	isTLS := string(u.Scheme()) == "https"
	var tlsCfg *tls.Config
	if isTLS {
		tlsCfg = ac.NewTLSConfig()
	}
	sendFullURL := !isTLS && proxyURL.IsHTTPOrHTTPS()
	getProxyAuthHeader := func() string { return "" }
	if sendFullURL {
		// Send full urls in requests to a proxy host for non-TLS apiServer
		// like net/http package from Go does.
		// See https://en.wikipedia.org/wiki/Proxy_server#Web_proxy_servers
		pu := proxyURL.URL()
		hostPort = pu.Host
		isTLS = pu.Scheme == "https"
		if isTLS {
			tlsCfg = proxyAC.NewTLSConfig()
		}
		proxyURLOrig := proxyURL
		getProxyAuthHeader = func() string {
			return proxyURLOrig.GetAuthHeader(proxyAC)
		}
		proxyURL = &proxy.URL{}
	}
	if !strings.Contains(hostPort, ":") {
		port := "80"
		if isTLS {
			port = "443"
		}
		hostPort = net.JoinHostPort(hostPort, port)
	}
	if dialFunc == nil {
		var err error
		dialFunc, err = proxyURL.NewDialFunc(proxyAC)
		if err != nil {
			return nil, err
		}
	}
	hc := &fasthttp.HostClient{
		Addr:                hostPort,
		Name:                "vm_promscrape/discovery",
		IsTLS:               isTLS,
		TLSConfig:           tlsCfg,
		ReadTimeout:         time.Minute,
		WriteTimeout:        10 * time.Second,
		MaxResponseBodySize: 300 * 1024 * 1024,
		MaxConns:            2 * *maxConcurrency,
		Dial:                dialFunc,
	}
	blockingClient := &fasthttp.HostClient{
		Addr:                hostPort,
		Name:                "vm_promscrape/discovery",
		IsTLS:               isTLS,
		TLSConfig:           tlsCfg,
		ReadTimeout:         BlockingClientReadTimeout,
		WriteTimeout:        10 * time.Second,
		MaxResponseBodySize: 300 * 1024 * 1024,
		MaxConns:            64 * 1024,
		Dial:                dialFunc,
	}
	getAuthHeader := func() string { return "" }
	if ac != nil {
		getAuthHeader = ac.GetAuthHeader
	}
	return &Client{
		hc:                 hc,
		blockingClient:     blockingClient,
		apiServer:          apiServer,
		hostPort:           hostPort,
		getAuthHeader:      getAuthHeader,
		getProxyAuthHeader: getProxyAuthHeader,
		sendFullURL:        sendFullURL,
	}, nil
}

// BlockingClientReadTimeout is the maximum duration for waiting the response from GetBlockingAPI*
const BlockingClientReadTimeout = 10 * time.Minute

var (
	concurrencyLimitCh     chan struct{}
	concurrencyLimitChOnce sync.Once
)

func concurrencyLimitChInit() {
	concurrencyLimitCh = make(chan struct{}, *maxConcurrency)
}

// Addr returns the address the client connects to.
func (c *Client) Addr() string {
	return c.hc.Addr
}

// GetAPIResponseWithReqParams returns response for given absolute path with optional callback for request.
// modifyRequestParams should never reference data from request.
func (c *Client) GetAPIResponseWithReqParams(path string, modifyRequestParams func(request *fasthttp.Request)) ([]byte, error) {
	return c.getAPIResponse(path, modifyRequestParams)
}

// GetAPIResponse returns response for the given absolute path.
func (c *Client) GetAPIResponse(path string) ([]byte, error) {
	return c.getAPIResponse(path, nil)
}

// GetAPIResponse returns response for the given absolute path with optional callback for request.
func (c *Client) getAPIResponse(path string, modifyRequest func(request *fasthttp.Request)) ([]byte, error) {
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
	return c.getAPIResponseWithParamsAndClient(c.hc, path, modifyRequest, nil)
}

// GetBlockingAPIResponse returns response for given absolute path with blocking client and optional callback for api response,
// inspectResponse - should never reference data from response.
func (c *Client) GetBlockingAPIResponse(path string, inspectResponse func(resp *fasthttp.Response)) ([]byte, error) {
	return c.getAPIResponseWithParamsAndClient(c.blockingClient, path, nil, inspectResponse)
}

// getAPIResponseWithParamsAndClient returns response for the given absolute path with optional callback for request and for response.
func (c *Client) getAPIResponseWithParamsAndClient(client *fasthttp.HostClient, path string, modifyRequest func(req *fasthttp.Request), inspectResponse func(resp *fasthttp.Response)) ([]byte, error) {
	requestURL := c.apiServer + path
	var u fasthttp.URI
	u.Update(requestURL)
	var req fasthttp.Request
	if c.sendFullURL {
		req.SetRequestURIBytes(u.FullURI())
	} else {
		req.SetRequestURIBytes(u.RequestURI())
	}
	req.Header.SetHost(c.hostPort)
	req.Header.Set("Accept-Encoding", "gzip")
	if ah := c.getAuthHeader(); ah != "" {
		req.Header.Set("Authorization", ah)
	}
	if ah := c.getProxyAuthHeader(); ah != "" {
		req.Header.Set("Proxy-Authorization", ah)
	}
	if modifyRequest != nil {
		modifyRequest(&req)
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
	sleepTime := time.Second
	discoveryRequests.Inc()
	for {
		// Use DoDeadline instead of Do even if hc.ReadTimeout is already set in order to guarantee the given deadline
		// across multiple retries.
		err := hc.DoDeadline(req, resp, deadline)
		if err == nil {
			return nil
		}
		if err != fasthttp.ErrConnectionClosed && !strings.Contains(err.Error(), "broken pipe") {
			return err
		}
		// Retry request if the server closes the keep-alive connection unless deadline exceeds.
		maxSleepTime := time.Until(deadline)
		if sleepTime > maxSleepTime {
			return fmt.Errorf("the server closes all the connection attempts: %w", err)
		}
		sleepTime += sleepTime
		if sleepTime > maxSleepTime {
			sleepTime = maxSleepTime
		}
		time.Sleep(sleepTime)
		discoveryRetries.Inc()
	}
}

var (
	discoveryRetries  = metrics.NewCounter(`vm_promscrape_discovery_retries_total`)
	discoveryRequests = metrics.NewCounter(`vm_promscrape_discovery_requests_total`)
)
