package remotewrite

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	"github.com/golang/snappy"
)

// Client is an asynchronous HTTP client for writing
// timeseries via remote write protocol.
type Client struct {
	addr           string
	c              *http.Client
	input          chan prompbmarshal.TimeSeries
	baUser, baPass string
	flushInterval  time.Duration
	maxBatchSize   int
	maxQueueSize   int

	wg     sync.WaitGroup
	doneCh chan struct{}
}

// Config is config for remote write.
type Config struct {
	// Addr of remote storage
	Addr string

	BasicAuthUser string
	BasicAuthPass string

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
}

const (
	defaultConcurrency   = 4
	defaultMaxBatchSize  = 1e3
	defaultMaxQueueSize  = 1e5
	defaultFlushInterval = time.Second
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
	c := &Client{
		c: &http.Client{
			Timeout:   cfg.WriteTimeout,
			Transport: cfg.Transport,
		},
		addr:          strings.TrimSuffix(cfg.Addr, "/") + writePath,
		baUser:        cfg.BasicAuthUser,
		baPass:        cfg.BasicAuthPass,
		flushInterval: cfg.FlushInterval,
		maxBatchSize:  cfg.MaxBatchSize,
		doneCh:        make(chan struct{}),
		input:         make(chan prompbmarshal.TimeSeries, cfg.MaxQueueSize),
	}
	cc := defaultConcurrency
	if cfg.Concurrency > 0 {
		cc = cfg.Concurrency
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
	wr := prompbmarshal.WriteRequest{}
	shutdown := func() {
		for ts := range c.input {
			wr.Timeseries = append(wr.Timeseries, ts)
		}
		if len(wr.Timeseries) < 1 {
			return
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
				wr = prompbmarshal.WriteRequest{}
			case ts := <-c.input:
				wr.Timeseries = append(wr.Timeseries, ts)
				if len(wr.Timeseries) >= c.maxBatchSize {
					c.flush(ctx, wr)
					wr = prompbmarshal.WriteRequest{}
				}
			}
		}
	}()
}

func (c *Client) flush(ctx context.Context, wr prompbmarshal.WriteRequest) {
	if len(wr.Timeseries) < 1 {
		return
	}
	data, err := wr.Marshal()
	if err != nil {
		logger.Errorf("failed to marshal WriteRequest: %s", err)
		return
	}
	req, err := http.NewRequest("POST", c.addr, bytes.NewReader(snappy.Encode(nil, data)))
	if err != nil {
		logger.Errorf("failed to create new HTTP request: %s", err)
		return
	}
	if c.baPass != "" {
		req.SetBasicAuth(c.baUser, c.baPass)
	}
	resp, err := c.c.Do(req.WithContext(ctx))
	if err != nil {
		logger.Errorf("error getting response from %s:%s", req.URL, err)
		return
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusNoContent {
		body, _ := ioutil.ReadAll(resp.Body)
		logger.Errorf("unexpected response code %d for %s. Response body %s", resp.StatusCode, req.URL, body)
		return
	}
}
