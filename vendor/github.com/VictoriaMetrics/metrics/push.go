package metrics

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"compress/gzip"
)

// PushOptions is the list of options, which may be applied to InitPushWithOptions().
type PushOptions struct {
	// ExtraLabels is an optional comma-separated list of `label="value"` labels, which must be added to all the metrics before pushing them to pushURL.
	ExtraLabels string

	// Headers is an optional list of HTTP headers to add to every push request to pushURL.
	//
	// Every item in the list must have the form `Header: value`. For example, `Authorization: Custom my-top-secret`.
	Headers []string

	// Whether to disable HTTP request body compression before sending the metrics to pushURL.
	//
	// By default the compression is enabled.
	DisableCompression bool

	// Method is HTTP request method to use when pushing metrics to pushURL.
	//
	// By default the Method is GET.
	Method string

	// Optional WaitGroup for waiting until all the push workers created with this WaitGroup are stopped.
	WaitGroup *sync.WaitGroup
}

// InitPushWithOptions sets up periodic push for globally registered metrics to the given pushURL with the given interval.
//
// The periodic push is stopped when ctx is canceled.
// It is possible to wait until the background metrics push worker is stopped on a WaitGroup passed via opts.WaitGroup.
//
// If pushProcessMetrics is set to true, then 'process_*' and `go_*` metrics are also pushed to pushURL.
//
// opts may contain additional configuration options if non-nil.
//
// The metrics are pushed to pushURL in Prometheus text exposition format.
// See https://github.com/prometheus/docs/blob/main/content/docs/instrumenting/exposition_formats.md#text-based-format
//
// It is recommended pushing metrics to /api/v1/import/prometheus endpoint according to
// https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#how-to-import-data-in-prometheus-exposition-format
//
// It is OK calling InitPushWithOptions multiple times with different pushURL -
// in this case metrics are pushed to all the provided pushURL urls.
func InitPushWithOptions(ctx context.Context, pushURL string, interval time.Duration, pushProcessMetrics bool, opts *PushOptions) error {
	writeMetrics := func(w io.Writer) {
		WritePrometheus(w, pushProcessMetrics)
	}
	return InitPushExtWithOptions(ctx, pushURL, interval, writeMetrics, opts)
}

// InitPushProcessMetrics sets up periodic push for 'process_*' metrics to the given pushURL with the given interval.
//
// extraLabels may contain comma-separated list of `label="value"` labels, which will be added
// to all the metrics before pushing them to pushURL.
//
// The metrics are pushed to pushURL in Prometheus text exposition format.
// See https://github.com/prometheus/docs/blob/main/content/docs/instrumenting/exposition_formats.md#text-based-format
//
// It is recommended pushing metrics to /api/v1/import/prometheus endpoint according to
// https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#how-to-import-data-in-prometheus-exposition-format
//
// It is OK calling InitPushProcessMetrics multiple times with different pushURL -
// in this case metrics are pushed to all the provided pushURL urls.
func InitPushProcessMetrics(pushURL string, interval time.Duration, extraLabels string) error {
	return InitPushExt(pushURL, interval, extraLabels, WriteProcessMetrics)
}

// InitPush sets up periodic push for globally registered metrics to the given pushURL with the given interval.
//
// extraLabels may contain comma-separated list of `label="value"` labels, which will be added
// to all the metrics before pushing them to pushURL.
//
// If pushProcessMetrics is set to true, then 'process_*' and `go_*` metrics are also pushed to pushURL.
//
// The metrics are pushed to pushURL in Prometheus text exposition format.
// See https://github.com/prometheus/docs/blob/main/content/docs/instrumenting/exposition_formats.md#text-based-format
//
// It is recommended pushing metrics to /api/v1/import/prometheus endpoint according to
// https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#how-to-import-data-in-prometheus-exposition-format
//
// It is OK calling InitPush multiple times with different pushURL -
// in this case metrics are pushed to all the provided pushURL urls.
func InitPush(pushURL string, interval time.Duration, extraLabels string, pushProcessMetrics bool) error {
	writeMetrics := func(w io.Writer) {
		WritePrometheus(w, pushProcessMetrics)
	}
	return InitPushExt(pushURL, interval, extraLabels, writeMetrics)
}

// PushMetrics pushes globally registered metrics to pushURL.
//
// If pushProcessMetrics is set to true, then 'process_*' and `go_*` metrics are also pushed to pushURL.
//
// opts may contain additional configuration options if non-nil.
//
// It is recommended pushing metrics to /api/v1/import/prometheus endpoint according to
// https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#how-to-import-data-in-prometheus-exposition-format
func PushMetrics(ctx context.Context, pushURL string, pushProcessMetrics bool, opts *PushOptions) error {
	writeMetrics := func(w io.Writer) {
		WritePrometheus(w, pushProcessMetrics)
	}
	return PushMetricsExt(ctx, pushURL, writeMetrics, opts)
}

// InitPushWithOptions sets up periodic push for metrics from s to the given pushURL with the given interval.
//
// The periodic push is stopped when the ctx is canceled.
// It is possible to wait until the background metrics push worker is stopped on a WaitGroup passed via opts.WaitGroup.
//
// opts may contain additional configuration options if non-nil.
//
// The metrics are pushed to pushURL in Prometheus text exposition format.
// See https://github.com/prometheus/docs/blob/main/content/docs/instrumenting/exposition_formats.md#text-based-format
//
// It is recommended pushing metrics to /api/v1/import/prometheus endpoint according to
// https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#how-to-import-data-in-prometheus-exposition-format
//
// It is OK calling InitPushWithOptions multiple times with different pushURL -
// in this case metrics are pushed to all the provided pushURL urls.
func (s *Set) InitPushWithOptions(ctx context.Context, pushURL string, interval time.Duration, opts *PushOptions) error {
	return InitPushExtWithOptions(ctx, pushURL, interval, s.WritePrometheus, opts)
}

// InitPush sets up periodic push for metrics from s to the given pushURL with the given interval.
//
// extraLabels may contain comma-separated list of `label="value"` labels, which will be added
// to all the metrics before pushing them to pushURL.
//
// The metrics are pushed to pushURL in Prometheus text exposition format.
// See https://github.com/prometheus/docs/blob/main/content/docs/instrumenting/exposition_formats.md#text-based-format
//
// It is recommended pushing metrics to /api/v1/import/prometheus endpoint according to
// https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#how-to-import-data-in-prometheus-exposition-format
//
// It is OK calling InitPush multiple times with different pushURL -
// in this case metrics are pushed to all the provided pushURL urls.
func (s *Set) InitPush(pushURL string, interval time.Duration, extraLabels string) error {
	return InitPushExt(pushURL, interval, extraLabels, s.WritePrometheus)
}

// PushMetrics pushes s metrics to pushURL.
//
// opts may contain additional configuration options if non-nil.
//
// It is recommended pushing metrics to /api/v1/import/prometheus endpoint according to
// https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#how-to-import-data-in-prometheus-exposition-format
func (s *Set) PushMetrics(ctx context.Context, pushURL string, opts *PushOptions) error {
	return PushMetricsExt(ctx, pushURL, s.WritePrometheus, opts)
}

// InitPushExt sets up periodic push for metrics obtained by calling writeMetrics with the given interval.
//
// extraLabels may contain comma-separated list of `label="value"` labels, which will be added
// to all the metrics before pushing them to pushURL.
//
// The writeMetrics callback must write metrics to w in Prometheus text exposition format without timestamps and trailing comments.
// See https://github.com/prometheus/docs/blob/main/content/docs/instrumenting/exposition_formats.md#text-based-format
//
// It is recommended pushing metrics to /api/v1/import/prometheus endpoint according to
// https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#how-to-import-data-in-prometheus-exposition-format
//
// It is OK calling InitPushExt multiple times with different pushURL -
// in this case metrics are pushed to all the provided pushURL urls.
//
// It is OK calling InitPushExt multiple times with different writeMetrics -
// in this case all the metrics generated by writeMetrics callbacks are written to pushURL.
func InitPushExt(pushURL string, interval time.Duration, extraLabels string, writeMetrics func(w io.Writer)) error {
	opts := &PushOptions{
		ExtraLabels: extraLabels,
	}
	return InitPushExtWithOptions(context.Background(), pushURL, interval, writeMetrics, opts)
}

// InitPushExtWithOptions sets up periodic push for metrics obtained by calling writeMetrics with the given interval.
//
// The writeMetrics callback must write metrics to w in Prometheus text exposition format without timestamps and trailing comments.
// See https://github.com/prometheus/docs/blob/main/content/docs/instrumenting/exposition_formats.md#text-based-format
//
// The periodic push is stopped when the ctx is canceled.
// It is possible to wait until the background metrics push worker is stopped on a WaitGroup passed via opts.WaitGroup.
//
// opts may contain additional configuration options if non-nil.
//
// It is recommended pushing metrics to /api/v1/import/prometheus endpoint according to
// https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#how-to-import-data-in-prometheus-exposition-format
//
// It is OK calling InitPushExtWithOptions multiple times with different pushURL -
// in this case metrics are pushed to all the provided pushURL urls.
//
// It is OK calling InitPushExtWithOptions multiple times with different writeMetrics -
// in this case all the metrics generated by writeMetrics callbacks are written to pushURL.
func InitPushExtWithOptions(ctx context.Context, pushURL string, interval time.Duration, writeMetrics func(w io.Writer), opts *PushOptions) error {
	pc, err := newPushContext(pushURL, opts)
	if err != nil {
		return err
	}

	// validate interval
	if interval <= 0 {
		return fmt.Errorf("interval must be positive; got %s", interval)
	}
	pushMetricsSet.GetOrCreateFloatCounter(fmt.Sprintf(`metrics_push_interval_seconds{url=%q}`, pc.pushURLRedacted)).Set(interval.Seconds())

	var wg *sync.WaitGroup
	if opts != nil {
		wg = opts.WaitGroup
		if wg != nil {
			wg.Add(1)
		}
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		stopCh := ctx.Done()
		for {
			select {
			case <-ticker.C:
				ctxLocal, cancel := context.WithTimeout(ctx, interval+time.Second)
				err := pc.pushMetrics(ctxLocal, writeMetrics)
				cancel()
				if err != nil {
					log.Printf("ERROR: metrics.push: %s", err)
				}
			case <-stopCh:
				if wg != nil {
					wg.Done()
				}
				return
			}
		}
	}()

	return nil
}

// PushMetricsExt pushes metrics generated by wirteMetrics to pushURL.
//
// The writeMetrics callback must write metrics to w in Prometheus text exposition format without timestamps and trailing comments.
// See https://github.com/prometheus/docs/blob/main/content/docs/instrumenting/exposition_formats.md#text-based-format
//
// opts may contain additional configuration options if non-nil.
//
// It is recommended pushing metrics to /api/v1/import/prometheus endpoint according to
// https://docs.victoriametrics.com/victoriametrics/single-server-victoriametrics/#how-to-import-data-in-prometheus-exposition-format
func PushMetricsExt(ctx context.Context, pushURL string, writeMetrics func(w io.Writer), opts *PushOptions) error {
	pc, err := newPushContext(pushURL, opts)
	if err != nil {
		return err
	}
	return pc.pushMetrics(ctx, writeMetrics)
}

type pushContext struct {
	pushURL            *url.URL
	method             string
	pushURLRedacted    string
	extraLabels        string
	headers            http.Header
	disableCompression bool

	client *http.Client

	pushesTotal      *Counter
	bytesPushedTotal *Counter
	pushBlockSize    *Histogram
	pushDuration     *Histogram
	pushErrors       *Counter
}

func newPushContext(pushURL string, opts *PushOptions) (*pushContext, error) {
	if opts == nil {
		opts = &PushOptions{}
	}

	// validate pushURL
	pu, err := url.Parse(pushURL)
	if err != nil {
		return nil, fmt.Errorf("cannot parse pushURL=%q: %w", pushURL, err)
	}
	if pu.Scheme != "http" && pu.Scheme != "https" {
		return nil, fmt.Errorf("unsupported scheme in pushURL=%q; expecting 'http' or 'https'", pushURL)
	}
	if pu.Host == "" {
		return nil, fmt.Errorf("missing host in pushURL=%q", pushURL)
	}

	method := opts.Method
	if method == "" {
		method = http.MethodGet
	}

	// validate ExtraLabels
	extraLabels := opts.ExtraLabels
	if err := validateTags(extraLabels); err != nil {
		return nil, fmt.Errorf("invalid extraLabels=%q: %w", extraLabels, err)
	}

	// validate Headers
	headers := make(http.Header)
	for _, h := range opts.Headers {
		n := strings.IndexByte(h, ':')
		if n < 0 {
			return nil, fmt.Errorf("missing `:` delimiter in the header %q", h)
		}
		name := strings.TrimSpace(h[:n])
		value := strings.TrimSpace(h[n+1:])
		headers.Add(name, value)
	}

	pushURLRedacted := pu.Redacted()
	client := &http.Client{}
	return &pushContext{
		pushURL:            pu,
		method:             method,
		pushURLRedacted:    pushURLRedacted,
		extraLabels:        extraLabels,
		headers:            headers,
		disableCompression: opts.DisableCompression,

		client: client,

		pushesTotal:      pushMetricsSet.GetOrCreateCounter(fmt.Sprintf(`metrics_push_total{url=%q}`, pushURLRedacted)),
		bytesPushedTotal: pushMetricsSet.GetOrCreateCounter(fmt.Sprintf(`metrics_push_bytes_pushed_total{url=%q}`, pushURLRedacted)),
		pushBlockSize:    pushMetricsSet.GetOrCreateHistogram(fmt.Sprintf(`metrics_push_block_size_bytes{url=%q}`, pushURLRedacted)),
		pushDuration:     pushMetricsSet.GetOrCreateHistogram(fmt.Sprintf(`metrics_push_duration_seconds{url=%q}`, pushURLRedacted)),
		pushErrors:       pushMetricsSet.GetOrCreateCounter(fmt.Sprintf(`metrics_push_errors_total{url=%q}`, pushURLRedacted)),
	}, nil
}

func (pc *pushContext) pushMetrics(ctx context.Context, writeMetrics func(w io.Writer)) error {
	bb := getBytesBuffer()
	defer putBytesBuffer(bb)

	writeMetrics(bb)

	if len(pc.extraLabels) > 0 {
		bbTmp := getBytesBuffer()
		bbTmp.B = append(bbTmp.B[:0], bb.B...)
		bb.B = addExtraLabels(bb.B[:0], bbTmp.B, pc.extraLabels)
		putBytesBuffer(bbTmp)
	}
	if !pc.disableCompression {
		bbTmp := getBytesBuffer()
		bbTmp.B = append(bbTmp.B[:0], bb.B...)
		bb.B = bb.B[:0]
		zw := getGzipWriter(bb)
		if _, err := zw.Write(bbTmp.B); err != nil {
			panic(fmt.Errorf("BUG: cannot write %d bytes to gzip writer: %s", len(bbTmp.B), err))
		}
		if err := zw.Close(); err != nil {
			panic(fmt.Errorf("BUG: cannot flush metrics to gzip writer: %s", err))
		}
		putGzipWriter(zw)
		putBytesBuffer(bbTmp)
	}

	// Update metrics
	pc.pushesTotal.Inc()
	blockLen := len(bb.B)
	pc.bytesPushedTotal.Add(blockLen)
	pc.pushBlockSize.Update(float64(blockLen))

	// Prepare the request to sent to pc.pushURL
	reqBody := bytes.NewReader(bb.B)
	req, err := http.NewRequestWithContext(ctx, pc.method, pc.pushURL.String(), reqBody)
	if err != nil {
		panic(fmt.Errorf("BUG: metrics.push: cannot initialize request for metrics push to %q: %w", pc.pushURLRedacted, err))
	}

	req.Header.Set("Content-Type", "text/plain")
	// Set the needed headers, and `Content-Type` allowed be overwrited.
	for name, values := range pc.headers {
		for _, value := range values {
			req.Header.Add(name, value)
		}
	}
	if !pc.disableCompression {
		req.Header.Set("Content-Encoding", "gzip")
	}

	// Perform the request
	startTime := time.Now()
	resp, err := pc.client.Do(req)
	pc.pushDuration.UpdateDuration(startTime)
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return nil
		}
		pc.pushErrors.Inc()
		return fmt.Errorf("cannot push metrics to %q: %s", pc.pushURLRedacted, err)
	}
	if resp.StatusCode/100 != 2 {
		body, _ := ioutil.ReadAll(resp.Body)
		_ = resp.Body.Close()
		pc.pushErrors.Inc()
		return fmt.Errorf("unexpected status code in response from %q: %d; expecting 2xx; response body: %q", pc.pushURLRedacted, resp.StatusCode, body)
	}
	_ = resp.Body.Close()
	return nil
}

var pushMetricsSet = NewSet()

func writePushMetrics(w io.Writer) {
	pushMetricsSet.WritePrometheus(w)
}

func addExtraLabels(dst, src []byte, extraLabels string) []byte {
	for len(src) > 0 {
		var line []byte
		n := bytes.IndexByte(src, '\n')
		if n >= 0 {
			line = src[:n]
			src = src[n+1:]
		} else {
			line = src
			src = nil
		}
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			// Skip empy lines
			continue
		}
		if bytes.HasPrefix(line, bashBytes) {
			// Copy comments as is
			dst = append(dst, line...)
			dst = append(dst, '\n')
			continue
		}
		n = bytes.IndexByte(line, '{')
		if n >= 0 {
			dst = append(dst, line[:n+1]...)
			dst = append(dst, extraLabels...)
			dst = append(dst, ',')
			dst = append(dst, line[n+1:]...)
		} else {
			n = bytes.LastIndexByte(line, ' ')
			if n < 0 {
				panic(fmt.Errorf("BUG: missing whitespace between metric name and metric value in Prometheus text exposition line %q", line))
			}
			dst = append(dst, line[:n]...)
			dst = append(dst, '{')
			dst = append(dst, extraLabels...)
			dst = append(dst, '}')
			dst = append(dst, line[n:]...)
		}
		dst = append(dst, '\n')
	}
	return dst
}

var bashBytes = []byte("#")

func getBytesBuffer() *bytesBuffer {
	v := bytesBufferPool.Get()
	if v == nil {
		return &bytesBuffer{}
	}
	return v.(*bytesBuffer)
}

func putBytesBuffer(bb *bytesBuffer) {
	bb.B = bb.B[:0]
	bytesBufferPool.Put(bb)
}

var bytesBufferPool sync.Pool

type bytesBuffer struct {
	B []byte
}

func (bb *bytesBuffer) Write(p []byte) (int, error) {
	bb.B = append(bb.B, p...)
	return len(p), nil
}

func getGzipWriter(w io.Writer) *gzip.Writer {
	v := gzipWriterPool.Get()
	if v == nil {
		return gzip.NewWriter(w)
	}
	zw := v.(*gzip.Writer)
	zw.Reset(w)
	return zw
}

func putGzipWriter(zw *gzip.Writer) {
	zw.Reset(io.Discard)
	gzipWriterPool.Put(zw)
}

var gzipWriterPool sync.Pool
