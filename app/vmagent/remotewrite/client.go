package remotewrite

import (
	"crypto/tls"
	"encoding/base64"
	"flag"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/persistentqueue"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
	"github.com/VictoriaMetrics/fasthttp"
	"github.com/VictoriaMetrics/metrics"
)

var (
	sendTimeout = flag.Duration("remoteWrite.sendTimeout", time.Minute, "Timeout for sending a single block of data to -remoteWrite.url")

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
	urlLabelValue  string
	remoteWriteURL string
	host           string
	requestURI     string
	authHeader     string
	fq             *persistentqueue.FastQueue
	hc             *fasthttp.HostClient

	requestDuration *metrics.Histogram
	requestsOKCount *metrics.Counter
	errorsCount     *metrics.Counter
	retriesCount    *metrics.Counter

	wg     sync.WaitGroup
	stopCh chan struct{}
}

func newClient(argIdx int, remoteWriteURL, urlLabelValue string, fq *persistentqueue.FastQueue, concurrency int) *client {
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

	readTimeout := *sendTimeout
	if readTimeout <= 0 {
		readTimeout = time.Minute
	}
	writeTimeout := readTimeout
	var u fasthttp.URI
	u.Update(remoteWriteURL)
	scheme := string(u.Scheme())
	switch scheme {
	case "http", "https":
	default:
		logger.Fatalf("unsupported scheme in -remoteWrite.url=%q: %q. It must be http or https", remoteWriteURL, scheme)
	}
	host := string(u.Host())
	if len(host) == 0 {
		logger.Fatalf("invalid -remoteWrite.url=%q: host cannot be empty. Make sure the url looks like `http://host:port/path`", remoteWriteURL)
	}
	requestURI := string(u.RequestURI())
	isTLS := scheme == "https"
	var tlsCfg *tls.Config
	if isTLS {
		var err error
		tlsCfg, err = getTLSConfig(argIdx)
		if err != nil {
			logger.Panicf("FATAL: cannot initialize TLS config: %s", err)
		}
	}
	if !strings.Contains(host, ":") {
		if isTLS {
			host += ":443"
		} else {
			host += ":80"
		}
	}
	maxConns := 2 * concurrency
	hc := &fasthttp.HostClient{
		Addr:                host,
		Name:                "vmagent",
		Dial:                statDial,
		IsTLS:               isTLS,
		TLSConfig:           tlsCfg,
		MaxConns:            maxConns,
		MaxIdleConnDuration: 10 * readTimeout,
		ReadTimeout:         readTimeout,
		WriteTimeout:        writeTimeout,
		MaxResponseBodySize: 1024 * 1024,
	}
	c := &client{
		urlLabelValue:  urlLabelValue,
		remoteWriteURL: remoteWriteURL,
		host:           host,
		requestURI:     requestURI,
		authHeader:     authHeader,
		fq:             fq,
		hc:             hc,
		stopCh:         make(chan struct{}),
	}
	c.requestDuration = metrics.GetOrCreateHistogram(fmt.Sprintf(`vmagent_remotewrite_duration_seconds{url=%q}`, c.urlLabelValue))
	c.requestsOKCount = metrics.GetOrCreateCounter(fmt.Sprintf(`vmagent_remotewrite_requests_total{url=%q, status_code="2XX"}`, c.urlLabelValue))
	c.errorsCount = metrics.GetOrCreateCounter(fmt.Sprintf(`vmagent_remotewrite_errors_total{url=%q}`, c.urlLabelValue))
	c.retriesCount = metrics.GetOrCreateCounter(fmt.Sprintf(`vmagent_remotewrite_retries_count_total{url=%q}`, c.urlLabelValue))
	for i := 0; i < concurrency; i++ {
		c.wg.Add(1)
		go func() {
			defer c.wg.Done()
			c.runWorker()
		}()
	}
	logger.Infof("initialized client for -remoteWrite.url=%q", c.remoteWriteURL)
	return c
}

func (c *client) MustStop() {
	close(c.stopCh)
	c.wg.Wait()
	logger.Infof("stopped client for -remoteWrite.url=%q", c.remoteWriteURL)
}

func getTLSConfig(argIdx int) (*tls.Config, error) {
	tlsConfig := &promauth.TLSConfig{
		CAFile:             tlsCAFile.GetOptionalArg(argIdx),
		CertFile:           tlsCertFile.GetOptionalArg(argIdx),
		KeyFile:            tlsKeyFile.GetOptionalArg(argIdx),
		ServerName:         tlsServerName.GetOptionalArg(argIdx),
		InsecureSkipVerify: *tlsInsecureSkipVerify,
	}
	cfg, err := promauth.NewConfig(".", nil, "", "", tlsConfig)
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
					len(block), c.remoteWriteURL, graceDuration.Seconds())
			}
			return
		}
	}
}

func (c *client) sendBlock(block []byte) {
	req := fasthttp.AcquireRequest()
	req.SetRequestURI(c.requestURI)
	req.SetHost(c.host)
	req.Header.SetMethod("POST")
	req.Header.Add("Content-Type", "application/x-protobuf")
	req.Header.Add("Content-Encoding", "snappy")
	req.Header.Add("X-Prometheus-Remote-Write-Version", "0.1.0")
	if c.authHeader != "" {
		req.Header.Set("Authorization", c.authHeader)
	}
	req.SetBody(block)

	retryDuration := time.Second
	resp := fasthttp.AcquireResponse()

again:
	select {
	case <-c.stopCh:
		fasthttp.ReleaseRequest(req)
		fasthttp.ReleaseResponse(resp)
		return
	default:
	}

	startTime := time.Now()
	err := doRequestWithPossibleRetry(c.hc, req, resp)
	c.requestDuration.UpdateDuration(startTime)
	if err != nil {
		c.errorsCount.Inc()
		retryDuration *= 2
		if retryDuration > time.Minute {
			retryDuration = time.Minute
		}
		logger.Errorf("couldn't send a block with size %d bytes to %q: %s; re-sending the block in %.3f seconds",
			len(block), c.remoteWriteURL, err, retryDuration.Seconds())
		time.Sleep(retryDuration)
		c.retriesCount.Inc()
		goto again
	}
	statusCode := resp.StatusCode()
	if statusCode/100 != 2 {
		metrics.GetOrCreateCounter(fmt.Sprintf(`vmagent_remotewrite_requests_total{url=%q, status_code="%d"}`, c.urlLabelValue, statusCode)).Inc()
		retryDuration *= 2
		if retryDuration > time.Minute {
			retryDuration = time.Minute
		}
		logger.Errorf("unexpected status code received after sending a block with size %d bytes to %q: %d; response body=%q; re-sending the block in %.3f seconds",
			len(block), c.remoteWriteURL, statusCode, resp.Body(), retryDuration.Seconds())
		time.Sleep(retryDuration)
		c.retriesCount.Inc()
		goto again
	}
	c.requestsOKCount.Inc()

	// The block has been successfully sent to the remote storage.
	fasthttp.ReleaseResponse(resp)
	fasthttp.ReleaseRequest(req)
}

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
