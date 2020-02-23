package remotewrite

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"flag"
	"fmt"
	"io/ioutil"
	"strings"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/netutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/persistentqueue"
	"github.com/VictoriaMetrics/metrics"
	"github.com/valyala/fasthttp"
)

var (
	sendTimeout = flag.Duration("remoteWrite.sendTimeout", time.Minute, "Timeout for sending a single block of data to -remoteWrite.url")

	tlsInsecureSkipVerify = flag.Bool("remoteWrite.tlsInsecureSkipVerify", false, "Whether to skip tls verification when connecting to -remoteWrite.url")
	tlsCertFile           = flag.String("remoteWrite.tlsCertFile", "", "Optional path to client-side TLS certificate file to use when connecting to -remoteWrite.url")
	tlsKeyFile            = flag.String("remoteWrite.tlsKeyFile", "", "Optional path to client-side TLS certificate key to use when connecting to -remoteWrite.url")
	tlsCAFile             = flag.String("remoteWrite.tlsCAFile", "", "Optional path to TLS CA file to use for verifying connections to -remoteWrite.url. "+
		"By default system CA is used")

	basicAuthUsername = flag.String("remoteWrite.basicAuth.username", "", "Optional basic auth username to use for -remoteWrite.url")
	basicAuthPassword = flag.String("remoteWrite.basicAuth.password", "", "Optional basic auth password to use for -remoteWrite.url")
	bearerToken       = flag.String("remoteWrite.bearerToken", "", "Optional bearer auth token to use for -remoteWrite.url")
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

func newClient(remoteWriteURL, urlLabelValue string, fq *persistentqueue.FastQueue) *client {
	authHeader := ""
	if len(*basicAuthUsername) > 0 || len(*basicAuthPassword) > 0 {
		// See https://en.wikipedia.org/wiki/Basic_access_authentication
		token := *basicAuthUsername + ":" + *basicAuthPassword
		token64 := base64.StdEncoding.EncodeToString([]byte(token))
		authHeader = "Basic " + token64
	}
	if len(*bearerToken) > 0 {
		if authHeader != "" {
			logger.Panicf("FATAL: `-remoteWrite.bearerToken`=%q cannot be set when `-remoteWrite.basicAuth.*` flags are set", *bearerToken)
		}
		authHeader = "Bearer " + *bearerToken
	}

	readTimeout := *sendTimeout
	if readTimeout <= 0 {
		readTimeout = time.Minute
	}
	var u fasthttp.URI
	u.Update(remoteWriteURL)
	scheme := string(u.Scheme())
	switch scheme {
	case "http", "https":
	default:
		logger.Panicf("FATAL: unsupported scheme in -remoteWrite.url=%q: %q. It must be http or https", remoteWriteURL, scheme)
	}
	host := string(u.Host())
	if len(host) == 0 {
		logger.Panicf("FATAL: invalid -remoteWrite.url=%q: host cannot be empty. Make sure the url looks like `http://host:port/path`", remoteWriteURL)
	}
	requestURI := string(u.RequestURI())
	isTLS := scheme == "https"
	var tlsCfg *tls.Config
	if isTLS {
		var err error
		tlsCfg, err = getTLSConfig()
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
	maxConns := 2 * *queues
	hc := &fasthttp.HostClient{
		Addr:                host,
		Name:                "vmagent",
		Dial:                statDial,
		DialDualStack:       netutil.TCP6Enabled(),
		IsTLS:               isTLS,
		TLSConfig:           tlsCfg,
		MaxConns:            maxConns,
		MaxIdleConnDuration: 10 * readTimeout,
		ReadTimeout:         readTimeout,
		WriteTimeout:        10 * time.Second,
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
	c.requestDuration = metrics.NewHistogram(fmt.Sprintf(`vmagent_remotewrite_duration_seconds{url=%q}`, c.urlLabelValue))
	c.requestsOKCount = metrics.NewCounter(fmt.Sprintf(`vmagent_remotewrite_requests_total{url=%q, status_code="2XX"}`, c.urlLabelValue))
	c.errorsCount = metrics.NewCounter(fmt.Sprintf(`vmagent_remotewrite_errors_total{url=%q}`, c.urlLabelValue))
	c.retriesCount = metrics.NewCounter(fmt.Sprintf(`vmagent_remotewrite_retries_count_total{url=%q}`, c.urlLabelValue))
	for i := 0; i < *queues; i++ {
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

func getTLSConfig() (*tls.Config, error) {
	var tlsRootCA *x509.CertPool
	var tlsCertificate *tls.Certificate
	if *tlsCertFile != "" || *tlsKeyFile != "" {
		cert, err := tls.LoadX509KeyPair(*tlsCertFile, *tlsKeyFile)
		if err != nil {
			return nil, fmt.Errorf("cannot load TLS certificate for -remoteWrite.tlsCertFile=%q and -remoteWrite.tlsKeyFile=%q: %s", *tlsCertFile, *tlsKeyFile, err)
		}
		tlsCertificate = &cert
	}
	if *tlsCAFile != "" {
		data, err := ioutil.ReadFile(*tlsCAFile)
		if err != nil {
			return nil, fmt.Errorf("cannot read -remoteWrite.tlsCAFile=%q: %s", *tlsCAFile, err)
		}
		tlsRootCA = x509.NewCertPool()
		if !tlsRootCA.AppendCertsFromPEM(data) {
			return nil, fmt.Errorf("cannot parse data -remoteWrite.tlsCAFile=%q", *tlsCAFile)
		}
	}
	tlsCfg := &tls.Config{
		RootCAs:            tlsRootCA,
		ClientSessionCache: tls.NewLRUClientSessionCache(0),
	}
	if tlsCertificate != nil {
		tlsCfg.Certificates = []tls.Certificate{*tlsCertificate}
	}
	tlsCfg.InsecureSkipVerify = *tlsInsecureSkipVerify
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
	// There is no need in calling DoTimeout, since the timeout is set in c.hc.ReadTimeout.
	err := c.hc.Do(req, resp)
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
