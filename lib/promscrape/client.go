package promscrape

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/fasthttp"
	"github.com/VictoriaMetrics/metrics"
)

var (
	maxScrapeSize = flagutil.NewBytes("promscrape.maxScrapeSize", 16*1024*1024, "The maximum size of scrape response in bytes to process from Prometheus targets. "+
		"Bigger responses are rejected")
	disableCompression = flag.Bool("promscrape.disableCompression", false, "Whether to disable sending 'Accept-Encoding: gzip' request headers to all the scrape targets. "+
		"This may reduce CPU usage on scrape targets at the cost of higher network bandwidth utilization. "+
		"It is possible to set 'disable_compression: true' individually per each 'scrape_config' section in '-promscrape.config' for fine grained control")
	disableKeepAlive = flag.Bool("promscrape.disableKeepAlive", false, "Whether to disable HTTP keep-alive connections when scraping all the targets. "+
		"This may be useful when targets has no support for HTTP keep-alive connection. "+
		"It is possible to set `disable_keepalive: true` individually per each 'scrape_config` section in '-promscrape.config' for fine grained control. "+
		"Note that disabling HTTP keep-alive may increase load on both vmagent and scrape targets")
	streamParse = flag.Bool("promscrape.streamParse", false, "Whether to enable stream parsing for metrics obtained from scrape targets. This may be useful "+
		"for reducing memory usage when millions of metrics are exposed per each scrape target. "+
		"It is posible to set `stream_parse: true` individually per each `scrape_config` section in `-promscrape.config` for fine grained control")
)

type client struct {
	// hc is the default client optimized for common case of scraping targets with moderate number of metrics.
	hc *fasthttp.HostClient

	// sc (aka `stream client`) is used instead of hc if ScrapeWork.ParseStream is set.
	// It may be useful for scraping targets with millions of metrics per target.
	sc *http.Client

	scrapeURL          string
	host               string
	requestURI         string
	authHeader         string
	disableCompression bool
	disableKeepAlive   bool
}

func newClient(sw *ScrapeWork) *client {
	var u fasthttp.URI
	u.Update(sw.ScrapeURL)
	host := string(u.Host())
	requestURI := string(u.RequestURI())
	isTLS := string(u.Scheme()) == "https"
	var tlsCfg *tls.Config
	if isTLS {
		tlsCfg = sw.AuthConfig.NewTLSConfig()
	}
	if !strings.Contains(host, ":") {
		if !isTLS {
			host += ":80"
		} else {
			host += ":443"
		}
	}
	hc := &fasthttp.HostClient{
		Addr:                         host,
		Name:                         "vm_promscrape",
		Dial:                         statDial,
		IsTLS:                        isTLS,
		TLSConfig:                    tlsCfg,
		MaxIdleConnDuration:          2 * sw.ScrapeInterval,
		ReadTimeout:                  sw.ScrapeTimeout,
		WriteTimeout:                 10 * time.Second,
		MaxResponseBodySize:          maxScrapeSize.N,
		MaxIdempotentRequestAttempts: 1,
	}
	var sc *http.Client
	if *streamParse || sw.StreamParse {
		sc = &http.Client{
			Transport: &http.Transport{
				TLSClientConfig:     tlsCfg,
				TLSHandshakeTimeout: 10 * time.Second,
				IdleConnTimeout:     2 * sw.ScrapeInterval,
				DisableCompression:  *disableCompression || sw.DisableCompression,
				DisableKeepAlives:   *disableKeepAlive || sw.DisableKeepAlive,
				DialContext:         statStdDial,
			},
			Timeout: sw.ScrapeTimeout,
		}
	}
	return &client{
		hc: hc,
		sc: sc,

		scrapeURL:          sw.ScrapeURL,
		host:               host,
		requestURI:         requestURI,
		authHeader:         sw.AuthConfig.Authorization,
		disableCompression: sw.DisableCompression,
		disableKeepAlive:   sw.DisableKeepAlive,
	}
}

func (c *client) GetStreamReader() (*streamReader, error) {
	deadline := time.Now().Add(c.hc.ReadTimeout)
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	req, err := http.NewRequestWithContext(ctx, "GET", c.scrapeURL, nil)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("cannot create request for %q: %w", c.scrapeURL, err)
	}
	// The following `Accept` header has been copied from Prometheus sources.
	// See https://github.com/prometheus/prometheus/blob/f9d21f10ecd2a343a381044f131ea4e46381ce09/scrape/scrape.go#L532 .
	// This is needed as a workaround for scraping stupid Java-based servers such as Spring Boot.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/608 for details.
	// Do not bloat the `Accept` header with OpenMetrics shit, since it looks like dead standard now.
	req.Header.Set("Accept", "text/plain;version=0.0.4;q=1,*/*;q=0.1")
	if c.authHeader != "" {
		req.Header.Set("Authorization", c.authHeader)
	}
	resp, err := c.sc.Do(req)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("cannot scrape %q: %w", c.scrapeURL, err)
	}
	if resp.StatusCode != http.StatusOK {
		metrics.GetOrCreateCounter(fmt.Sprintf(`vm_promscrape_scrapes_total{status_code="%d"}`, resp.StatusCode)).Inc()
		respBody, _ := ioutil.ReadAll(resp.Body)
		_ = resp.Body.Close()
		cancel()
		return nil, fmt.Errorf("unexpected status code returned when scraping %q: %d; expecting %d; response body: %q",
			c.scrapeURL, resp.StatusCode, http.StatusOK, respBody)
	}
	scrapesOK.Inc()
	return &streamReader{
		r:      resp.Body,
		cancel: cancel,
	}, nil
}

func (c *client) ReadData(dst []byte) ([]byte, error) {
	deadline := time.Now().Add(c.hc.ReadTimeout)
	req := fasthttp.AcquireRequest()
	req.SetRequestURI(c.requestURI)
	req.SetHost(c.host)
	// The following `Accept` header has been copied from Prometheus sources.
	// See https://github.com/prometheus/prometheus/blob/f9d21f10ecd2a343a381044f131ea4e46381ce09/scrape/scrape.go#L532 .
	// This is needed as a workaround for scraping stupid Java-based servers such as Spring Boot.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/608 for details.
	// Do not bloat the `Accept` header with OpenMetrics shit, since it looks like dead standard now.
	req.Header.Set("Accept", "text/plain;version=0.0.4;q=1,*/*;q=0.1")
	if !*disableCompression && !c.disableCompression {
		req.Header.Set("Accept-Encoding", "gzip")
	}
	if *disableKeepAlive || c.disableKeepAlive {
		req.SetConnectionClose()
	}
	if c.authHeader != "" {
		req.Header.Set("Authorization", c.authHeader)
	}
	resp := fasthttp.AcquireResponse()
	swapResponseBodies := len(dst) == 0
	if swapResponseBodies {
		// An optimization: write response directly to dst.
		// This should reduce memory uage when scraping big targets.
		dst = resp.SwapBody(dst)
	}
	err := doRequestWithPossibleRetry(c.hc, req, resp, deadline)
	statusCode := resp.StatusCode()
	if err == nil && (statusCode == fasthttp.StatusMovedPermanently || statusCode == fasthttp.StatusFound) {
		// Allow a single redirect.
		// It is expected that the redirect is made on the same host.
		// Otherwise it won't work.
		if location := resp.Header.Peek("Location"); len(location) > 0 {
			req.URI().UpdateBytes(location)
			err = c.hc.DoDeadline(req, resp, deadline)
			statusCode = resp.StatusCode()
		}
	}
	if swapResponseBodies {
		dst = resp.SwapBody(dst)
	}
	fasthttp.ReleaseRequest(req)
	if err != nil {
		fasthttp.ReleaseResponse(resp)
		if err == fasthttp.ErrTimeout {
			scrapesTimedout.Inc()
			return dst, fmt.Errorf("error when scraping %q with timeout %s: %w", c.scrapeURL, c.hc.ReadTimeout, err)
		}
		if err == fasthttp.ErrBodyTooLarge {
			return dst, fmt.Errorf("the response from %q exceeds -promscrape.maxScrapeSize=%d; "+
				"either reduce the response size for the target or increase -promscrape.maxScrapeSize", c.scrapeURL, maxScrapeSize.N)
		}
		return dst, fmt.Errorf("error when scraping %q: %w", c.scrapeURL, err)
	}
	if ce := resp.Header.Peek("Content-Encoding"); string(ce) == "gzip" {
		var err error
		var src []byte
		if swapResponseBodies {
			src = append(src, dst...)
			dst = dst[:0]
		} else {
			src = resp.Body()
		}
		dst, err = fasthttp.AppendGunzipBytes(dst, src)
		if err != nil {
			fasthttp.ReleaseResponse(resp)
			scrapesGunzipFailed.Inc()
			return dst, fmt.Errorf("cannot ungzip response from %q: %w", c.scrapeURL, err)
		}
		scrapesGunzipped.Inc()
	} else if !swapResponseBodies {
		dst = append(dst, resp.Body()...)
	}
	fasthttp.ReleaseResponse(resp)
	if statusCode != fasthttp.StatusOK {
		metrics.GetOrCreateCounter(fmt.Sprintf(`vm_promscrape_scrapes_total{status_code="%d"}`, statusCode)).Inc()
		return dst, fmt.Errorf("unexpected status code returned when scraping %q: %d; expecting %d; response body: %q",
			c.scrapeURL, statusCode, fasthttp.StatusOK, dst)
	}
	scrapesOK.Inc()
	return dst, nil
}

var (
	scrapesTimedout     = metrics.NewCounter(`vm_promscrape_scrapes_timed_out_total`)
	scrapesOK           = metrics.NewCounter(`vm_promscrape_scrapes_total{status_code="200"}`)
	scrapesGunzipped    = metrics.NewCounter(`vm_promscrape_scrapes_gunziped_total`)
	scrapesGunzipFailed = metrics.NewCounter(`vm_promscrape_scrapes_gunzip_failed_total`)
)

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

type streamReader struct {
	r         io.ReadCloser
	cancel    context.CancelFunc
	bytesRead int64
}

func (sr *streamReader) Read(p []byte) (int, error) {
	n, err := sr.r.Read(p)
	sr.bytesRead += int64(n)
	return n, err
}

func (sr *streamReader) MustClose() {
	sr.cancel()
	if err := sr.r.Close(); err != nil {
		logger.Errorf("cannot close reader: %s", err)
	}
}
