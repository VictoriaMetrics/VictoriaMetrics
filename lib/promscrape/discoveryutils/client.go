package discoveryutils

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/netutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
	"github.com/VictoriaMetrics/fasthttp"
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
	hc        *fasthttp.HostClient
	ac        *promauth.Config
	apiServer string
	hostPort  string
}

// NewClient returns new Client for the given apiServer and the given ac.
func NewClient(apiServer string, ac *promauth.Config) (*Client, error) {
	var u fasthttp.URI
	u.Update(apiServer)
	hostPort := string(u.Host())
	isTLS := string(u.Scheme()) == "https"
	var tlsCfg *tls.Config
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
	}
	return &Client{
		hc:        hc,
		ac:        ac,
		apiServer: apiServer,
		hostPort:  hostPort,
	}, nil
}

// GetAPIResponse returns response for the given absolute path.
func (c *Client) GetAPIResponse(path string) ([]byte, error) {
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
	// There is no need in calling DoTimeout, since the timeout is already set in c.hc.ReadTimeout above.
	if err := c.hc.Do(&req, &resp); err != nil {
		return nil, fmt.Errorf("cannot fetch %q: %s", requestURL, err)
	}
	var data []byte
	if ce := resp.Header.Peek("Content-Encoding"); string(ce) == "gzip" {
		dst, err := fasthttp.AppendGunzipBytes(nil, resp.Body())
		if err != nil {
			return nil, fmt.Errorf("cannot ungzip response from %q: %s", requestURL, err)
		}
		data = dst
	} else {
		data = append(data[:0], resp.Body()...)
	}
	statusCode := resp.StatusCode()
	if statusCode != fasthttp.StatusOK {
		return nil, fmt.Errorf("unexpected status code returned from %q: %d; expecting %d; response body: %q",
			requestURL, statusCode, fasthttp.StatusOK, data)
	}
	return data, nil
}
