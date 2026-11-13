// Package push implements background exporting of VictoriaMetrics query traces
// to VictoriaTraces in OTLP protobuf format over HTTP.
package push

import (
	"bytes"
	"compress/gzip"
	"flag"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/buildinfo"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/querytracer"
	"github.com/VictoriaMetrics/metrics"
)

var (
	pushURL = flag.String("search.traceExportURL", "", "If set, query traces are exported to this URL in OTLP protobuf format. "+
		"For example, -search.traceExportURL=http://victoria-traces:4318/insert/opentelemetry/v1/traces . "+
		"See https://docs.victoriametrics.com/victoriametrics/query-tracing/")
	minTraceDuration = flag.Duration("search.traceExportMinDuration", 0, "Minimum query duration for exporting traces via -search.traceExportURL. "+
		"Traces for faster queries are dropped. 0 means all traces are exported.")
)

var (
	pushesTotal  = metrics.NewCounter(`vm_trace_export_pushes_total`)
	errorsTotal  = metrics.NewCounter(`vm_trace_export_errors_total`)
	droppedTotal = metrics.NewCounter(`vm_trace_export_dropped_total`)
)

// queueCap is the maximum number of pending traces waiting to be exported.
const queueCap = 1000

var (
	queue chan *querytracer.OTLPTrace

	stopCh chan struct{}
	wg     sync.WaitGroup
)

// IsEnabled returns true when trace export is configured.
func IsEnabled() bool {
	return *pushURL != ""
}

// MinTraceDuration returns the configured minimum trace duration threshold.
func MinTraceDuration() time.Duration {
	return *minTraceDuration
}

// Init starts the background export goroutine. Must be called after flag.Parse and logger.Init.
func Init() {
	if !IsEnabled() {
		return
	}
	queue = make(chan *querytracer.OTLPTrace, queueCap)
	stopCh = make(chan struct{})
	wg.Add(1)
	go func() {
		defer wg.Done()
		runExporter()
	}()
	logger.Infof("started query trace exporter to %s", *pushURL)
}

// Stop drains pending traces and stops the background goroutine.
// Must be called during graceful shutdown before the process exits.
func Stop() {
	if !IsEnabled() {
		return
	}
	close(stopCh)
	wg.Wait()
}

// Push enqueues t for async export. t must be a finished tracer.
// Traces are silently dropped if the queue is full.
func Push(t *querytracer.Tracer) {
	if !t.Enabled() {
		return
	}
	tr := t.ToOTLPTrace()
	if tr == nil {
		return
	}
	select {
	case queue <- tr:
	default:
		droppedTotal.Inc()
	}
}

// runExporter reads from queue and sends batches to the configured endpoint.
func runExporter() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	var batch []*querytracer.OTLPTrace
	for {
		select {
		case tr := <-queue:
			batch = append(batch, tr)
			if len(batch) >= 100 {
				flushBatch(batch)
				batch = batch[:0]
			}
		case <-ticker.C:
			if len(batch) > 0 {
				flushBatch(batch)
				batch = batch[:0]
			}
		case <-stopCh:
			// Drain remaining items.
			for {
				select {
				case tr := <-queue:
					batch = append(batch, tr)
				default:
					if len(batch) > 0 {
						flushBatch(batch)
					}
					return
				}
			}
		}
	}
}

// flushBatch serializes and POSTs a batch of traces.
func flushBatch(batch []*querytracer.OTLPTrace) {
	data, err := marshalBatch(batch)
	if err != nil {
		logger.Errorf("cannot marshal trace batch: %s", err)
		errorsTotal.Inc()
		return
	}
	if err := postData(data); err != nil {
		logger.Warnf("cannot export query traces to %s: %s", *pushURL, err)
		errorsTotal.Inc()
		return
	}
	pushesTotal.Add(len(batch))
}

// marshalBatch packs all traces into a single ExportTraceServiceRequest and gzip-compresses it.
func marshalBatch(batch []*querytracer.OTLPTrace) ([]byte, error) {
	svcVersion := buildinfo.Version

	// Build resource attributes once (same for all spans in this process).
	resAttrs := []*keyValue{
		{Key: "service.name", Value: anyValue{StringValue: "victoriametrics"}},
		{Key: "service.version", Value: anyValue{StringValue: svcVersion}},
	}

	var pbSpans []*span
	for _, tr := range batch {
		for i := range tr.Spans {
			s := &tr.Spans[i]
			pbSpan := &span{
				TraceID:           s.TraceID,
				SpanID:            s.SpanID,
				ParentSpanID:      s.ParentSpanID,
				Name:              s.Name,
				StartTimeUnixNano: s.StartNano,
				EndTimeUnixNano:   s.EndNano,
			}
			for _, ev := range s.Events {
				pbSpan.Events = append(pbSpan.Events, &spanEvent{
					TimeUnixNano: ev.TimeNano,
					Name:         ev.Name,
				})
			}
			pbSpans = append(pbSpans, pbSpan)
		}
	}

	req := &exportTraceServiceRequest{
		ResourceSpans: []*resourceSpans{
			{
				Resource: resource{Attributes: resAttrs},
				ScopeSpans: []*scopeSpans{
					{
						Scope: instrumentationScope{
							Name:    "victoriametrics/querytracer",
							Version: svcVersion,
						},
						Spans: pbSpans,
					},
				},
			},
		},
	}

	raw := req.marshalProtobuf(nil)

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	if _, err := gz.Write(raw); err != nil {
		return nil, fmt.Errorf("cannot gzip trace data: %w", err)
	}
	if err := gz.Close(); err != nil {
		return nil, fmt.Errorf("cannot close gzip writer: %w", err)
	}
	return buf.Bytes(), nil
}

var httpClient = &http.Client{
	Timeout: 10 * time.Second,
}

// postData sends gzip-compressed protobuf data to the configured endpoint.
func postData(data []byte) error {
	req, err := http.NewRequest(http.MethodPost, *pushURL, bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("cannot create HTTP request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-protobuf")
	req.Header.Set("Content-Encoding", "gzip")

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode/100 != 2 {
		return fmt.Errorf("unexpected HTTP response status %d", resp.StatusCode)
	}
	return nil
}
