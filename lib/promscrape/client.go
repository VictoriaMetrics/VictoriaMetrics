package promscrape

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
	"github.com/VictoriaMetrics/metrics"
)

var (
	maxScrapeSize = flagutil.NewBytes("promscrape.maxScrapeSize", 16*1024*1024, "The maximum size of scrape response in bytes to process from Prometheus targets. "+
		"Bigger responses are rejected")
	maxResponseHeadersSize = flagutil.NewBytes("promscrape.maxResponseHeadersSize", 4096, "The maximum size of http response headers from Prometheus scrape targets")
	disableCompression     = flag.Bool("promscrape.disableCompression", false, "Whether to disable sending 'Accept-Encoding: gzip' request headers to all the scrape targets. "+
		"This may reduce CPU usage on scrape targets at the cost of higher network bandwidth utilization. "+
		"It is possible to set 'disable_compression: true' individually per each 'scrape_config' section in '-promscrape.config' for fine-grained control")
	disableKeepAlive = flag.Bool("promscrape.disableKeepAlive", false, "Whether to disable HTTP keep-alive connections when scraping all the targets. "+
		"This may be useful when targets has no support for HTTP keep-alive connection. "+
		"It is possible to set 'disable_keepalive: true' individually per each 'scrape_config' section in '-promscrape.config' for fine-grained control. "+
		"Note that disabling HTTP keep-alive may increase load on both vmagent and scrape targets")
	streamParse = flag.Bool("promscrape.streamParse", false, "Whether to enable stream parsing for metrics obtained from scrape targets. This may be useful "+
		"for reducing memory usage when millions of metrics are exposed per each scrape target. "+
		"It is possible to set 'stream_parse: true' individually per each 'scrape_config' section in '-promscrape.config' for fine-grained control")
)

type client struct {
	c                       *http.Client
	ctx                     context.Context
	scrapeURL               string
	scrapeTimeoutSecondsStr string
	setHeaders              func(req *http.Request) error
	setProxyHeaders         func(req *http.Request) error
}

func newClient(ctx context.Context, sw *ScrapeWork) (*client, error) {
	isTLS := strings.HasPrefix(sw.ScrapeURL, "https://")
	var tlsCfg *tls.Config
	if isTLS {
		var err error
		tlsCfg, err = sw.AuthConfig.NewTLSConfig()
		if err != nil {
			return nil, fmt.Errorf("cannot initialize tls config: %w", err)
		}
	}
	setHeaders := func(req *http.Request) error {
		return sw.AuthConfig.SetHeaders(req, true)
	}
	setProxyHeaders := func(req *http.Request) error {
		return nil
	}
	proxyURL := sw.ProxyURL
	if !isTLS && proxyURL.IsHTTPOrHTTPS() {
		pu := proxyURL.GetURL()
		if pu.Scheme == "https" {
			var err error
			tlsCfg, err = sw.ProxyAuthConfig.NewTLSConfig()
			if err != nil {
				return nil, fmt.Errorf("cannot initialize proxy tls config: %w", err)
			}
		}
		setProxyHeaders = func(req *http.Request) error {
			return proxyURL.SetHeaders(sw.ProxyAuthConfig, req)
		}
	}
	var proxyURLFunc func(*http.Request) (*url.URL, error)
	if pu := sw.ProxyURL.GetURL(); pu != nil {
		proxyURLFunc = http.ProxyURL(pu)
	}
	hc := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig:        tlsCfg,
			Proxy:                  proxyURLFunc,
			TLSHandshakeTimeout:    10 * time.Second,
			IdleConnTimeout:        2 * sw.ScrapeInterval,
			DisableCompression:     *disableCompression || sw.DisableCompression,
			DisableKeepAlives:      *disableKeepAlive || sw.DisableKeepAlive,
			DialContext:            statStdDial,
			MaxIdleConnsPerHost:    100,
			MaxResponseHeaderBytes: int64(maxResponseHeadersSize.N),
		},
		Timeout: sw.ScrapeTimeout,
	}
	if sw.DenyRedirects {
		hc.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		}
	}

	c := &client{
		c:                       hc,
		ctx:                     ctx,
		scrapeURL:               sw.ScrapeURL,
		scrapeTimeoutSecondsStr: fmt.Sprintf("%.3f", sw.ScrapeTimeout.Seconds()),
		setHeaders:              setHeaders,
		setProxyHeaders:         setProxyHeaders,
	}
	return c, nil
}

func (c *client) ReadData(dst *bytesutil.ByteBuffer) error {
	deadline := time.Now().Add(c.c.Timeout)
	ctx, cancel := context.WithDeadline(c.ctx, deadline)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.scrapeURL, nil)
	if err != nil {
		cancel()
		return fmt.Errorf("cannot create request for %q: %w", c.scrapeURL, err)
	}
	// The following `Accept` header has been copied from Prometheus sources.
	// See https://github.com/prometheus/prometheus/blob/f9d21f10ecd2a343a381044f131ea4e46381ce09/scrape/scrape.go#L532 .
	// This is needed as a workaround for scraping stupid Java-based servers such as Spring Boot.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/608 for details.
	// Do not bloat the `Accept` header with OpenMetrics shit, since it looks like dead standard now.
	req.Header.Set("Accept", "text/plain;version=0.0.4;q=1,*/*;q=0.1")
	// Set X-Prometheus-Scrape-Timeout-Seconds like Prometheus does, since it is used by some exporters such as PushProx.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1179#issuecomment-813117162
	req.Header.Set("X-Prometheus-Scrape-Timeout-Seconds", c.scrapeTimeoutSecondsStr)
	req.Header.Set("User-Agent", "vm_promscrape")
	if err := c.setHeaders(req); err != nil {
		cancel()
		return fmt.Errorf("failed to set request headers for %q: %w", c.scrapeURL, err)
	}
	if err := c.setProxyHeaders(req); err != nil {
		cancel()
		return fmt.Errorf("failed to set proxy request headers for %q: %w", c.scrapeURL, err)
	}
	scrapeRequests.Inc()
	resp, err := c.c.Do(req)
	if err != nil {
		cancel()
		if ue, ok := err.(*url.Error); ok && ue.Timeout() {
			scrapesTimedout.Inc()
		}
		return fmt.Errorf("cannot perform request to %q: %w", c.scrapeURL, err)
	}
	if resp.StatusCode != http.StatusOK {
		metrics.GetOrCreateCounter(fmt.Sprintf(`vm_promscrape_scrapes_total{status_code="%d"}`, resp.StatusCode)).Inc()
		respBody, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		cancel()
		return fmt.Errorf("unexpected status code returned when scraping %q: %d; expecting %d; response body: %q",
			c.scrapeURL, resp.StatusCode, http.StatusOK, respBody)
	}
	scrapesOK.Inc()

	// Read the data from resp.Body
	r := &io.LimitedReader{
		R: resp.Body,
		N: maxScrapeSize.N,
	}
	_, err = dst.ReadFrom(r)
	_ = resp.Body.Close()
	cancel()
	if err != nil {
		if ue, ok := err.(*url.Error); ok && ue.Timeout() {
			scrapesTimedout.Inc()
		}
		return fmt.Errorf("cannot read data from %s: %w", c.scrapeURL, err)
	}
	if int64(len(dst.B)) >= maxScrapeSize.N {
		maxScrapeSizeExceeded.Inc()
		return fmt.Errorf("the response from %q exceeds -promscrape.maxScrapeSize=%d; "+
			"either reduce the response size for the target or increase -promscrape.maxScrapeSize command-line flag value", c.scrapeURL, maxScrapeSize.N)
	}
	return nil
}

var (
	maxScrapeSizeExceeded = metrics.NewCounter(`vm_promscrape_max_scrape_size_exceeded_errors_total`)
	scrapesTimedout       = metrics.NewCounter(`vm_promscrape_scrapes_timed_out_total`)
	scrapesOK             = metrics.NewCounter(`vm_promscrape_scrapes_total{status_code="200"}`)
	scrapeRequests        = metrics.NewCounter(`vm_promscrape_scrape_requests_total`)
)
