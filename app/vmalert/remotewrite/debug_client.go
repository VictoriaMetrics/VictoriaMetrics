package remotewrite

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/golang/snappy"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/utils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
)

// DebugClient won't push series periodically, but have debug flush
type DebugClient struct {
	addr         string
	c            *http.Client
	authCfg      *promauth.Config
	input        chan prompbmarshal.TimeSeries
	maxBatchSize int
	maxQueueSize int

	wg     sync.WaitGroup
	doneCh chan struct{}

	debugflushWG sync.WaitGroup
	debugflushCh chan struct{}
}

// NewDebugClient returns a debug remotewrite client
func NewDebugClient(ctx context.Context) (*DebugClient, error) {
	if *addr == "" {
		return nil, nil
	}

	t, err := utils.Transport(*addr, *tlsCertFile, *tlsKeyFile, *tlsCAFile, *tlsServerName, *tlsInsecureSkipVerify)
	if err != nil {
		return nil, fmt.Errorf("failed to create transport: %w", err)
	}

	authCfg, err := utils.AuthConfig(
		utils.WithBasicAuth(*basicAuthUsername, *basicAuthPassword, *basicAuthPasswordFile),
		utils.WithBearer(*bearerToken, *bearerTokenFile),
		utils.WithOAuth(*oauth2ClientID, *oauth2ClientSecret, *oauth2ClientSecretFile, *oauth2TokenURL, *oauth2Scopes),
		utils.WithHeaders(*headers))
	if err != nil {
		return nil, fmt.Errorf("failed to configure auth: %w", err)
	}
	c := &DebugClient{
		c: &http.Client{
			Timeout:   *sendTimeout,
			Transport: t,
		},
		addr:         strings.TrimSuffix(*addr, "/"),
		authCfg:      authCfg,
		input:        make(chan prompbmarshal.TimeSeries, defaultMaxQueueSize),
		maxBatchSize: *maxQueueSize,
		maxQueueSize: *maxQueueSize,
		doneCh:       make(chan struct{}),

		debugflushCh: make(chan struct{}, 1),
	}
	c.run(ctx)
	return c, nil
}

// Push adds timeseries into queue for writing into remote storage.
// Push returns and error if client is stopped or if queue is full.
func (c *DebugClient) Push(s prompbmarshal.TimeSeries) error {
	select {
	case <-c.doneCh:
		return fmt.Errorf("client is closed")
	case c.input <- s:
		return nil
	default:
		return fmt.Errorf("failed to push timeseries - queue is full (%d entries). "+
			"Queue size is controlled by -remoteWrite.maxQueueSize flag",
			c.maxQueueSize)
	}
}

// Close stops the client and waits for all goroutines
// to exit.
func (c *DebugClient) Close() error {
	if c.doneCh == nil {
		return fmt.Errorf("client is already closed")
	}
	close(c.input)
	close(c.doneCh)
	c.wg.Wait()
	return nil
}

// DebugFlush will flush all the series in wr
func (c *DebugClient) DebugFlush() {
	c.debugflushWG.Add(1)
	c.debugflushCh <- struct{}{}
	c.debugflushWG.Wait()
}

func (c *DebugClient) run(ctx context.Context) {
	// ticker := time.NewTicker(c.flushInterval)
	wr := &prompbmarshal.WriteRequest{}
	shutdown := func() {
		for ts := range c.input {
			wr.Timeseries = append(wr.Timeseries, ts)
		}
		lastCtx, cancel := context.WithTimeout(context.Background(), defaultWriteTimeout)
		logger.Infof("shutting down remote write client and flushing remained %d series", len(wr.Timeseries))
		c.flush(lastCtx, wr)
		cancel()
	}
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		// defer ticker.Stop()
		for {
			select {
			case <-c.doneCh:
				shutdown()
				return
			case <-ctx.Done():
				shutdown()
				return
			case <-c.debugflushCh:
				c.flush(ctx, wr)
				c.debugflushWG.Done()
			case ts, ok := <-c.input:
				if !ok {
					continue
				}
				wr.Timeseries = append(wr.Timeseries, ts)
				if len(wr.Timeseries) >= c.maxBatchSize {
					c.flush(ctx, wr)
				}
			}
		}
	}()
}

// flush is a blocking function that marshals WriteRequest and sends
// it to remote-write endpoint. Flush performs limited amount of retries
// if request fails.
func (c *DebugClient) flush(ctx context.Context, wr *prompbmarshal.WriteRequest) {
	if len(wr.Timeseries) < 1 {
		return
	}
	defer prompbmarshal.ResetWriteRequest(wr)
	defer bufferFlushDuration.UpdateDuration(time.Now())

	data, err := wr.Marshal()
	if err != nil {
		logger.Errorf("failed to marshal WriteRequest: %s", err)
		return
	}

	b := snappy.Encode(nil, data)

	retryInterval, maxRetryInterval := *retryMinInterval, *retryMaxTime
	if retryInterval > maxRetryInterval {
		retryInterval = maxRetryInterval
	}
	timeStart := time.Now()
	defer sendDuration.Add(time.Since(timeStart).Seconds())
L:
	for attempts := 0; ; attempts++ {
		err := c.send(ctx, b)
		if err == nil {
			sentRows.Add(len(wr.Timeseries))
			sentBytes.Add(len(b))
			return
		}

		_, isNotRetriable := err.(*nonRetriableError)
		logger.Warnf("attempt %d to send request failed: %s (retriable: %v)", attempts+1, err, !isNotRetriable)

		if isNotRetriable {
			// exit fast if error isn't retriable
			break
		}

		// check if request has been cancelled before backoff
		select {
		case <-ctx.Done():
			logger.Errorf("interrupting retry attempt %d: context cancelled", attempts+1)
			break L
		default:
		}

		timeLeftForRetries := maxRetryInterval - time.Since(timeStart)
		if timeLeftForRetries < 0 {
			// the max retry time has passed, so we give up
			break
		}

		if retryInterval > timeLeftForRetries {
			retryInterval = timeLeftForRetries
		}
		// sleeping to prevent remote db hammering
		time.Sleep(retryInterval)
		retryInterval *= 2

	}

	droppedRows.Add(len(wr.Timeseries))
	droppedBytes.Add(len(b))
	logger.Errorf("attempts to send remote-write request failed - dropping %d time series",
		len(wr.Timeseries))
}

func (c *DebugClient) send(ctx context.Context, data []byte) error {
	r := bytes.NewReader(data)
	req, err := http.NewRequest(http.MethodPost, c.addr, r)
	if err != nil {
		return fmt.Errorf("failed to create new HTTP request: %w", err)
	}

	// RFC standard compliant headers
	req.Header.Set("Content-Encoding", "snappy")
	req.Header.Set("Content-Type", "application/x-protobuf")

	// Prometheus compliant headers
	req.Header.Set("X-Prometheus-Remote-Write-Version", "0.1.0")

	if c.authCfg != nil {
		c.authCfg.SetHeaders(req, true)
	}
	if !*disablePathAppend {
		req.URL.Path = path.Join(req.URL.Path, "/api/v1/write")
	}
	resp, err := c.c.Do(req.WithContext(ctx))
	if err != nil {
		return fmt.Errorf("error while sending request to %s: %w; Data len %d(%d)",
			req.URL.Redacted(), err, len(data), r.Size())
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)

	// according to https://prometheus.io/docs/concepts/remote_write_spec/
	// Prometheus remote Write compatible receivers MUST
	switch resp.StatusCode / 100 {
	case 2:
		// respond with a HTTP 2xx status code when the write is successful.
		return nil
	case 4:
		if resp.StatusCode != http.StatusTooManyRequests {
			// MUST NOT retry write requests on HTTP 4xx responses other than 429
			return &nonRetriableError{fmt.Errorf("unexpected response code %d for %s. Response body %q",
				resp.StatusCode, req.URL.Redacted(), body)}
		}
		fallthrough
	default:
		return fmt.Errorf("unexpected response code %d for %s. Response body %q",
			resp.StatusCode, req.URL.Redacted(), body)
	}
}
