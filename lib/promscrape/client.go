package promscrape

import (
	"crypto/tls"
	"flag"
	"fmt"
	"strings"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/netutil"
	"github.com/VictoriaMetrics/metrics"
	"github.com/valyala/fasthttp"
)

var (
	maxScrapeSize = flag.Int("promscrape.maxScrapeSize", 16*1024*1024, "The maximum size of scrape response in bytes to process from Prometheus targets. "+
		"Bigger responses are rejected")
	disableCompression = flag.Bool("promscrape.disableCompression", false, "Whether to disable sending 'Accept-Encoding: gzip' request headers to scrape targets. "+
		"This may reduce CPU usage on scrape targets at the cost of higher network bandwidth utilization")
)

type client struct {
	hc *fasthttp.HostClient

	scrapeURL  string
	host       string
	requestURI string
	authHeader string
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
		Addr:                      host,
		Name:                      "vm_promscrape",
		Dial:                      statDial,
		DialDualStack:             netutil.TCP6Enabled(),
		IsTLS:                     isTLS,
		TLSConfig:                 tlsCfg,
		MaxIdleConnDuration:       2 * sw.ScrapeInterval,
		ReadTimeout:               sw.ScrapeTimeout,
		WriteTimeout:              10 * time.Second,
		MaxResponseBodySize:       *maxScrapeSize,
		MaxIdemponentCallAttempts: 1,
	}
	return &client{
		hc: hc,

		scrapeURL:  sw.ScrapeURL,
		host:       host,
		requestURI: requestURI,
		authHeader: sw.AuthConfig.Authorization,
	}
}

func (c *client) ReadData(dst []byte) ([]byte, error) {
	req := fasthttp.AcquireRequest()
	req.SetRequestURI(c.requestURI)
	req.SetHost(c.host)
	if !*disableCompression {
		req.Header.Set("Accept-Encoding", "gzip")
	}
	if c.authHeader != "" {
		req.Header.Set("Authorization", c.authHeader)
	}
	resp := fasthttp.AcquireResponse()
	err := doRequestWithPossibleRetry(c.hc, req, resp)

	fasthttp.ReleaseRequest(req)
	if err != nil {
		fasthttp.ReleaseResponse(resp)
		if err == fasthttp.ErrTimeout {
			scrapesTimedout.Inc()
			return dst, fmt.Errorf("error when scraping %q with timeout %s: %s", c.scrapeURL, c.hc.ReadTimeout, err)
		}
		return dst, fmt.Errorf("error when scraping %q: %s", c.scrapeURL, err)
	}
	dstLen := len(dst)
	if ce := resp.Header.Peek("Content-Encoding"); string(ce) == "gzip" {
		var err error
		dst, err = fasthttp.AppendGunzipBytes(dst, resp.Body())
		if err != nil {
			fasthttp.ReleaseResponse(resp)
			scrapesGunzipFailed.Inc()
			return dst, fmt.Errorf("cannot ungzip response from %q: %s", c.scrapeURL, err)
		}
		scrapesGunzipped.Inc()
	} else {
		dst = append(dst, resp.Body()...)
	}
	statusCode := resp.StatusCode()
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

func doRequestWithPossibleRetry(hc *fasthttp.HostClient, req *fasthttp.Request, resp *fasthttp.Response) error {
	// There is no need in calling DoTimeout, since the timeout must be already set in hc.ReadTimeout.
	err := hc.Do(req, resp)
	if err == nil {
		return nil
	}
	if err != fasthttp.ErrConnectionClosed {
		return err
	}
	// Retry request if the server closed the keep-alive connection during the first attempt.
	return hc.Do(req, resp)
}
