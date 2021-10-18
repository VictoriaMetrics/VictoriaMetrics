package remotewrite

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/golang/snappy"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promauth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	"github.com/VictoriaMetrics/metrics"
)

// Client is an asynchronous HTTP client for writing
// timeseries via remote write protocol.
type Client struct {
	addr              string
	c                 *http.Client
	authCfg           *promauth.Config
	input             chan prompbmarshal.TimeSeries
	flushInterval     time.Duration
	maxBatchSize      int
	maxQueueSize      int
	disablePathAppend bool

	wg     sync.WaitGroup
	doneCh chan struct{}
}

// Config is config for remote write.
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
	// WriteTimeout defines timeout for HTTP write request
	// to remote storage
	WriteTimeout time.Duration
	// Transport will be used by the underlying http.Client
	Transport *http.Transport
	// DisablePathAppend can be used to not automatically append '/api/v1/write' to the remote write url
	DisablePathAppend bool
}

const (
	defaultConcurrency   = 4
	defaultMaxBatchSize  = 1e3
	defaultMaxQueueSize  = 1e5
	defaultFlushInterval = 5 * time.Second
	defaultWriteTimeout  = 30 * time.Second
)

const writePath = "/api/v1/write"

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
	if cfg.WriteTimeout == 0 {
		cfg.WriteTimeout = defaultWriteTimeout
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
			Timeout:   cfg.WriteTimeout,
			Transport: cfg.Transport,
		},
		addr:              strings.TrimSuffix(cfg.Addr, "/"),
		authCfg:           cfg.AuthCfg,
		flushInterval:     cfg.FlushInterval,
		maxBatchSize:      cfg.MaxBatchSize,
		maxQueueSize:      cfg.MaxQueueSize,
		doneCh:            make(chan struct{}),
		input:             make(chan prompbmarshal.TimeSeries, cfg.MaxQueueSize),
		disablePathAppend: cfg.DisablePathAppend,
	}

	for i := 0; i < cc; i++ {
		c.run(ctx)
	}
	return c, nil
}

// Push adds timeseries into queue for writing into remote storage.
// Push returns and error if client is stopped or if queue is full.
func (c *Client) Push(s prompbmarshal.TimeSeries) error {
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
		for ts := range c.input {
			wr.Timeseries = append(wr.Timeseries, ts)
		}
		lastCtx, cancel := context.WithTimeout(context.Background(), defaultWriteTimeout)
		c.flush(lastCtx, wr)
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
	sentRows            = metrics.NewCounter(`vmalert_remotewrite_sent_rows_total`)
	sentBytes           = metrics.NewCounter(`vmalert_remotewrite_sent_bytes_total`)
	droppedRows         = metrics.NewCounter(`vmalert_remotewrite_dropped_rows_total`)
	droppedBytes        = metrics.NewCounter(`vmalert_remotewrite_dropped_bytes_total`)
	bufferFlushDuration = metrics.NewHistogram(`vmalert_remotewrite_flush_duration_seconds`)
)

// flush is a blocking function that marshals WriteRequest and sends
// it to remote write endpoint. Flush performs limited amount of retries
// if request fails.
func (c *Client) flush(ctx context.Context, wr *prompbmarshal.WriteRequest) {
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

	const attempts = 5
	b := snappy.Encode(nil, data)
	for i := 0; i < attempts; i++ {
		err := c.send(ctx, b)
		if err == nil {
			sentRows.Add(len(wr.Timeseries))
			sentBytes.Add(len(b))
			return
		}

		logger.Errorf("attempt %d to send request failed: %s", i+1, err)
		// sleeping to avoid remote db hammering
		time.Sleep(time.Second)
		continue
	}

	droppedRows.Add(len(wr.Timeseries))
	droppedBytes.Add(len(b))
	logger.Errorf("all %d attempts to send request failed - dropping %d timeseries",
		attempts, len(wr.Timeseries))
}

func (c *Client) send(ctx context.Context, data []byte) error {
	r := bytes.NewReader(data)
	req, err := http.NewRequest("POST", c.addr, r)
	if err != nil {
		return fmt.Errorf("failed to create new HTTP request: %w", err)
	}
	if c.authCfg != nil {
		if auth := c.authCfg.GetAuthHeader(); auth != "" {
			req.Header.Set("Authorization", auth)
		}
	}
	if !c.disablePathAppend {
		req.URL.Path = path.Join(req.URL.Path, writePath)
	}
	resp, err := c.c.Do(req.WithContext(ctx))
	if err != nil {
		return fmt.Errorf("error while sending request to %s: %w; Data len %d(%d)",
			req.URL.Redacted(), err, len(data), r.Size())
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		body, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("unexpected response code %d for %s. Response body %q",
			resp.StatusCode, req.URL.Redacted(), body)
	}
	return nil
}
