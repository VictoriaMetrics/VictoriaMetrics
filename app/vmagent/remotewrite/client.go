package remotewrite

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/awsapi"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/persistentqueue"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/common"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/timerpool"
	"github.com/VictoriaMetrics/metrics"
)

var (
	forcePromProto = flagutil.NewArrayBool("remoteWrite.forcePromProto", "Whether to force Prometheus remote write protocol for sending data "+
		"to the corresponding -remoteWrite.url . See https://docs.victoriametrics.com/vmagent.html#victoriametrics-remote-write-protocol")
	forceVMProto = flagutil.NewArrayBool("remoteWrite.forceVMProto", "Whether to force VictoriaMetrics remote write protocol for sending data "+
		"to the corresponding -remoteWrite.url . See https://docs.victoriametrics.com/vmagent.html#victoriametrics-remote-write-protocol")

	rateLimit = flagutil.NewArrayInt("remoteWrite.rateLimit", 0, "Optional rate limit in bytes per second for data sent to the corresponding -remoteWrite.url. "+
		"By default, the rate limit is disabled. It can be useful for limiting load on remote storage when big amounts of buffered data "+
		"is sent after temporary unavailability of the remote storage")
	sendTimeout = flagutil.NewArrayDuration("remoteWrite.sendTimeout", time.Minute, "Timeout for sending a single block of data to the corresponding -remoteWrite.url")
	proxyURL    = flagutil.NewArrayString("remoteWrite.proxyURL", "Optional proxy URL for writing data to the corresponding -remoteWrite.url. "+
		"Supported proxies: http, https, socks5. Example: -remoteWrite.proxyURL=socks5://proxy:1234")

	tlsInsecureSkipVerify = flagutil.NewArrayBool("remoteWrite.tlsInsecureSkipVerify", "Whether to skip tls verification when connecting to the corresponding -remoteWrite.url")
	tlsCertFile           = flagutil.NewArrayString("remoteWrite.tlsCertFile", "Optional path to client-side TLS certificate file to use when connecting "+
		"to the corresponding -remoteWrite.url")
	tlsKeyFile = flagutil.NewArrayString("remoteWrite.tlsKeyFile", "Optional path to client-side TLS certificate key to use when connecting to the corresponding -remoteWrite.url")
	tlsCAFile  = flagutil.NewArrayString("remoteWrite.tlsCAFile", "Optional path to TLS CA file to use for verifying connections to the corresponding -remoteWrite.url. "+
		"By default, system CA is used")
	tlsServerName = flagutil.NewArrayString("remoteWrite.tlsServerName", "Optional TLS server name to use for connections to the corresponding -remoteWrite.url. "+
		"By default, the server name from -remoteWrite.url is used")

	headers = flagutil.NewArrayString("remoteWrite.headers", "Optional HTTP headers to send with each request to the corresponding -remoteWrite.url. "+
		"For example, -remoteWrite.headers='My-Auth:foobar' would send 'My-Auth: foobar' HTTP header with every request to the corresponding -remoteWrite.url. "+
		"Multiple headers must be delimited by '^^': -remoteWrite.headers='header1:value1^^header2:value2'")

	basicAuthUsername     = flagutil.NewArrayString("remoteWrite.basicAuth.username", "Optional basic auth username to use for the corresponding -remoteWrite.url")
	basicAuthPassword     = flagutil.NewArrayString("remoteWrite.basicAuth.password", "Optional basic auth password to use for the corresponding -remoteWrite.url")
	basicAuthPasswordFile = flagutil.NewArrayString("remoteWrite.basicAuth.passwordFile", "Optional path to basic auth password to use for the corresponding -remoteWrite.url. "+
		"The file is re-read every second")
	bearerToken     = flagutil.NewArrayString("remoteWrite.bearerToken", "Optional bearer auth token to use for the corresponding -remoteWrite.url")
	bearerTokenFile = flagutil.NewArrayString("remoteWrite.bearerTokenFile", "Optional path to bearer token file to use for the corresponding -remoteWrite.url. "+
		"The token is re-read from the file every second")

	oauth2ClientID         = flagutil.NewArrayString("remoteWrite.oauth2.clientID", "Optional OAuth2 clientID to use for the corresponding -remoteWrite.url")
	oauth2ClientSecret     = flagutil.NewArrayString("remoteWrite.oauth2.clientSecret", "Optional OAuth2 clientSecret to use for the corresponding -remoteWrite.url")
	oauth2ClientSecretFile = flagutil.NewArrayString("remoteWrite.oauth2.clientSecretFile", "Optional OAuth2 clientSecretFile to use for the corresponding -remoteWrite.url")
	oauth2EndpointParams   = flagutil.NewArrayString("remoteWrite.oauth2.endpointParams", "Optional OAuth2 endpoint parameters to use for the corresponding -remoteWrite.url . "+
		`The endpoint parameters must be set in JSON format: {"param1":"value1",...,"paramN":"valueN"}`)
	oauth2TokenURL = flagutil.NewArrayString("remoteWrite.oauth2.tokenUrl", "Optional OAuth2 tokenURL to use for the corresponding -remoteWrite.url")
	oauth2Scopes   = flagutil.NewArrayString("remoteWrite.oauth2.scopes", "Optional OAuth2 scopes to use for the corresponding -remoteWrite.url. Scopes must be delimited by ';'")

	awsUseSigv4 = flagutil.NewArrayBool("remoteWrite.aws.useSigv4", "Enables SigV4 request signing for the corresponding -remoteWrite.url. "+
		"It is expected that other -remoteWrite.aws.* command-line flags are set if sigv4 request signing is enabled")
	awsEC2Endpoint = flagutil.NewArrayString("remoteWrite.aws.ec2Endpoint", "Optional AWS EC2 API endpoint to use for the corresponding -remoteWrite.url if -remoteWrite.aws.useSigv4 is set")
	awsSTSEndpoint = flagutil.NewArrayString("remoteWrite.aws.stsEndpoint", "Optional AWS STS API endpoint to use for the corresponding -remoteWrite.url if -remoteWrite.aws.useSigv4 is set")
	awsRegion      = flagutil.NewArrayString("remoteWrite.aws.region", "Optional AWS region to use for the corresponding -remoteWrite.url if -remoteWrite.aws.useSigv4 is set")
	awsRoleARN     = flagutil.NewArrayString("remoteWrite.aws.roleARN", "Optional AWS roleARN to use for the corresponding -remoteWrite.url if -remoteWrite.aws.useSigv4 is set")
	awsAccessKey   = flagutil.NewArrayString("remoteWrite.aws.accessKey", "Optional AWS AccessKey to use for the corresponding -remoteWrite.url if -remoteWrite.aws.useSigv4 is set")
	awsService     = flagutil.NewArrayString("remoteWrite.aws.service", "Optional AWS Service to use for the corresponding -remoteWrite.url if -remoteWrite.aws.useSigv4 is set. "+
		"Defaults to \"aps\"")
	awsSecretKey = flagutil.NewArrayString("remoteWrite.aws.secretKey", "Optional AWS SecretKey to use for the corresponding -remoteWrite.url if -remoteWrite.aws.useSigv4 is set")
)

type client struct {
	sanitizedURL   string
	remoteWriteURL string

	// Whether to use VictoriaMetrics remote write protocol for sending the data to remoteWriteURL
	useVMProto bool

	fq *persistentqueue.FastQueue
	hc *http.Client

	sendBlock func(block []byte) bool
	authCfg   *promauth.Config
	awsCfg    *awsapi.Config

	rl rateLimiter

	bytesSent       *metrics.Counter
	blocksSent      *metrics.Counter
	requestDuration *metrics.Histogram
	requestsOKCount *metrics.Counter
	errorsCount     *metrics.Counter
	packetsDropped  *metrics.Counter
	rateLimit       *metrics.Gauge
	retriesCount    *metrics.Counter
	sendDuration    *metrics.FloatCounter

	wg     sync.WaitGroup
	stopCh chan struct{}
}

func newHTTPClient(argIdx int, remoteWriteURL, sanitizedURL string, fq *persistentqueue.FastQueue, concurrency int) *client {
	authCfg, err := getAuthConfig(argIdx)
	if err != nil {
		logger.Fatalf("cannot initialize auth config for -remoteWrite.url=%q: %s", remoteWriteURL, err)
	}
	tlsCfg, err := authCfg.NewTLSConfig()
	if err != nil {
		logger.Fatalf("cannot initialize tls config for -remoteWrite.url=%q: %s", remoteWriteURL, err)
	}
	awsCfg, err := getAWSAPIConfig(argIdx)
	if err != nil {
		logger.Fatalf("cannot initialize AWS Config for -remoteWrite.url=%q: %s", remoteWriteURL, err)
	}
	tr := &http.Transport{
		DialContext:         statDial,
		TLSClientConfig:     tlsCfg,
		TLSHandshakeTimeout: 10 * time.Second,
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
	hc := &http.Client{
		Transport: tr,
		Timeout:   sendTimeout.GetOptionalArg(argIdx),
	}
	c := &client{
		sanitizedURL:   sanitizedURL,
		remoteWriteURL: remoteWriteURL,
		authCfg:        authCfg,
		awsCfg:         awsCfg,
		fq:             fq,
		hc:             hc,
		stopCh:         make(chan struct{}),
	}
	c.sendBlock = c.sendBlockHTTP

	useVMProto := forceVMProto.GetOptionalArg(argIdx)
	usePromProto := forcePromProto.GetOptionalArg(argIdx)
	if useVMProto && usePromProto {
		logger.Fatalf("-remoteWrite.useVMProto and -remoteWrite.usePromProto cannot be set simultaneously for -remoteWrite.url=%s", sanitizedURL)
	}
	if !useVMProto && !usePromProto {
		// Auto-detect whether the remote storage supports VictoriaMetrics remote write protocol.
		doRequest := func(url string) (*http.Response, error) {
			return c.doRequest(url, nil)
		}
		useVMProto = common.HandleVMProtoClientHandshake(c.remoteWriteURL, doRequest)
		if !useVMProto {
			logger.Infof("the remote storage at %q doesn't support VictoriaMetrics remote write protocol. Switching to Prometheus remote write protocol. "+
				"See https://docs.victoriametrics.com/vmagent.html#victoriametrics-remote-write-protocol", sanitizedURL)
		}
	}
	c.useVMProto = useVMProto

	return c
}

func (c *client) init(argIdx, concurrency int, sanitizedURL string) {
	if bytesPerSec := rateLimit.GetOptionalArg(argIdx); bytesPerSec > 0 {
		logger.Infof("applying %d bytes per second rate limit for -remoteWrite.url=%q", bytesPerSec, sanitizedURL)
		c.rl.perSecondLimit = int64(bytesPerSec)
	}
	c.rl.limitReached = metrics.GetOrCreateCounter(fmt.Sprintf(`vmagent_remotewrite_rate_limit_reached_total{url=%q}`, c.sanitizedURL))

	c.bytesSent = metrics.GetOrCreateCounter(fmt.Sprintf(`vmagent_remotewrite_bytes_sent_total{url=%q}`, c.sanitizedURL))
	c.blocksSent = metrics.GetOrCreateCounter(fmt.Sprintf(`vmagent_remotewrite_blocks_sent_total{url=%q}`, c.sanitizedURL))
	c.rateLimit = metrics.GetOrCreateGauge(fmt.Sprintf(`vmagent_remotewrite_rate_limit{url=%q}`, c.sanitizedURL), func() float64 {
		return float64(rateLimit.GetOptionalArg(argIdx))
	})
	c.requestDuration = metrics.GetOrCreateHistogram(fmt.Sprintf(`vmagent_remotewrite_duration_seconds{url=%q}`, c.sanitizedURL))
	c.requestsOKCount = metrics.GetOrCreateCounter(fmt.Sprintf(`vmagent_remotewrite_requests_total{url=%q, status_code="2XX"}`, c.sanitizedURL))
	c.errorsCount = metrics.GetOrCreateCounter(fmt.Sprintf(`vmagent_remotewrite_errors_total{url=%q}`, c.sanitizedURL))
	c.packetsDropped = metrics.GetOrCreateCounter(fmt.Sprintf(`vmagent_remotewrite_packets_dropped_total{url=%q}`, c.sanitizedURL))
	c.retriesCount = metrics.GetOrCreateCounter(fmt.Sprintf(`vmagent_remotewrite_retries_count_total{url=%q}`, c.sanitizedURL))
	c.sendDuration = metrics.GetOrCreateFloatCounter(fmt.Sprintf(`vmagent_remotewrite_send_duration_seconds_total{url=%q}`, c.sanitizedURL))
	metrics.GetOrCreateGauge(fmt.Sprintf(`vmagent_remotewrite_queues{url=%q}`, c.sanitizedURL), func() float64 {
		return float64(*queues)
	})
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
	headersValue := headers.GetOptionalArg(argIdx)
	var hdrs []string
	if headersValue != "" {
		hdrs = strings.Split(headersValue, "^^")
	}
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
		endpointParamsJSON := oauth2EndpointParams.GetOptionalArg(argIdx)
		endpointParams, err := flagutil.ParseJSONMap(endpointParamsJSON)
		if err != nil {
			return nil, fmt.Errorf("cannot parse JSON for -remoteWrite.oauth2.endpointParams=%s: %w", endpointParamsJSON, err)
		}
		oauth2Cfg = &promauth.OAuth2Config{
			ClientID:         oauth2ClientID.GetOptionalArg(argIdx),
			ClientSecret:     promauth.NewSecret(clientSecret),
			ClientSecretFile: clientSecretFile,
			EndpointParams:   endpointParams,
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

	opts := &promauth.Options{
		BasicAuth:       basicAuthCfg,
		BearerToken:     token,
		BearerTokenFile: tokenFile,
		OAuth2:          oauth2Cfg,
		TLSConfig:       tlsCfg,
		Headers:         hdrs,
	}
	authCfg, err := opts.NewConfig()
	if err != nil {
		return nil, fmt.Errorf("cannot populate auth config for remoteWrite idx: %d, err: %w", argIdx, err)
	}
	return authCfg, nil
}

func getAWSAPIConfig(argIdx int) (*awsapi.Config, error) {
	if !awsUseSigv4.GetOptionalArg(argIdx) {
		return nil, nil
	}
	ec2Endpoint := awsEC2Endpoint.GetOptionalArg(argIdx)
	stsEndpoint := awsSTSEndpoint.GetOptionalArg(argIdx)
	region := awsRegion.GetOptionalArg(argIdx)
	roleARN := awsRoleARN.GetOptionalArg(argIdx)
	accessKey := awsAccessKey.GetOptionalArg(argIdx)
	secretKey := awsSecretKey.GetOptionalArg(argIdx)
	service := awsService.GetOptionalArg(argIdx)
	cfg, err := awsapi.NewConfig(ec2Endpoint, stsEndpoint, region, roleARN, accessKey, secretKey, service)
	if err != nil {
		return nil, err
	}
	return cfg, nil
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
			c.fq.MustWriteBlockIgnoreDisabledPQ(block)
			return
		case <-c.stopCh:
			// c must be stopped. Wait for a while in the hope the block will be sent.
			graceDuration := 5 * time.Second
			select {
			case ok := <-ch:
				if !ok {
					// Return unsent block to the queue.
					c.fq.MustWriteBlockIgnoreDisabledPQ(block)
				}
			case <-time.After(graceDuration):
				// Return unsent block to the queue.
				c.fq.MustWriteBlockIgnoreDisabledPQ(block)
			}
			return
		}
	}
}

func (c *client) doRequest(url string, body []byte) (*http.Response, error) {
	req, err := c.newRequest(url, body)
	if err != nil {
		return nil, err
	}
	resp, err := c.hc.Do(req)
	if err == nil {
		return resp, nil
	}
	if !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		return nil, err
	}
	// It is likely connection became stale or timed out during the first request.
	// Make another attempt in hope request will succeed.
	// If not, the error should be handled by the caller as usual.
	// This should help with https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4139
	req, err = c.newRequest(url, body)
	if err != nil {
		return nil, fmt.Errorf("second attempt: %w", err)
	}
	resp, err = c.hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("second attempt: %w", err)
	}
	return resp, nil
}

func (c *client) newRequest(url string, body []byte) (*http.Request, error) {
	reqBody := bytes.NewBuffer(body)
	req, err := http.NewRequest(http.MethodPost, url, reqBody)
	if err != nil {
		logger.Panicf("BUG: unexpected error from http.NewRequest(%q): %s", url, err)
	}
	err = c.authCfg.SetHeaders(req, true)
	if err != nil {
		return nil, err
	}
	h := req.Header
	h.Set("User-Agent", "vmagent")
	h.Set("Content-Type", "application/x-protobuf")
	if c.useVMProto {
		h.Set("Content-Encoding", "zstd")
		h.Set("X-VictoriaMetrics-Remote-Write-Version", "1")
	} else {
		h.Set("Content-Encoding", "snappy")
		h.Set("X-Prometheus-Remote-Write-Version", "0.1.0")
	}
	if c.awsCfg != nil {
		sigv4Hash := awsapi.HashHex(body)
		if err := c.awsCfg.SignRequest(req, sigv4Hash); err != nil {
			return nil, fmt.Errorf("cannot sign remoteWrite request with AWS sigv4: %w", err)
		}
	}
	return req, nil
}

// sendBlockHTTP sends the given block to c.remoteWriteURL.
//
// The function returns false only if c.stopCh is closed.
// Otherwise it tries sending the block to remote storage indefinitely.
func (c *client) sendBlockHTTP(block []byte) bool {
	c.rl.register(len(block), c.stopCh)
	retryDuration := time.Second
	retriesCount := 0

again:
	startTime := time.Now()
	resp, err := c.doRequest(c.remoteWriteURL, block)
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
		c.bytesSent.Add(len(block))
		c.blocksSent.Inc()
		return true
	}
	metrics.GetOrCreateCounter(fmt.Sprintf(`vmagent_remotewrite_requests_total{url=%q, status_code="%d"}`, c.sanitizedURL, statusCode)).Inc()
	if statusCode == 409 || statusCode == 400 {
		body, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			remoteWriteRejectedLogger.Errorf("sending a block with size %d bytes to %q was rejected (skipping the block): status code %d; "+
				"failed to read response body: %s",
				len(block), c.sanitizedURL, statusCode, err)
		} else {
			remoteWriteRejectedLogger.Errorf("sending a block with size %d bytes to %q was rejected (skipping the block): status code %d; response body: %s",
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
	body, err := io.ReadAll(resp.Body)
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

var remoteWriteRejectedLogger = logger.WithThrottler("remoteWriteRejected", 5*time.Second)

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
