package vm

import (
	"bufio"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"net/http"
	"strings"
	"sync"
	"time"

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
	// User name for basic auth
	User string
	// Password for basic auth
	Password string
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
}

// Importer performs insertion of timeseries
// via VictoriaMetrics import protocol
// see https://docs.victoriametrics.com/#how-to-import-time-series-data
type Importer struct {
	addr       string
	importPath string
	compress   bool
	user       string
	password   string

	close  chan struct{}
	input  chan *TimeSeries
	errors chan *ImportError

	rl *limiter.Limiter

	wg   sync.WaitGroup
	once sync.Once

	s *stats
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
		dst += fmt.Sprintf("%sextra_label=%s", separator, extraLabel)
	}
	return dst, nil
}

// NewImporter creates new Importer for the given cfg.
func NewImporter(cfg Config) (*Importer, error) {
	if cfg.Concurrency < 1 {
		return nil, fmt.Errorf("concurrency can't be lower than 1")
	}

	addr := strings.TrimRight(cfg.Addr, "/")
	// if single version
	// see https://docs.victoriametrics.com/#how-to-import-time-series-data
	importPath := addr + "/api/v1/import"
	if cfg.AccountID != "" {
		// if cluster version
		// see https://docs.victoriametrics.com/Cluster-VictoriaMetrics.html#url-format
		importPath = fmt.Sprintf("%s/insert/%s/prometheus/api/v1/import", addr, cfg.AccountID)
	}
	importPath, err := AddExtraLabelsToImportPath(importPath, cfg.ExtraLabels)
	if err != nil {
		return nil, err
	}

	im := &Importer{
		addr:       addr,
		importPath: importPath,
		compress:   cfg.Compress,
		user:       cfg.User,
		password:   cfg.Password,
		rl:         limiter.NewLimiter(cfg.RateLimit),
		close:      make(chan struct{}),
		input:      make(chan *TimeSeries, cfg.Concurrency*4),
		errors:     make(chan *ImportError, cfg.Concurrency),
	}
	if err := im.Ping(); err != nil {
		return nil, fmt.Errorf("ping to %q failed: %s", addr, err)
	}

	if cfg.BatchSize < 1 {
		cfg.BatchSize = 1e5
	}

	im.wg.Add(int(cfg.Concurrency))
	for i := 0; i < int(cfg.Concurrency); i++ {
		go func() {
			defer im.wg.Done()
			im.startWorker(cfg.BatchSize, cfg.SignificantFigures, cfg.RoundDigits)
		}()
	}
	im.ResetStats()
	return im, nil
}

// ImportError is type of error generated
// in case of unsuccessful import request
type ImportError struct {
	// The batch of timeseries processed by importer at the moment
	Batch []*TimeSeries
	// The error that appeared during insert
	// If err is nil - no error happened and Batch
	// Is the latest delivered Batch.
	Err error
}

// Errors returns a channel for receiving
// import errors if any
func (im *Importer) Errors() chan *ImportError { return im.errors }

// Input returns a channel for sending timeseries
// that need to be imported
func (im *Importer) Input() chan<- *TimeSeries { return im.input }

// Close sends signal to all goroutines to exit
// and waits until they are finished
func (im *Importer) Close() {
	im.once.Do(func() {
		close(im.close)
		im.wg.Wait()
		close(im.errors)
	})
}

func (im *Importer) startWorker(batchSize, significantFigures, roundDigits int) {
	var batch []*TimeSeries
	var dataPoints int
	var waitForBatch time.Time
	for {
		select {
		case <-im.close:
			exitErr := &ImportError{
				Batch: batch,
			}
			if err := im.Import(batch); err != nil {
				exitErr.Err = err
			}
			im.errors <- exitErr
			return
		case ts := <-im.input:
			// init waitForBatch when first
			// value was received
			if waitForBatch.IsZero() {
				waitForBatch = time.Now()
			}

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

			batch = append(batch, ts)
			dataPoints += len(ts.Values)
			if dataPoints < batchSize {
				continue
			}
			im.s.Lock()
			im.s.idleDuration += time.Since(waitForBatch)
			im.s.Unlock()

			if err := im.flush(batch); err != nil {
				im.errors <- &ImportError{
					Batch: batch,
					Err:   err,
				}
				// make a new batch, since old one was referenced as err
				batch = make([]*TimeSeries, len(batch))
			}
			batch = batch[:0]
			dataPoints = 0
			waitForBatch = time.Now()
		}
	}
}

const (
	// TODO: make configurable
	backoffRetries     = 5
	backoffFactor      = 1.7
	backoffMinDuration = time.Second
)

func (im *Importer) flush(b []*TimeSeries) error {
	var err error
	for i := 0; i < backoffRetries; i++ {
		err = im.Import(b)
		if err == nil {
			return nil
		}
		if errors.Is(err, ErrBadRequest) {
			return err // fail fast if not recoverable
		}
		im.s.Lock()
		im.s.retries++
		im.s.Unlock()
		backoff := float64(backoffMinDuration) * math.Pow(backoffFactor, float64(i))
		time.Sleep(time.Duration(backoff))
	}
	return fmt.Errorf("import failed with %d retries: %s", backoffRetries, err)
}

// Ping sends a ping to im.addr.
func (im *Importer) Ping() error {
	url := fmt.Sprintf("%s/health", im.addr)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("cannot create request to %q: %s", im.addr, err)
	}
	if im.user != "" {
		req.SetBasicAuth(im.user, im.password)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
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

	pr, pw := io.Pipe()
	req, err := http.NewRequest("POST", im.importPath, pr)
	if err != nil {
		return fmt.Errorf("cannot create request to %q: %s", im.addr, err)
	}
	if im.user != "" {
		req.SetBasicAuth(im.user, im.password)
	}
	if im.compress {
		req.Header.Set("Content-Encoding", "gzip")
	}

	errCh := make(chan error)
	go func() {
		errCh <- do(req)
		close(errCh)
	}()

	w := io.Writer(pw)
	if im.compress {
		zw, err := gzip.NewWriterLevel(w, 1)
		if err != nil {
			return fmt.Errorf("unexpected error when creating gzip writer: %s", err)
		}
		w = zw
	}
	w = limiter.NewWriteLimiter(w, im.rl)
	bw := bufio.NewWriterSize(w, 16*1024)

	var totalSamples, totalBytes int
	for _, ts := range tsBatch {
		n, err := ts.write(bw)
		if err != nil {
			return fmt.Errorf("write err: %w", err)
		}
		totalBytes += n
		totalSamples += len(ts.Values)
	}
	if err := bw.Flush(); err != nil {
		return err
	}
	if closer, ok := w.(io.Closer); ok {
		err := closer.Close()
		if err != nil {
			return err
		}
	}
	if err := pw.Close(); err != nil {
		return err
	}

	requestErr := <-errCh
	if requestErr != nil {
		return fmt.Errorf("import request error for %q: %w", im.addr, requestErr)
	}

	im.s.Lock()
	im.s.bytes += uint64(totalBytes)
	im.s.samples += uint64(totalSamples)
	im.s.requests++
	im.s.Unlock()

	return nil
}

// ErrBadRequest represents bad request error.
var ErrBadRequest = errors.New("bad request")

func do(req *http.Request) error {
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("unexpected error when performing request: %s", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	if resp.StatusCode != http.StatusNoContent {
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("failed to read response body for status code %d: %s", resp.StatusCode, err)
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
