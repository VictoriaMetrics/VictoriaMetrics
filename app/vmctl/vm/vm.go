package vm

import (
	"bufio"
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/auth"
	"github.com/VictoriaMetrics/metrics"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/backoff"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/barpool"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmctl/limiter"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/decimal"
)

// Config contains list of params to configure
// the Importer
type Config struct {
	// VictoriaMetrics address to perform import requests
	//   --httpListenAddr value for single node version
	//   --httpListenAddr value of vmselect  component for cluster version
	Addr string

	AuthCfg *auth.Config
	// Transport allows specifying custom http.Transport
	Transport *http.Transport
	// Concurrency defines number of worker
	// performing the import requests concurrently
	Concurrency uint8
	// Whether to apply gzip compression
	Compress bool
	// AccountID for cluster version.
	// Empty value assumes it is a single node version
	AccountID string
	// BatchSize defines how many samples
	// importer collects before sending the import request
	BatchSize int
	// SignificantFigures defines the number of significant figures to leave
	// in metric values before importing.
	// Zero value saves all the significant decimal places
	SignificantFigures int
	// RoundDigits defines the number of decimal digits after the point that must be left
	// in metric values before importing.
	RoundDigits int
	// ExtraLabels that will be added to all imported series. Must be in label=value format.
	ExtraLabels []string
	// RateLimit defines a data transfer speed in bytes per second.
	// Is applied to each worker (see Concurrency) independently.
	RateLimit int64
	// Backoff defines backoff policy for retries
	Backoff *backoff.Backoff
}

// Importer performs insertion of timeseries
// via VictoriaMetrics import protocol
// see https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#how-to-import-time-series-data
type Importer struct {
	addr        string
	authCfg     *auth.Config
	client      *http.Client
	importPath  string
	accountID   string
	extraLabels []string
	compress    bool

	close  chan struct{}
	input  chan importItem
	errors chan *ImportError

	flushWorkerChs []chan chan struct{}
	flushMu        sync.Mutex

	rl *limiter.Limiter

	wg    sync.WaitGroup
	errWg sync.WaitGroup
	once  sync.Once

	s                  *stats
	backoff            *backoff.Backoff
	significantFigures int
	roundDigits        int

	importRequestsTotal       *metrics.Counter
	importRequestsErrorsTotal *metrics.Counter
	importSamplesTotal        *metrics.Counter
	importBytesTotal          *metrics.Counter
	importDuration            *metrics.Histogram
}

func (im *Importer) Addr() string          { return im.addr }
func (im *Importer) AccountID() string     { return im.accountID }
func (im *Importer) ExtraLabels() []string { return im.extraLabels }

type importItem struct {
	ts    *TimeSeries
	block *blockAck
}

type blockAck struct {
	mu      sync.Mutex
	pending int
	closed  bool
	err     error
	done    chan struct{}
	once    sync.Once
	wg      sync.WaitGroup // Tracks active submissions
}

func (im *Importer) NewBlockAck() *blockAck { return &blockAck{done: make(chan struct{})} }

func (b *blockAck) add() error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return errors.New("cannot add to a closed blockAck")
	}
	b.pending++
	b.wg.Add(1)
	return nil
}

func (b *blockAck) finish(err error) {
	b.mu.Lock()
	if err != nil && b.err == nil {
		b.err = err
	}
	if b.pending <= 0 {
		b.mu.Unlock()
		panic("blockAck: negative pending count")
	}
	b.pending--
	closeNow := b.closed && b.pending == 0
	b.mu.Unlock()
	if closeNow {
		b.once.Do(func() { close(b.done) })
	}
}

func (b *blockAck) seal() {
	b.mu.Lock()
	b.closed = true
	b.mu.Unlock()
	// Wait for any in-flight InputBlock items to finish queueing into the channel
	b.wg.Wait()
}

func (b *blockAck) wait() error {
	b.mu.Lock()
	closeNow := b.pending == 0
	b.mu.Unlock()
	if closeNow {
		b.once.Do(func() { close(b.done) })
	}
	<-b.done
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.err
}

func (im *Importer) Flush() {
	im.flushMu.Lock()
	defer im.flushMu.Unlock()

	select {
	case <-im.close:
		return
	default:
	}

	var wg sync.WaitGroup
	for _, ch := range im.flushWorkerChs {
		wg.Add(1)
		go func(c chan chan struct{}) {
			defer wg.Done()
			done := make(chan struct{})
			select {
			case c <- done:
				select {
				case <-done:
				case <-im.close:
				}
			case <-im.close:
			}
		}(ch)
	}
	wg.Wait()
}

// WaitBlock waits until all series submitted in b have been imported.
func (im *Importer) WaitBlock(b *blockAck) error {
	b.seal()
	im.Flush()
	return b.wait()
}

// ResetStats resets im stats.
func (im *Importer) ResetStats() {
	im.s = &stats{
		startTime: time.Now(),
	}
}

// Stats returns im stats.
func (im *Importer) Stats() string {
	return im.s.String()
}

// AddExtraLabelsToImportPath - adds extra labels query params to given url path.
func AddExtraLabelsToImportPath(path string, extraLabels []string) (string, error) {
	dst := path
	separator := "?"
	for _, extraLabel := range extraLabels {
		if !strings.Contains(extraLabel, "=") {
			return path, fmt.Errorf("bad format for extra_label flag, it must be `key=value`, got: %q", extraLabel)
		}
		if strings.Contains(dst, "?") {
			separator = "&"
		}
		dst += fmt.Sprintf("%sextra_label=%s", separator, url.QueryEscape(extraLabel))
	}
	return dst, nil
}

// NewImporter creates new Importer for the given cfg.
func NewImporter(ctx context.Context, cfg Config) (*Importer, error) {
	if cfg.Concurrency < 1 {
		return nil, fmt.Errorf("concurrency can't be lower than 1")
	}

	addr := strings.TrimRight(cfg.Addr, "/")
	// if single version
	// see https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#how-to-import-time-series-data
	importPath := addr + "/api/v1/import"
	if cfg.AccountID != "" {
		// if cluster version
		// see https://docs.victoriametrics.com/victoriametrics/cluster-victoriametrics/#url-format
		importPath = fmt.Sprintf("%s/insert/%s/prometheus/api/v1/import", addr, cfg.AccountID)
	}
	importPath, err := AddExtraLabelsToImportPath(importPath, cfg.ExtraLabels)
	if err != nil {
		return nil, err
	}

	client := &http.Client{}
	if cfg.Transport != nil {
		client.Transport = cfg.Transport
	}

	im := &Importer{
		addr:               addr,
		client:             client,
		importPath:         importPath,
		accountID:          cfg.AccountID,
		extraLabels:        append([]string(nil), cfg.ExtraLabels...),
		compress:           cfg.Compress,
		authCfg:            cfg.AuthCfg,
		rl:                 limiter.NewLimiter(cfg.RateLimit),
		close:              make(chan struct{}),
		input:              make(chan importItem, cfg.Concurrency*4),
		errors:             make(chan *ImportError, cfg.Concurrency),
		flushWorkerChs:     make([]chan chan struct{}, cfg.Concurrency),
		backoff:            cfg.Backoff,
		significantFigures: cfg.SignificantFigures,
		roundDigits:        cfg.RoundDigits,

		importRequestsTotal:       metrics.GetOrCreateCounter(`vmctl_importer_requests_total`),
		importRequestsErrorsTotal: metrics.GetOrCreateCounter(`vmctl_importer_request_errors_total`),
		importSamplesTotal:        metrics.GetOrCreateCounter(`vmctl_importer_samples_total`),
		importBytesTotal:          metrics.GetOrCreateCounter(`vmctl_importer_bytes_total`),
		importDuration:            metrics.GetOrCreateHistogram(`vmctl_importer_request_duration_seconds`),
	}
	if err := im.Ping(); err != nil {
		return nil, fmt.Errorf("ping to %q failed: %w", addr, err)
	}

	if cfg.BatchSize < 1 {
		cfg.BatchSize = 1e5
	}

	for i := range int(cfg.Concurrency) {
		pbPrefix := fmt.Sprintf(`{{ green "VM worker %d:" }}`, i)
		bar := barpool.AddWithTemplate(pbPrefix+pbTpl, 0)
		ch := make(chan chan struct{})
		im.flushWorkerChs[i] = ch

		im.wg.Go(func() {
			im.startWorker(ctx, bar, cfg.BatchSize, cfg.SignificantFigures, cfg.RoundDigits, ch)
		})
	}
	im.ResetStats()
	return im, nil
}

const pbTpl = `{{ (cycle . "←" "↖" "↑" "↗" "→" "↘" "↓" "↙" ) }} {{speed . "%s samples/s"}}`

// ImportError is type of error generated
// in case of unsuccessful import request
type ImportError struct {
	// The batch of timeseries processed by importer at the moment
	Batch []*TimeSeries
	Err   error
}

// Errors returns a channel for receiving
// import errors if any
func (im *Importer) Errors() chan *ImportError { return im.errors }

// Input returns a channel for sending timeseries
// that need to be imported
func (im *Importer) Input(ts *TimeSeries) error {
	return im.inputItem(importItem{ts: ts})
}

func (im *Importer) SignificantFigures() int {
	return im.significantFigures
}

func (im *Importer) RoundDigits() int {
	return im.roundDigits
}

func (im *Importer) InputBlock(b *blockAck, ts *TimeSeries) error {
	if err := b.add(); err != nil {
		return err
	}
	defer b.wg.Done()

	if err := im.inputItem(importItem{ts: ts, block: b}); err != nil {
		b.finish(err)
		return err
	}
	return nil
}

func (im *Importer) inputItem(item importItem) error {
	select {
	case <-im.close:
		return fmt.Errorf("importer is closed")
	case im.input <- item:
		return nil
	case err := <-im.errors:
		if err != nil && err.Err != nil {
			return err.Err
		}
		return fmt.Errorf("process aborted")
	}
}

// Close sends signal to all goroutines to exit
// and waits until they are finished
func (im *Importer) Close() {
	im.once.Do(func() {
		close(im.close)

		// Wait for active flushes before closing the input channel.
		im.flushMu.Lock()
		im.flushMu.Unlock()

		close(im.input)
		im.wg.Wait()

		go func() {
			im.errWg.Wait()
			close(im.errors)
		}()
	})
}

func (im *Importer) startWorker(ctx context.Context, bar barpool.Bar, batchSize, significantFigures, roundDigits int, flushCh chan chan struct{}) {
	var batch []importItem
	var dataPoints int
	var waitForBatch time.Time

	sendErr := func(err *ImportError) {
		im.errWg.Go(func() {
			im.errors <- err
		})
	}

	for {
		select {
		case <-im.close:
			for item := range im.input {
				item.ts = roundTimeseriesValue(item.ts, significantFigures, roundDigits)
				batch = append(batch, item)
			}
			if len(batch) > 0 {
				exitErr := &ImportError{Batch: importItemsSeries(batch)}
				retryableFunc := func() error { return im.Import(importItemsSeries(batch)) }
				_, err := im.backoff.Retry(ctx, retryableFunc)
				if err != nil {
					exitErr.Err = err
				}
				ackItems(batch, exitErr.Err)
				sendErr(exitErr)
			}
			return

		case done := <-flushCh:
		drain:
			for {
				select {
				case item, ok := <-im.input:
					if !ok {
						break drain
					}
					if waitForBatch.IsZero() {
						waitForBatch = time.Now()
					}
					item.ts = roundTimeseriesValue(item.ts, significantFigures, roundDigits)
					batch = append(batch, item)
					dataPoints += len(item.ts.Values)
					bar.Add(len(item.ts.Values))

					if dataPoints >= batchSize {
						im.s.Lock()
						im.s.idleDuration += time.Since(waitForBatch)
						im.s.Unlock()

						err := im.flush(ctx, importItemsSeries(batch))
						ackItems(batch, err)
						if err != nil {
							sendErr(&ImportError{Batch: importItemsSeries(batch), Err: err})
							batch = make([]importItem, 0, len(batch))
						} else {
							batch = batch[:0]
						}
						dataPoints = 0
						waitForBatch = time.Now()
					}
				default:
					break drain
				}
			}

			if len(batch) > 0 {
				im.s.Lock()
				im.s.idleDuration += time.Since(waitForBatch)
				im.s.Unlock()

				err := im.flush(ctx, importItemsSeries(batch))
				ackItems(batch, err)
				if err != nil {
					sendErr(&ImportError{Batch: importItemsSeries(batch), Err: err})
					batch = make([]importItem, 0, len(batch))
				} else {
					batch = batch[:0]
				}
				dataPoints = 0
				waitForBatch = time.Time{}
			}
			close(done)

		case item, ok := <-im.input:
			if !ok {
				continue
			}
			// init waitForBatch when first
			// value was received
			if waitForBatch.IsZero() {
				waitForBatch = time.Now()
			}

			item.ts = roundTimeseriesValue(item.ts, significantFigures, roundDigits)
			batch = append(batch, item)
			dataPoints += len(item.ts.Values)

			bar.Add(len(item.ts.Values))

			if dataPoints >= batchSize {
				im.s.Lock()
				im.s.idleDuration += time.Since(waitForBatch)
				im.s.Unlock()

				err := im.flush(ctx, importItemsSeries(batch))
				ackItems(batch, err)
				if err != nil {
					sendErr(&ImportError{Batch: importItemsSeries(batch), Err: err})
					batch = make([]importItem, 0, len(batch))
				} else {
					batch = batch[:0]
				}
				dataPoints = 0
				waitForBatch = time.Now()
			}
		}
	}
}

func importItemsSeries(items []importItem) []*TimeSeries {
	result := make([]*TimeSeries, len(items))
	for i := range items {
		result[i] = items[i].ts
	}
	return result
}

func ackItems(items []importItem, err error) {
	for _, item := range items {
		if item.block != nil {
			item.block.finish(err)
		}
	}
}

func (im *Importer) flush(ctx context.Context, b []*TimeSeries) error {
	retryableFunc := func() error { return im.Import(b) }
	attempts, err := im.backoff.Retry(ctx, retryableFunc)
	if err != nil {
		return fmt.Errorf("import failed with %d retries: %w", attempts, err)
	}
	im.s.Lock()
	im.s.retries = attempts
	im.s.Unlock()
	return nil
}

// Ping sends a ping to im.addr.
func (im *Importer) Ping() error {
	url := fmt.Sprintf("%s/health", im.addr)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("cannot create request to %q: %w", im.addr, err)
	}
	if im.authCfg != nil {
		im.authCfg.SetHeaders(req, true)
	}
	resp, err := im.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status code: %d", resp.StatusCode)
	}
	return nil
}

// Import imports tsBatch.
func (im *Importer) Import(tsBatch []*TimeSeries) error {
	if len(tsBatch) < 1 {
		return nil
	}

	startTime := time.Now()
	im.importRequestsTotal.Inc()

	pr, pw := io.Pipe()
	req, err := http.NewRequest(http.MethodPost, im.importPath, pr)
	if err != nil {
		im.importRequestsErrorsTotal.Inc()
		return fmt.Errorf("cannot create request to %q: %w", im.addr, err)
	}
	if im.authCfg != nil {
		im.authCfg.SetHeaders(req, true)
	}
	if im.compress {
		req.Header.Set("Content-Encoding", "gzip")
	}

	errCh := make(chan error)
	go func() {
		errCh <- im.do(req)
		close(errCh)
	}()

	w := io.Writer(pw)
	if im.compress {
		zw, err := gzip.NewWriterLevel(w, 1)
		if err != nil {
			im.importRequestsErrorsTotal.Inc()
			return fmt.Errorf("unexpected error when creating gzip writer: %w", err)
		}
		w = zw
	}
	w = limiter.NewWriteLimiter(w, im.rl)
	bw := bufio.NewWriterSize(w, 16*1024)

	var totalSamples, totalBytes int
	for _, ts := range tsBatch {
		n, err := ts.write(bw)
		if err != nil {
			im.importRequestsErrorsTotal.Inc()
			return fmt.Errorf("write err: %w", err)
		}
		totalBytes += n
		totalSamples += len(ts.Values)
	}
	if err := bw.Flush(); err != nil {
		im.importRequestsErrorsTotal.Inc()
		return err
	}
	if closer, ok := w.(io.Closer); ok {
		err := closer.Close()
		if err != nil {
			im.importRequestsErrorsTotal.Inc()
			return err
		}
	}
	if err := pw.Close(); err != nil {
		im.importRequestsErrorsTotal.Inc()
		return err
	}

	requestErr := <-errCh
	if requestErr != nil {
		im.importRequestsErrorsTotal.Inc()
		im.importDuration.UpdateDuration(startTime)
		return fmt.Errorf("import request error for %q: %w", im.addr, requestErr)
	}

	im.importSamplesTotal.Add(totalSamples)
	im.importBytesTotal.Add(totalBytes)
	im.importDuration.UpdateDuration(startTime)

	im.s.Lock()
	im.s.bytes += uint64(totalBytes)
	im.s.samples += uint64(totalSamples)
	im.s.requests++
	im.s.Unlock()

	return nil
}

// ErrBadRequest represents bad request error.
var ErrBadRequest = errors.New("bad request")

func (im *Importer) do(req *http.Request) error {
	resp, err := im.client.Do(req)
	if err != nil {
		return fmt.Errorf("unexpected error when performing request: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	if resp.StatusCode != http.StatusNoContent {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("failed to read response body for status code %d: %w", resp.StatusCode, err)
		}
		if resp.StatusCode == http.StatusBadRequest {
			return fmt.Errorf("%w: unexpected response code %d: %s", ErrBadRequest, resp.StatusCode, string(body))
		}
		return fmt.Errorf("unexpected response code %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

func byteCountSI(b int64) string {
	const unit = 1000
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB",
		float64(b)/float64(div), "kMGTPE"[exp])
}

func roundTimeseriesValue(ts *TimeSeries, significantFigures, roundDigits int) *TimeSeries {
	if significantFigures > 0 {
		for i, v := range ts.Values {
			ts.Values[i] = decimal.RoundToSignificantFigures(v, significantFigures)
		}
	}
	if roundDigits < 100 {
		for i, v := range ts.Values {
			ts.Values[i] = decimal.RoundToDecimalDigits(v, roundDigits)
		}
	}

	return ts
}
