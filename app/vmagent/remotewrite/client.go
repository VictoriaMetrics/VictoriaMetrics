package remotewrite

import (
	"bytes"
	"crypto/tls"
	"encoding/base64"
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/persistentqueue"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
	"github.com/VictoriaMetrics/metrics"
)

var (
	sendTimeout = flag.Duration("remoteWrite.sendTimeout", time.Minute, "Timeout for sending a single block of data to -remoteWrite.url")
	proxyURL    = flagutil.NewArray("remoteWrite.proxyURL", "Optional proxy URL for writing data to -remoteWrite.url. Supported proxies: http, https, socks5. "+
		"Example: -remoteWrite.proxyURL=socks5://proxy:1234")

	tlsInsecureSkipVerify = flag.Bool("remoteWrite.tlsInsecureSkipVerify", false, "Whether to skip tls verification when connecting to -remoteWrite.url")
	tlsCertFile           = flagutil.NewArray("remoteWrite.tlsCertFile", "Optional path to client-side TLS certificate file to use when connecting to -remoteWrite.url. "+
		"If multiple args are set, then they are applied independently for the corresponding -remoteWrite.url")
	tlsKeyFile = flagutil.NewArray("remoteWrite.tlsKeyFile", "Optional path to client-side TLS certificate key to use when connecting to -remoteWrite.url. "+
		"If multiple args are set, then they are applied independently for the corresponding -remoteWrite.url")
	tlsCAFile = flagutil.NewArray("remoteWrite.tlsCAFile", "Optional path to TLS CA file to use for verifying connections to -remoteWrite.url. "+
		"By default system CA is used. If multiple args are set, then they are applied independently for the corresponding -remoteWrite.url")
	tlsServerName = flagutil.NewArray("remoteWrite.tlsServerName", "Optional TLS server name to use for connections to -remoteWrite.url. "+
		"By default the server name from -remoteWrite.url is used. If multiple args are set, then they are applied independently for the corresponding -remoteWrite.url")

	basicAuthUsername = flagutil.NewArray("remoteWrite.basicAuth.username", "Optional basic auth username to use for -remoteWrite.url. "+
		"If multiple args are set, then they are applied independently for the corresponding -remoteWrite.url")
	basicAuthPassword = flagutil.NewArray("remoteWrite.basicAuth.password", "Optional basic auth password to use for -remoteWrite.url. "+
		"If multiple args are set, then they are applied independently for the corresponding -remoteWrite.url")
	bearerToken = flagutil.NewArray("remoteWrite.bearerToken", "Optional bearer auth token to use for -remoteWrite.url. "+
		"If multiple args are set, then they are applied independently for the corresponding -remoteWrite.url")
)

type client struct {
	sanitizedURL   string
	remoteWriteURL string
	authHeader     string
	fq             *persistentqueue.FastQueue
	hc             *http.Client

	requestDuration *metrics.Histogram
	requestsOKCount *metrics.Counter
	errorsCount     *metrics.Counter
	retriesCount    *metrics.Counter

	wg     sync.WaitGroup
	stopCh chan struct{}
}

func newClient(argIdx int, remoteWriteURL, sanitizedURL string, fq *persistentqueue.FastQueue, concurrency int) *client {
	tlsCfg, err := getTLSConfig(argIdx)
	if err != nil {
		logger.Panicf("FATAL: cannot initialize TLS config: %s", err)
	}
	tr := &http.Transport{
		Dial:                statDial,
		TLSClientConfig:     tlsCfg,
		TLSHandshakeTimeout: 5 * time.Second,
		MaxConnsPerHost:     2 * concurrency,
		MaxIdleConnsPerHost: 2 * concurrency,
		IdleConnTimeout:     time.Minute,
		WriteBufferSize:     64 * 1024,
	}
	pURL := proxyURL.GetOptionalArg(argIdx)
	if len(pURL) > 0 {
		if !strings.Contains(pURL, "://") {
			logger.Fatalf("cannot parse -remoteWrite.proxyURL=%q: it must start with `http://`, `https://` or `socks5://`", pURL)
		}
		urlProxy, err := url.Parse(pURL)
		if err != nil {
			logger.Fatalf("cannot parse -remoteWrite.proxyURL=%q: %s", pURL, err)
		}
		tr.Proxy = http.ProxyURL(urlProxy)
	}
	authHeader := ""
	username := basicAuthUsername.GetOptionalArg(argIdx)
	password := basicAuthPassword.GetOptionalArg(argIdx)
	if len(username) > 0 || len(password) > 0 {
		// See https://en.wikipedia.org/wiki/Basic_access_authentication
		token := username + ":" + password
		token64 := base64.StdEncoding.EncodeToString([]byte(token))
		authHeader = "Basic " + token64
	}
	token := bearerToken.GetOptionalArg(argIdx)
	if len(token) > 0 {
		if authHeader != "" {
			logger.Fatalf("`-remoteWrite.bearerToken`=%q cannot be set when `-remoteWrite.basicAuth.*` flags are set", token)
		}
		authHeader = "Bearer " + token
	}
	c := &client{
		sanitizedURL:   sanitizedURL,
		remoteWriteURL: remoteWriteURL,
		authHeader:     authHeader,
		fq:             fq,
		hc: &http.Client{
			Transport: tr,
			Timeout:   *sendTimeout,
		},
		stopCh: make(chan struct{}),
	}
	c.requestDuration = metrics.GetOrCreateHistogram(fmt.Sprintf(`vmagent_remotewrite_duration_seconds{url=%q}`, c.sanitizedURL))
	c.requestsOKCount = metrics.GetOrCreateCounter(fmt.Sprintf(`vmagent_remotewrite_requests_total{url=%q, status_code="2XX"}`, c.sanitizedURL))
	c.errorsCount = metrics.GetOrCreateCounter(fmt.Sprintf(`vmagent_remotewrite_errors_total{url=%q}`, c.sanitizedURL))
	c.retriesCount = metrics.GetOrCreateCounter(fmt.Sprintf(`vmagent_remotewrite_retries_count_total{url=%q}`, c.sanitizedURL))
	for i := 0; i < concurrency; i++ {
		c.wg.Add(1)
		go func() {
			defer c.wg.Done()
			c.runWorker()
		}()
	}
	logger.Infof("initialized client for -remoteWrite.url=%q", c.sanitizedURL)
	return c
}

func (c *client) MustStop() {
	close(c.stopCh)
	c.wg.Wait()
	logger.Infof("stopped client for -remoteWrite.url=%q", c.sanitizedURL)
}

func getTLSConfig(argIdx int) (*tls.Config, error) {
	c := &promauth.TLSConfig{
		CAFile:             tlsCAFile.GetOptionalArg(argIdx),
		CertFile:           tlsCertFile.GetOptionalArg(argIdx),
		KeyFile:            tlsKeyFile.GetOptionalArg(argIdx),
		ServerName:         tlsServerName.GetOptionalArg(argIdx),
		InsecureSkipVerify: *tlsInsecureSkipVerify,
	}
	if c.CAFile == "" && c.CertFile == "" && c.KeyFile == "" && c.ServerName == "" && !c.InsecureSkipVerify {
		return nil, nil
	}
	cfg, err := promauth.NewConfig(".", nil, "", "", c)
	if err != nil {
		return nil, fmt.Errorf("cannot populate TLS config: %w", err)
	}
	tlsCfg := cfg.NewTLSConfig()
	return tlsCfg, nil
}

func (c *client) runWorker() {
	var ok bool
	var block []byte
	ch := make(chan struct{})
	for {
		block, ok = c.fq.MustReadBlock(block[:0])
		if !ok {
			return
		}
		go func() {
			c.sendBlock(block)
			ch <- struct{}{}
		}()
		select {
		case <-ch:
			// The block has been sent successfully
			continue
		case <-c.stopCh:
			// c must be stopped. Wait for a while in the hope the block will be sent.
			graceDuration := 5 * time.Second
			select {
			case <-ch:
				// The block has been sent successfully.
			case <-time.After(graceDuration):
				logger.Errorf("couldn't sent block with size %d bytes to %q in %.3f seconds during shutdown; dropping it",
					len(block), c.sanitizedURL, graceDuration.Seconds())
			}
			return
		}
	}
}

func (c *client) sendBlock(block []byte) {
	retryDuration := time.Second
	retriesCount := 0

again:
	req, err := http.NewRequest("POST", c.remoteWriteURL, bytes.NewBuffer(block))
	if err != nil {
		logger.Panicf("BUG: unexected error from http.NewRequest(%q): %s", c.sanitizedURL, err)
	}
	h := req.Header
	h.Set("User-Agent", "vmagent")
	h.Set("Content-Type", "application/x-protobuf")
	h.Set("Content-Encoding", "snappy")
	h.Set("X-Prometheus-Remote-Write-Version", "0.1.0")
	if c.authHeader != "" {
		req.Header.Set("Authorization", c.authHeader)
	}

	startTime := time.Now()
	resp, err := c.hc.Do(req)
	c.requestDuration.UpdateDuration(startTime)
	if err != nil {
		c.errorsCount.Inc()
		retryDuration *= 2
		if retryDuration > time.Minute {
			retryDuration = time.Minute
		}
		logger.Errorf("couldn't send a block with size %d bytes to %q: %s; re-sending the block in %.3f seconds",
			len(block), c.sanitizedURL, err, retryDuration.Seconds())
		t := time.NewTimer(retryDuration)
		select {
		case <-c.stopCh:
			t.Stop()
			return
		case <-t.C:
		}
		c.retriesCount.Inc()
		goto again
	}
	statusCode := resp.StatusCode
	if statusCode/100 == 2 {
		_ = resp.Body.Close()
		c.requestsOKCount.Inc()
		return
	}

	// Unexpected status code returned
	retriesCount++
	metrics.GetOrCreateCounter(fmt.Sprintf(`vmagent_remotewrite_requests_total{url=%q, status_code="%d"}`, c.sanitizedURL, statusCode)).Inc()
	retryDuration *= 2
	if retryDuration > time.Minute {
		retryDuration = time.Minute
	}
	body, err := ioutil.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		logger.Errorf("cannot read response body from %q during retry #%d: %s", c.sanitizedURL, retriesCount, err)
	} else {
		logger.Errorf("unexpected status code received after sending a block with size %d bytes to %q during retry #%d: %d; response body=%q; "+
			"re-sending the block in %.3f seconds", len(block), c.sanitizedURL, retriesCount, statusCode, body, retryDuration.Seconds())
	}
	t := time.NewTimer(retryDuration)
	select {
	case <-c.stopCh:
		t.Stop()
		return
	case <-t.C:
	}
	c.retriesCount.Inc()
	goto again
}
