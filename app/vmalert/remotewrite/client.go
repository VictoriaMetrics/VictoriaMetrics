package remotewrite

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/golang/snappy"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/netutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	"github.com/VictoriaMetrics/metrics"
)

const (
	defaultConcurrency   = 4
	defaultMaxBatchSize  = 1e3
	defaultMaxQueueSize  = 1e5
	defaultFlushInterval = 5 * time.Second
	defaultWriteTimeout  = 30 * time.Second
)

var (
	disablePathAppend = flag.Bool("remoteWrite.disablePathAppend", false, "Whether to disable automatic appending of '/api/v1/write' path to the configured -remoteWrite.url.")
	sendTimeout       = flag.Duration("remoteWrite.sendTimeout", 30*time.Second, "Timeout for sending data to the configured -remoteWrite.url.")
	retryMinInterval  = flag.Duration("remoteWrite.retryMinInterval", time.Second, "The minimum delay between retry attempts. Every next retry attempt will double the delay to prevent hammering of remote database. See also -remoteWrite.retryMaxTime")
	retryMaxTime      = flag.Duration("remoteWrite.retryMaxTime", time.Second*30, "The max time spent on retry attempts for the failed remote-write request. Change this value if it is expected for remoteWrite.url to be unreachable for more than -remoteWrite.retryMaxTime. See also -remoteWrite.retryMinInterval")
)

// Client is an asynchronous HTTP client for writing
// timeseries via remote write protocol.
type Client struct {
	addr          string
	c             *http.Client
	authCfg       *promauth.Config
	input         chan prompbmarshal.TimeSeries
	flushInterval time.Duration
	maxBatchSize  int
	maxQueueSize  int

	wg     sync.WaitGroup
	doneCh chan struct{}
}

// Config is config for remote write client.
type Config struct {
	// Addr of remote storage
	Addr    string
	AuthCfg *promauth.Config

	// Concurrency defines number of readers that
	// concurrently read from the queue and flush data
	Concurrency int
	// MaxBatchSize defines max number of timeseries
	// to be flushed at once
	MaxBatchSize int
	// MaxQueueSize defines max length of input queue
	// populated by Push method.
	// Push will be rejected once queue is full.
	MaxQueueSize int
	// FlushInterval defines time interval for flushing batches
	FlushInterval time.Duration
	// Transport will be used by the underlying http.Client
	Transport *http.Transport
}

// NewClient returns asynchronous client for
// writing timeseries via remotewrite protocol.
func NewClient(ctx context.Context, cfg Config) (*Client, error) {
	if cfg.Addr == "" {
		return nil, fmt.Errorf("config.Addr can't be empty")
	}
	if cfg.MaxBatchSize == 0 {
		cfg.MaxBatchSize = defaultMaxBatchSize
	}
	if cfg.MaxQueueSize == 0 {
		cfg.MaxQueueSize = defaultMaxQueueSize
	}
	if cfg.FlushInterval == 0 {
		cfg.FlushInterval = defaultFlushInterval
	}
	if cfg.Transport == nil {
		cfg.Transport = http.DefaultTransport.(*http.Transport).Clone()
	}
	cc := defaultConcurrency
	if cfg.Concurrency > 0 {
		cc = cfg.Concurrency
	}
	c := &Client{
		c: &http.Client{
			Timeout:   *sendTimeout,
			Transport: cfg.Transport,
		},
		addr:          strings.TrimSuffix(cfg.Addr, "/"),
		authCfg:       cfg.AuthCfg,
		flushInterval: cfg.FlushInterval,
		maxBatchSize:  cfg.MaxBatchSize,
		maxQueueSize:  cfg.MaxQueueSize,
		doneCh:        make(chan struct{}),
		input:         make(chan prompbmarshal.TimeSeries, cfg.MaxQueueSize),
	}

	for i := 0; i < cc; i++ {
		c.run(ctx)
	}
	return c, nil
}

// Push adds timeseries into queue for writing into remote storage.
// Push returns and error if client is stopped or if queue is full.
func (c *Client) Push(s prompbmarshal.TimeSeries) error {
	rwTotal.Inc()
	select {
	case <-c.doneCh:
		rwErrors.Inc()
		droppedRows.Add(len(s.Samples))
		return fmt.Errorf("client is closed")
	case c.input <- s:
		return nil
	default:
		rwErrors.Inc()
		droppedRows.Add(len(s.Samples))
		return fmt.Errorf("failed to push timeseries - queue is full (%d entries). "+
			"Queue size is controlled by -remoteWrite.maxQueueSize flag",
			c.maxQueueSize)
	}
}

// Close stops the client and waits for all goroutines
// to exit.
func (c *Client) Close() error {
	if c.doneCh == nil {
		return fmt.Errorf("client is already closed")
	}
	close(c.input)
	close(c.doneCh)
	c.wg.Wait()
	return nil
}

func (c *Client) run(ctx context.Context) {
	ticker := time.NewTicker(c.flushInterval)
	wr := &prompbmarshal.WriteRequest{}
	shutdown := func() {
		lastCtx, cancel := context.WithTimeout(context.Background(), defaultWriteTimeout)
		logger.Infof("shutting down remote write client and flushing remained series")

		shutdownFlushCnt := 0
		for ts := range c.input {
			wr.Timeseries = append(wr.Timeseries, ts)
			if len(wr.Timeseries) >= c.maxBatchSize {
				shutdownFlushCnt += len(wr.Timeseries)
				c.flush(lastCtx, wr)
			}
		}
		// flush the last batch. `flush` will re-check and avoid flushing empty batch.
		shutdownFlushCnt += len(wr.Timeseries)
		c.flush(lastCtx, wr)

		logger.Infof("shutting down remote write client flushed %d series", shutdownFlushCnt)
		cancel()
	}
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		defer ticker.Stop()
		for {
			select {
			case <-c.doneCh:
				shutdown()
				return
			case <-ctx.Done():
				shutdown()
				return
			case <-ticker.C:
				c.flush(ctx, wr)
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

var (
	rwErrors = metrics.NewCounter(`vmalert_remotewrite_errors_total`)
	rwTotal  = metrics.NewCounter(`vmalert_remotewrite_total`)

	sentRows            = metrics.NewCounter(`vmalert_remotewrite_sent_rows_total`)
	sentBytes           = metrics.NewCounter(`vmalert_remotewrite_sent_bytes_total`)
	droppedRows         = metrics.NewCounter(`vmalert_remotewrite_dropped_rows_total`)
	sendDuration        = metrics.NewFloatCounter(`vmalert_remotewrite_send_duration_seconds_total`)
	bufferFlushDuration = metrics.NewHistogram(`vmalert_remotewrite_flush_duration_seconds`)

	_ = metrics.NewGauge(`vmalert_remotewrite_concurrency`, func() float64 {
		return float64(*concurrency)
	})
)

// GetDroppedRows returns value of droppedRows metric
func GetDroppedRows() int64 { return int64(droppedRows.Get()) }

// flush is a blocking function that marshals WriteRequest and sends
// it to remote-write endpoint. Flush performs limited amount of retries
// if request fails.
func (c *Client) flush(ctx context.Context, wr *prompbmarshal.WriteRequest) {
	if len(wr.Timeseries) < 1 {
		return
	}
	defer wr.Reset()
	defer bufferFlushDuration.UpdateDuration(time.Now())

	data := wr.MarshalProtobuf(nil)
	b := snappy.Encode(nil, data)

	retryInterval, maxRetryInterval := *retryMinInterval, *retryMaxTime
	if retryInterval > maxRetryInterval {
		retryInterval = maxRetryInterval
	}
	timeStart := time.Now()
	defer func() {
		sendDuration.Add(time.Since(timeStart).Seconds())
	}()
L:
	for attempts := 0; ; attempts++ {
		err := c.send(ctx, b)
		if err != nil && (errors.Is(err, io.EOF) || netutil.IsTrivialNetworkError(err)) {
			// Something in the middle between client and destination might be closing
			// the connection. So we do a one more attempt in hope request will succeed.
			err = c.send(ctx, b)
		}
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

	rwErrors.Inc()
	rows := 0
	for _, ts := range wr.Timeseries {
		rows += len(ts.Samples)
	}
	droppedRows.Add(rows)
	logger.Errorf("attempts to send remote-write request failed - dropping %d time series",
		len(wr.Timeseries))
}

func (c *Client) send(ctx context.Context, data []byte) error {
	r := bytes.NewReader(data)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.addr, r)
	if err != nil {
		return fmt.Errorf("failed to create new HTTP request: %w", err)
	}

	// RFC standard compliant headers
	req.Header.Set("Content-Encoding", "snappy")
	req.Header.Set("Content-Type", "application/x-protobuf")

	// Prometheus compliant headers
	req.Header.Set("X-Prometheus-Remote-Write-Version", "0.1.0")

	if c.authCfg != nil {
		err = c.authCfg.SetHeaders(req, true)
		if err != nil {
			return &nonRetriableError{
				err: err,
			}
		}
	}
	if !*disablePathAppend {
		req.URL.Path = path.Join(req.URL.Path, "/api/v1/write")
	}
	resp, err := c.c.Do(req)
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
		// respond with HTTP 2xx status code when write is successful.
		return nil
	case 4:
		if resp.StatusCode != http.StatusTooManyRequests {
			// MUST NOT retry write requests on HTTP 4xx responses other than 429
			return &nonRetriableError{
				err: fmt.Errorf("unexpected response code %d for %s. Response body %q", resp.StatusCode, req.URL.Redacted(), body),
			}
		}
		fallthrough
	default:
		return fmt.Errorf("unexpected response code %d for %s. Response body %q",
			resp.StatusCode, req.URL.Redacted(), body)
	}
}

type nonRetriableError struct {
	err error
}

func (e *nonRetriableError) Error() string {
	return e.err.Error()
}
