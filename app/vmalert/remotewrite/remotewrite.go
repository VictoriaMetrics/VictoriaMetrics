package remotewrite

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/auth"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	"github.com/VictoriaMetrics/metrics"
	"github.com/golang/snappy"
)

// Client is an asynchronous HTTP client for writing
// timeseries via remote write protocol.
type Client struct {
	URL     string
	baseURL string
	suffix  string

	c              *http.Client
	tswCh          chan TimeseriesWrapper
	baUser, baPass string
	flushInterval  time.Duration
	maxBatchSize   int
	maxQueueSize   int

	wg     sync.WaitGroup
	doneCh chan struct{}
}

type TimeseriesWrapper struct {
	at string
	ts prompbmarshal.TimeSeries
}

type WriteRequestWrapper struct {
	m       map[string]*prompbmarshal.WriteRequest
	tsCount int
}

func (w *WriteRequestWrapper) AddTimeseries(tsw TimeseriesWrapper) {
	tss, ok := w.m[tsw.at]
	if !ok {
		tss = &prompbmarshal.WriteRequest{}
	}
	tss.Timeseries = append(tss.Timeseries, tsw.ts)
	w.m[tsw.at] = tss
	w.tsCount += 1
}

func (w *WriteRequestWrapper) Reset() {
	for _, wr := range w.m {
		prompbmarshal.ResetWriteRequest(wr)
	}
	w.tsCount = 0
}

// Config is config for remote write.
type Config struct {
	URL     string
	BaseURL string
	Suffix  string

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
	defaultFlushInterval = 5 * time.Second
	defaultWriteTimeout  = 30 * time.Second
)

const writePath = "/api/v1/write"

// NewClient returns asynchronous client for
// writing timeseries via remotewrite protocol.
func NewClient(ctx context.Context, cfg Config) (*Client, error) {
	if cfg.BaseURL == "" && cfg.URL == "" {
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
	c := &Client{
		c: &http.Client{
			Timeout:   cfg.WriteTimeout,
			Transport: cfg.Transport,
		},
		baseURL:       cfg.BaseURL,
		suffix:        cfg.Suffix,
		baUser:        cfg.BasicAuthUser,
		baPass:        cfg.BasicAuthPass,
		flushInterval: cfg.FlushInterval,
		maxBatchSize:  cfg.MaxBatchSize,
		maxQueueSize:  cfg.MaxQueueSize,
		doneCh:        make(chan struct{}),
		tswCh:         make(chan TimeseriesWrapper, cfg.MaxQueueSize),
	}
	if cfg.URL != "" {
		c.URL = cfg.URL
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
func (c *Client) Push(at *auth.Token, s prompbmarshal.TimeSeries) error {
	select {
	case <-c.doneCh:
		return fmt.Errorf("client is closed")
	case c.tswCh <- TimeseriesWrapper{at.String(), s}:
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
	close(c.tswCh)
	close(c.doneCh)
	c.wg.Wait()
	return nil
}

func (c *Client) run(ctx context.Context) {
	ticker := time.NewTicker(c.flushInterval)
	wrw := &WriteRequestWrapper{
		m: make(map[string]*prompbmarshal.WriteRequest),
	}
	shutdown := func() {
		for tsw := range c.tswCh {
			wrw.AddTimeseries(tsw)
		}
		lastCtx, cancel := context.WithTimeout(context.Background(), defaultWriteTimeout)
		c.flush(lastCtx, wrw)
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
				c.flush(ctx, wrw)
			case tsw, ok := <-c.tswCh:
				if !ok {
					continue
				}
				wrw.AddTimeseries(tsw)
				if wrw.tsCount >= c.maxBatchSize {
					c.flush(ctx, wrw)
				}
			}
		}
	}()
}

var (
	sentRows     = metrics.NewCounter(`vmalert_remotewrite_sent_rows_total`)
	sentBytes    = metrics.NewCounter(`vmalert_remotewrite_sent_bytes_total`)
	droppedRows  = metrics.NewCounter(`vmalert_remotewrite_dropped_rows_total`)
	droppedBytes = metrics.NewCounter(`vmalert_remotewrite_dropped_bytes_total`)
)

// flush is a blocking function that marshals WriteRequest and sends
// it to remote write endpoint. Flush performs limited amount of retries
// if request fails.
func (c *Client) flush(ctx context.Context, wrw *WriteRequestWrapper) {
	if wrw.tsCount < 1 {
		return
	}
	defer wrw.Reset()

	for at, wr := range wrw.m {
		data, err := wr.Marshal()
		if err != nil {
			logger.Errorf("failed to marshal WriteRequest: %s", err)
			return
		}

		const attempts = 5
		b := snappy.Encode(nil, data)
		for i := 0; i < attempts; i++ {
			err := c.send(ctx, at, b)
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
}

func (c *Client) send(ctx context.Context, at string, data []byte) error {
	r := bytes.NewReader(data)
	var addr string
	if c.URL == "" {
		addr = fmt.Sprintf("%v/%s/%s%s", c.baseURL, at, c.suffix, writePath)
	} else {
		addr = fmt.Sprintf("%v%s", c.URL, writePath)
	}

	req, err := http.NewRequest("POST", addr, r)
	if err != nil {
		return fmt.Errorf("failed to create new HTTP request: %w", err)
	}
	if c.baPass != "" {
		req.SetBasicAuth(c.baUser, c.baPass)
	}
	resp, err := c.c.Do(req.WithContext(ctx))
	if err != nil {
		return fmt.Errorf("error while sending request to %s: %w; Data len %d(%d)",
			req.URL, err, len(data), r.Size())
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusNoContent {
		body, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("unexpected response code %d for %s. Response body %q",
			resp.StatusCode, req.URL, body)
	}
	return nil
}
