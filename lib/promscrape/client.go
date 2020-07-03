package promscrape

import (
	"crypto/tls"
	"flag"
	"fmt"
	"strings"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/fasthttp"
	"github.com/VictoriaMetrics/metrics"
)

var (
	maxScrapeSize = flag.Int("promscrape.maxScrapeSize", 16*1024*1024, "The maximum size of scrape response in bytes to process from Prometheus targets. "+
		"Bigger responses are rejected")
	disableCompression = flag.Bool("promscrape.disableCompression", false, "Whether to disable sending 'Accept-Encoding: gzip' request headers to all the scrape targets. "+
		"This may reduce CPU usage on scrape targets at the cost of higher network bandwidth utilization. "+
		"It is possible to set 'disable_compression: true' individually per each 'scrape_config' section in '-promscrape.config' for fine grained control")
	disableKeepAlive = flag.Bool("promscrape.disableKeepAlive", false, "Whether to disable HTTP keep-alive connections when scraping all the targets. "+
		"This may be useful when targets has no support for HTTP keep-alive connection. "+
		"It is possible to set `disable_keepalive: true` individually per each 'scrape_config` section in '-promscrape.config' for fine grained control. "+
		"Note that disabling HTTP keep-alive may increase load on both vmagent and scrape targets")
)

type client struct {
	hc *fasthttp.HostClient

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
		MaxResponseBodySize:          *maxScrapeSize,
		MaxIdempotentRequestAttempts: 1,
	}
	return &client{
		hc: hc,

		scrapeURL:          sw.ScrapeURL,
		host:               host,
		requestURI:         requestURI,
		authHeader:         sw.AuthConfig.Authorization,
		disableCompression: sw.DisableCompression,
		disableKeepAlive:   sw.DisableKeepAlive,
	}
}

func (c *client) ReadData(dst []byte) ([]byte, error) {
	req := fasthttp.AcquireRequest()
	req.SetRequestURI(c.requestURI)
	req.SetHost(c.host)
	if !*disableCompression || c.disableCompression {
		req.Header.Set("Accept-Encoding", "gzip")
	}
	if *disableKeepAlive || c.disableKeepAlive {
		req.SetConnectionClose()
	}
	if c.authHeader != "" {
		req.Header.Set("Authorization", c.authHeader)
	}
	resp := fasthttp.AcquireResponse()
	err := doRequestWithPossibleRetry(c.hc, req, resp, c.hc.ReadTimeout)
	statusCode := resp.StatusCode()
	if err == nil && (statusCode == fasthttp.StatusMovedPermanently || statusCode == fasthttp.StatusFound) {
		// Allow a single redirect.
		// It is expected that the redirect is made on the same host.
		// Otherwise it won't work.
		if location := resp.Header.Peek("Location"); len(location) > 0 {
			req.URI().UpdateBytes(location)
			err = c.hc.Do(req, resp)
			statusCode = resp.StatusCode()
		}
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
				"either reduce the response size for the target or increase -promscrape.maxScrapeSize", c.scrapeURL, *maxScrapeSize)
		}
		return dst, fmt.Errorf("error when scraping %q: %w", c.scrapeURL, err)
	}
	dstLen := len(dst)
	if ce := resp.Header.Peek("Content-Encoding"); string(ce) == "gzip" {
		var err error
		dst, err = fasthttp.AppendGunzipBytes(dst, resp.Body())
		if err != nil {
			fasthttp.ReleaseResponse(resp)
			scrapesGunzipFailed.Inc()
			return dst, fmt.Errorf("cannot ungzip response from %q: %w", c.scrapeURL, err)
		}
		scrapesGunzipped.Inc()
	} else {
		dst = append(dst, resp.Body()...)
	}
	if statusCode != fasthttp.StatusOK {
		metrics.GetOrCreateCounter(fmt.Sprintf(`vm_promscrape_scrapes_total{status_code="%d"}`, statusCode)).Inc()
		return dst, fmt.Errorf("unexpected status code returned when scraping %q: %d; expecting %d; response body: %q",
			c.scrapeURL, statusCode, fasthttp.StatusOK, dst[dstLen:])
	}
	scrapesOK.Inc()
	fasthttp.ReleaseResponse(resp)
	return dst, nil
}

var (
	scrapesTimedout     = metrics.NewCounter(`vm_promscrape_scrapes_timed_out_total`)
	scrapesOK           = metrics.NewCounter(`vm_promscrape_scrapes_total{status_code="200"}`)
	scrapesGunzipped    = metrics.NewCounter(`vm_promscrape_scrapes_gunziped_total`)
	scrapesGunzipFailed = metrics.NewCounter(`vm_promscrape_scrapes_gunzip_failed_total`)
)

func doRequestWithPossibleRetry(hc *fasthttp.HostClient, req *fasthttp.Request, resp *fasthttp.Response, timeout time.Duration) error {
	// Round deadline to the smallest value in order to protect from too big deadline misses on retry.
	deadline := fasttime.UnixTimestamp() + uint64(timeout.Seconds()) - 1
	attempts := 0
again:
	// There is no need in calling DoTimeout, since the timeout must be already set in hc.ReadTimeout.
	err := hc.Do(req, resp)
	if err == nil {
		return nil
	}
	if err != fasthttp.ErrConnectionClosed {
		return err
	}
	// Retry request if the server closes the keep-alive connection unless deadline exceeds.
	if fasttime.UnixTimestamp() > deadline {
		return fasthttp.ErrTimeout
	}
	attempts++
	if attempts > 3 {
		return fmt.Errorf("the server closed 3 subsequent connections: %w", err)
	}
	goto again
}
