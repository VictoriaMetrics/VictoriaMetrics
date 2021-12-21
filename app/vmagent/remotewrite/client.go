package remotewrite

import (
	"bytes"
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
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/timerpool"
	"github.com/VictoriaMetrics/metrics"
)

var (
	rateLimit = flagutil.NewArrayInt("remoteWrite.rateLimit", "Optional rate limit in bytes per second for data sent to -remoteWrite.url. "+
		"By default the rate limit is disabled. It can be useful for limiting load on remote storage when big amounts of buffered data "+
		"is sent after temporary unavailability of the remote storage")
	sendTimeout = flagutil.NewArrayDuration("remoteWrite.sendTimeout", "Timeout for sending a single block of data to -remoteWrite.url")
	proxyURL    = flagutil.NewArray("remoteWrite.proxyURL", "Optional proxy URL for writing data to -remoteWrite.url. Supported proxies: http, https, socks5. "+
		"Example: -remoteWrite.proxyURL=socks5://proxy:1234")

	tlsInsecureSkipVerify = flagutil.NewArrayBool("remoteWrite.tlsInsecureSkipVerify", "Whether to skip tls verification when connecting to -remoteWrite.url")
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
	basicAuthPasswordFile = flagutil.NewArray("remoteWrite.basicAuth.passwordFile", "Optional path to basic auth password to use for -remoteWrite.url. "+
		"The file is re-read every second. "+
		"If multiple args are set, then they are applied independently for the corresponding -remoteWrite.url")
	bearerToken = flagutil.NewArray("remoteWrite.bearerToken", "Optional bearer auth token to use for -remoteWrite.url. "+
		"If multiple args are set, then they are applied independently for the corresponding -remoteWrite.url")
	bearerTokenFile = flagutil.NewArray("remoteWrite.bearerTokenFile", "Optional path to bearer token file to use for -remoteWrite.url. "+
		"The token is re-read from the file every second. "+
		"If multiple args are set, then they are applied independently for the corresponding -remoteWrite.url")

	oauth2ClientID = flagutil.NewArray("remoteWrite.oauth2.clientID", "Optional OAuth2 clientID to use for -remoteWrite.url. "+
		"If multiple args are set, then they are applied independently for the corresponding -remoteWrite.url")
	oauth2ClientSecret = flagutil.NewArray("remoteWrite.oauth2.clientSecret", "Optional OAuth2 clientSecret to use for -remoteWrite.url. "+
		"If multiple args are set, then they are applied independently for the corresponding -remoteWrite.url")
	oauth2ClientSecretFile = flagutil.NewArray("remoteWrite.oauth2.clientSecretFile", "Optional OAuth2 clientSecretFile to use for -remoteWrite.url. "+
		"If multiple args are set, then they are applied independently for the corresponding -remoteWrite.url")
	oauth2TokenURL = flagutil.NewArray("remoteWrite.oauth2.tokenUrl", "Optional OAuth2 tokenURL to use for -remoteWrite.url. "+
		"If multiple args are set, then they are applied independently for the corresponding -remoteWrite.url")
	oauth2Scopes = flagutil.NewArray("remoteWrite.oauth2.scopes", "Optional OAuth2 scopes to use for -remoteWrite.url. Scopes must be delimited by ';'. "+
		"If multiple args are set, then they are applied independently for the corresponding -remoteWrite.url")
)

type client struct {
	sanitizedURL   string
	remoteWriteURL string
	fq             *persistentqueue.FastQueue
	hc             *http.Client

	sendBlock func(block []byte) bool
	authCfg   *promauth.Config

	rl rateLimiter

	bytesSent       *metrics.Counter
	blocksSent      *metrics.Counter
	requestDuration *metrics.Histogram
	requestsOKCount *metrics.Counter
	errorsCount     *metrics.Counter
	packetsDropped  *metrics.Counter
	retriesCount    *metrics.Counter
	sendDuration    *metrics.FloatCounter

	wg     sync.WaitGroup
	stopCh chan struct{}
}

func newHTTPClient(argIdx int, remoteWriteURL, sanitizedURL string, fq *persistentqueue.FastQueue, concurrency int) *client {
	authCfg, err := getAuthConfig(argIdx)
	if err != nil {
		logger.Panicf("FATAL: cannot initialize auth config: %s", err)
	}
	tlsCfg := authCfg.NewTLSConfig()
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
		pu, err := url.Parse(pURL)
		if err != nil {
			logger.Fatalf("cannot parse -remoteWrite.proxyURL=%q: %s", pURL, err)
		}
		tr.Proxy = http.ProxyURL(pu)
	}
	c := &client{
		sanitizedURL:   sanitizedURL,
		remoteWriteURL: remoteWriteURL,
		authCfg:        authCfg,
		fq:             fq,
		hc: &http.Client{
			Transport: tr,
			Timeout:   sendTimeout.GetOptionalArgOrDefault(argIdx, time.Minute),
		},
		stopCh: make(chan struct{}),
	}
	c.sendBlock = c.sendBlockHTTP
	return c
}

func (c *client) init(argIdx, concurrency int, sanitizedURL string) {
	if bytesPerSec := rateLimit.GetOptionalArgOrDefault(argIdx, 0); bytesPerSec > 0 {
		logger.Infof("applying %d bytes per second rate limit for -remoteWrite.url=%q", bytesPerSec, sanitizedURL)
		c.rl.perSecondLimit = int64(bytesPerSec)
	}
	c.rl.limitReached = metrics.GetOrCreateCounter(fmt.Sprintf(`vmagent_remote_write_rate_limit_reached_total{url=%q}`, c.sanitizedURL))

	c.bytesSent = metrics.GetOrCreateCounter(fmt.Sprintf(`vmagent_remotewrite_bytes_sent_total{url=%q}`, c.sanitizedURL))
	c.blocksSent = metrics.GetOrCreateCounter(fmt.Sprintf(`vmagent_remotewrite_blocks_sent_total{url=%q}`, c.sanitizedURL))
	c.requestDuration = metrics.GetOrCreateHistogram(fmt.Sprintf(`vmagent_remotewrite_duration_seconds{url=%q}`, c.sanitizedURL))
	c.requestsOKCount = metrics.GetOrCreateCounter(fmt.Sprintf(`vmagent_remotewrite_requests_total{url=%q, status_code="2XX"}`, c.sanitizedURL))
	c.errorsCount = metrics.GetOrCreateCounter(fmt.Sprintf(`vmagent_remotewrite_errors_total{url=%q}`, c.sanitizedURL))
	c.packetsDropped = metrics.GetOrCreateCounter(fmt.Sprintf(`vmagent_remotewrite_packets_dropped_total{url=%q}`, c.sanitizedURL))
	c.retriesCount = metrics.GetOrCreateCounter(fmt.Sprintf(`vmagent_remotewrite_retries_count_total{url=%q}`, c.sanitizedURL))
	c.sendDuration = metrics.GetOrCreateFloatCounter(fmt.Sprintf(`vmagent_remotewrite_send_duration_seconds_total{url=%q}`, c.sanitizedURL))
	for i := 0; i < concurrency; i++ {
		c.wg.Add(1)
		go func() {
			defer c.wg.Done()
			c.runWorker()
		}()
	}
	logger.Infof("initialized client for -remoteWrite.url=%q", c.sanitizedURL)
}

func (c *client) MustStop() {
	close(c.stopCh)
	c.wg.Wait()
	logger.Infof("stopped client for -remoteWrite.url=%q", c.sanitizedURL)
}

func getAuthConfig(argIdx int) (*promauth.Config, error) {
	username := basicAuthUsername.GetOptionalArg(argIdx)
	password := basicAuthPassword.GetOptionalArg(argIdx)
	passwordFile := basicAuthPasswordFile.GetOptionalArg(argIdx)
	var basicAuthCfg *promauth.BasicAuthConfig
	if username != "" || password != "" || passwordFile != "" {
		basicAuthCfg = &promauth.BasicAuthConfig{
			Username:     username,
			Password:     promauth.NewSecret(password),
			PasswordFile: passwordFile,
		}
	}

	token := bearerToken.GetOptionalArg(argIdx)
	tokenFile := bearerTokenFile.GetOptionalArg(argIdx)

	var oauth2Cfg *promauth.OAuth2Config
	clientSecret := oauth2ClientSecret.GetOptionalArg(argIdx)
	clientSecretFile := oauth2ClientSecretFile.GetOptionalArg(argIdx)
	if clientSecretFile != "" || clientSecret != "" {
		oauth2Cfg = &promauth.OAuth2Config{
			ClientID:         oauth2ClientID.GetOptionalArg(argIdx),
			ClientSecret:     promauth.NewSecret(clientSecret),
			ClientSecretFile: clientSecretFile,
			TokenURL:         oauth2TokenURL.GetOptionalArg(argIdx),
			Scopes:           strings.Split(oauth2Scopes.GetOptionalArg(argIdx), ";"),
		}
	}

	tlsCfg := &promauth.TLSConfig{
		CAFile:             tlsCAFile.GetOptionalArg(argIdx),
		CertFile:           tlsCertFile.GetOptionalArg(argIdx),
		KeyFile:            tlsKeyFile.GetOptionalArg(argIdx),
		ServerName:         tlsServerName.GetOptionalArg(argIdx),
		InsecureSkipVerify: tlsInsecureSkipVerify.GetOptionalArg(argIdx),
	}

	authCfg, err := promauth.NewConfig(".", nil, basicAuthCfg, token, tokenFile, oauth2Cfg, tlsCfg)
	if err != nil {
		return nil, fmt.Errorf("cannot populate OAuth2 config for remoteWrite idx: %d, err: %w", argIdx, err)
	}
	return authCfg, nil
}

func (c *client) runWorker() {
	var ok bool
	var block []byte
	ch := make(chan bool, 1)
	for {
		block, ok = c.fq.MustReadBlock(block[:0])
		if !ok {
			return
		}
		go func() {
			startTime := time.Now()
			ch <- c.sendBlock(block)
			c.sendDuration.Add(time.Since(startTime).Seconds())
		}()
		select {
		case ok := <-ch:
			if ok {
				// The block has been sent successfully
				continue
			}
			// Return unsent block to the queue.
			c.fq.MustWriteBlock(block)
			return
		case <-c.stopCh:
			// c must be stopped. Wait for a while in the hope the block will be sent.
			graceDuration := 5 * time.Second
			select {
			case ok := <-ch:
				if !ok {
					// Return unsent block to the queue.
					c.fq.MustWriteBlock(block)
				}
			case <-time.After(graceDuration):
				// Return unsent block to the queue.
				c.fq.MustWriteBlock(block)
			}
			return
		}
	}
}

// sendBlockHTTP returns false only if c.stopCh is closed.
// Otherwise it tries sending the block to remote storage indefinitely.
func (c *client) sendBlockHTTP(block []byte) bool {
	c.rl.register(len(block), c.stopCh)
	retryDuration := time.Second
	retriesCount := 0
	c.bytesSent.Add(len(block))
	c.blocksSent.Inc()

again:
	req, err := http.NewRequest("POST", c.remoteWriteURL, bytes.NewBuffer(block))
	if err != nil {
		logger.Panicf("BUG: unexpected error from http.NewRequest(%q): %s", c.sanitizedURL, err)
	}
	h := req.Header
	h.Set("User-Agent", "vmagent")
	h.Set("Content-Type", "application/x-protobuf")
	h.Set("Content-Encoding", "snappy")
	h.Set("X-Prometheus-Remote-Write-Version", "0.1.0")
	if ah := c.authCfg.GetAuthHeader(); ah != "" {
		req.Header.Set("Authorization", ah)
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
		logger.Warnf("couldn't send a block with size %d bytes to %q: %s; re-sending the block in %.3f seconds",
			len(block), c.sanitizedURL, err, retryDuration.Seconds())
		t := timerpool.Get(retryDuration)
		select {
		case <-c.stopCh:
			timerpool.Put(t)
			return false
		case <-t.C:
			timerpool.Put(t)
		}
		c.retriesCount.Inc()
		goto again
	}
	statusCode := resp.StatusCode
	if statusCode/100 == 2 {
		_ = resp.Body.Close()
		c.requestsOKCount.Inc()
		return true
	}
	metrics.GetOrCreateCounter(fmt.Sprintf(`vmagent_remotewrite_requests_total{url=%q, status_code="%d"}`, c.sanitizedURL, statusCode)).Inc()
	if statusCode == 409 || statusCode == 400 {
		body, err := ioutil.ReadAll(resp.Body)
		_ = resp.Body.Close()
		l := logger.WithThrottler("remoteWriteRejected", 5*time.Second)
		if err != nil {
			l.Errorf("sending a block with size %d bytes to %q was rejected (skipping the block): status code %d; "+
				"failed to read response body: %s",
				len(block), c.sanitizedURL, statusCode, err)
		} else {
			l.Errorf("sending a block with size %d bytes to %q was rejected (skipping the block): status code %d; response body: %s",
				len(block), c.sanitizedURL, statusCode, string(body))
		}
		// Just drop block on 409 and 400 status codes like Prometheus does.
		// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/873
		// and https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1149
		_ = resp.Body.Close()
		c.packetsDropped.Inc()
		return true
	}

	// Unexpected status code returned
	retriesCount++
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
	t := timerpool.Get(retryDuration)
	select {
	case <-c.stopCh:
		timerpool.Put(t)
		return false
	case <-t.C:
		timerpool.Put(t)
	}
	c.retriesCount.Inc()
	goto again
}

type rateLimiter struct {
	perSecondLimit int64

	// mu protects budget and deadline from concurrent access.
	mu sync.Mutex

	// The current budget. It is increased by perSecondLimit every second.
	budget int64

	// The next deadline for increasing the budget by perSecondLimit
	deadline time.Time

	limitReached *metrics.Counter
}

func (rl *rateLimiter) register(dataLen int, stopCh <-chan struct{}) {
	limit := rl.perSecondLimit
	if limit <= 0 {
		return
	}

	rl.mu.Lock()
	defer rl.mu.Unlock()

	for rl.budget <= 0 {
		if d := time.Until(rl.deadline); d > 0 {
			rl.limitReached.Inc()
			t := timerpool.Get(d)
			select {
			case <-stopCh:
				timerpool.Put(t)
				return
			case <-t.C:
				timerpool.Put(t)
			}
		}
		rl.budget += limit
		rl.deadline = time.Now().Add(time.Second)
	}
	rl.budget -= int64(dataLen)
}
