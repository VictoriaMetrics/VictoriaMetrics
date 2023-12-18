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
}

// InitPushWithOptions sets up periodic push for globally registered metrics to the given pushURL with the given interval.
//
// The periodic push is stopped when ctx is canceled.
//
// If pushProcessMetrics is set to true, then 'process_*' and `go_*` metrics are also pushed to pushURL.
//
// opts may contain additional configuration options if non-nil.
//
// The metrics are pushed to pushURL in Prometheus text exposition format.
// See https://github.com/prometheus/docs/blob/main/content/docs/instrumenting/exposition_formats.md#text-based-format
//
// It is recommended pushing metrics to /api/v1/import/prometheus endpoint according to
// https://docs.victoriametrics.com/#how-to-import-data-in-prometheus-exposition-format
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
// https://docs.victoriametrics.com/#how-to-import-data-in-prometheus-exposition-format
//
// It is OK calling InitPushProcessMetrics multiple times with different pushURL -
// in this case metrics are pushed to all the provided pushURL urls.
func InitPushProcessMetrics(pushURL string, interval time.Duration, extraLabels string) error {
	writeMetrics := func(w io.Writer) {
		WriteProcessMetrics(w)
	}
	return InitPushExt(pushURL, interval, extraLabels, writeMetrics)
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
// https://docs.victoriametrics.com/#how-to-import-data-in-prometheus-exposition-format
//
// It is OK calling InitPush multiple times with different pushURL -
// in this case metrics are pushed to all the provided pushURL urls.
func InitPush(pushURL string, interval time.Duration, extraLabels string, pushProcessMetrics bool) error {
	writeMetrics := func(w io.Writer) {
		WritePrometheus(w, pushProcessMetrics)
	}
	return InitPushExt(pushURL, interval, extraLabels, writeMetrics)
}

// InitPushWithOptions sets up periodic push for metrics from s to the given pushURL with the given interval.
//
// The periodic push is stopped when the ctx is canceled.
//
// opts may contain additional configuration options if non-nil.
//
// The metrics are pushed to pushURL in Prometheus text exposition format.
// See https://github.com/prometheus/docs/blob/main/content/docs/instrumenting/exposition_formats.md#text-based-format
//
// It is recommended pushing metrics to /api/v1/import/prometheus endpoint according to
// https://docs.victoriametrics.com/#how-to-import-data-in-prometheus-exposition-format
//
// It is OK calling InitPushWithOptions multiple times with different pushURL -
// in this case metrics are pushed to all the provided pushURL urls.
func (s *Set) InitPushWithOptions(ctx context.Context, pushURL string, interval time.Duration, opts *PushOptions) error {
	writeMetrics := func(w io.Writer) {
		s.WritePrometheus(w)
	}
	return InitPushExtWithOptions(ctx, pushURL, interval, writeMetrics, opts)
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
// https://docs.victoriametrics.com/#how-to-import-data-in-prometheus-exposition-format
//
// It is OK calling InitPush multiple times with different pushURL -
// in this case metrics are pushed to all the provided pushURL urls.
func (s *Set) InitPush(pushURL string, interval time.Duration, extraLabels string) error {
	writeMetrics := func(w io.Writer) {
		s.WritePrometheus(w)
	}
	return InitPushExt(pushURL, interval, extraLabels, writeMetrics)
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
// https://docs.victoriametrics.com/#how-to-import-data-in-prometheus-exposition-format
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
//
// opts may contain additional configuration options if non-nil.
//
// It is recommended pushing metrics to /api/v1/import/prometheus endpoint according to
// https://docs.victoriametrics.com/#how-to-import-data-in-prometheus-exposition-format
//
// It is OK calling InitPushExtWithOptions multiple times with different pushURL -
// in this case metrics are pushed to all the provided pushURL urls.
//
// It is OK calling InitPushExtWithOptions multiple times with different writeMetrics -
// in this case all the metrics generated by writeMetrics callbacks are written to pushURL.
func InitPushExtWithOptions(ctx context.Context, pushURL string, interval time.Duration, writeMetrics func(w io.Writer), opts *PushOptions) error {
	// validate pushURL
	pu, err := url.Parse(pushURL)
	if err != nil {
		return fmt.Errorf("cannot parse pushURL=%q: %w", pushURL, err)
	}
	if pu.Scheme != "http" && pu.Scheme != "https" {
		return fmt.Errorf("unsupported scheme in pushURL=%q; expecting 'http' or 'https'", pushURL)
	}
	if pu.Host == "" {
		return fmt.Errorf("missing host in pushURL=%q", pushURL)
	}

	// validate interval
	if interval <= 0 {
		return fmt.Errorf("interval must be positive; got %s", interval)
	}

	// validate ExtraLabels
	var extraLabels string
	if opts != nil {
		extraLabels = opts.ExtraLabels
	}
	if err := validateTags(extraLabels); err != nil {
		return fmt.Errorf("invalid extraLabels=%q: %w", extraLabels, err)
	}

	// validate Headers
	headers := make(http.Header)
	if opts != nil {
		for _, h := range opts.Headers {
			n := strings.IndexByte(h, ':')
			if n < 0 {
				return fmt.Errorf("missing `:` delimiter in the header %q", h)
			}
			name := strings.TrimSpace(h[:n])
			value := strings.TrimSpace(h[n+1:])
			headers.Add(name, value)
		}
	}

	// validate DisableCompression
	disableCompression := false
	if opts != nil {
		disableCompression = opts.DisableCompression
	}

	// Initialize metrics for the given pushURL
	pushURLRedacted := pu.Redacted()
	pushesTotal := pushMetrics.GetOrCreateCounter(fmt.Sprintf(`metrics_push_total{url=%q}`, pushURLRedacted))
	pushErrorsTotal := pushMetrics.GetOrCreateCounter(fmt.Sprintf(`metrics_push_errors_total{url=%q}`, pushURLRedacted))
	bytesPushedTotal := pushMetrics.GetOrCreateCounter(fmt.Sprintf(`metrics_push_bytes_pushed_total{url=%q}`, pushURLRedacted))
	pushDuration := pushMetrics.GetOrCreateHistogram(fmt.Sprintf(`metrics_push_duration_seconds{url=%q}`, pushURLRedacted))
	pushBlockSize := pushMetrics.GetOrCreateHistogram(fmt.Sprintf(`metrics_push_block_size_bytes{url=%q}`, pushURLRedacted))
	pushMetrics.GetOrCreateFloatCounter(fmt.Sprintf(`metrics_push_interval_seconds{url=%q}`, pushURLRedacted)).Set(interval.Seconds())

	c := &http.Client{
		Timeout: interval,
	}
	go func() {
		ticker := time.NewTicker(interval)
		var bb bytes.Buffer
		var tmpBuf []byte
		zw := gzip.NewWriter(&bb)
		stopCh := ctx.Done()
		for {
			select {
			case <-ticker.C:
			case <-stopCh:
				return
			}

			bb.Reset()
			writeMetrics(&bb)
			if len(extraLabels) > 0 {
				tmpBuf = addExtraLabels(tmpBuf[:0], bb.Bytes(), extraLabels)
				bb.Reset()
				if _, err := bb.Write(tmpBuf); err != nil {
					panic(fmt.Errorf("BUG: cannot write %d bytes to bytes.Buffer: %s", len(tmpBuf), err))
				}
			}
			if !disableCompression {
				tmpBuf = append(tmpBuf[:0], bb.Bytes()...)
				bb.Reset()
				zw.Reset(&bb)
				if _, err := zw.Write(tmpBuf); err != nil {
					panic(fmt.Errorf("BUG: cannot write %d bytes to gzip writer: %s", len(tmpBuf), err))
				}
				if err := zw.Close(); err != nil {
					panic(fmt.Errorf("BUG: cannot flush metrics to gzip writer: %s", err))
				}
			}
			pushesTotal.Inc()
			blockLen := bb.Len()
			bytesPushedTotal.Add(blockLen)
			pushBlockSize.Update(float64(blockLen))
			req, err := http.NewRequestWithContext(ctx, "GET", pushURL, &bb)
			if err != nil {
				panic(fmt.Errorf("BUG: metrics.push: cannot initialize request for metrics push to %q: %w", pushURLRedacted, err))
			}

			// Set the needed headers
			for name, values := range headers {
				for _, value := range values {
					req.Header.Add(name, value)
				}
			}
			req.Header.Set("Content-Type", "text/plain")

			if !disableCompression {
				req.Header.Set("Content-Encoding", "gzip")
			}

			// Perform the request
			startTime := time.Now()
			resp, err := c.Do(req)
			pushDuration.UpdateDuration(startTime)
			if err != nil {
				if !errors.Is(err, context.Canceled) {
					log.Printf("ERROR: metrics.push: cannot push metrics to %q: %s", pushURLRedacted, err)
					pushErrorsTotal.Inc()
				}
				continue
			}
			if resp.StatusCode/100 != 2 {
				body, _ := ioutil.ReadAll(resp.Body)
				_ = resp.Body.Close()
				log.Printf("ERROR: metrics.push: unexpected status code in response from %q: %d; expecting 2xx; response body: %q",
					pushURLRedacted, resp.StatusCode, body)
				pushErrorsTotal.Inc()
				continue
			}
			_ = resp.Body.Close()
		}
	}()
	return nil
}

var pushMetrics = NewSet()

func writePushMetrics(w io.Writer) {
	pushMetrics.WritePrometheus(w)
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
