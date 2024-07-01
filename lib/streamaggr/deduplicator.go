package streamaggr

import (
	"fmt"
	"slices"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutils"
	"github.com/VictoriaMetrics/metrics"
)

// Deduplicator deduplicates samples per each time series.
type Deduplicator struct {
	da *dedupAggr

	dropLabels []string

	wg     sync.WaitGroup
	stopCh chan struct{}

	ms *metrics.Set

	dedupFlushDuration *metrics.Histogram
	dedupFlushTimeouts *metrics.Counter
}

// NewDeduplicator returns new deduplicator, which deduplicates samples per each time series.
//
// The de-duplicated samples are passed to pushFunc once per dedupInterval.
//
// An optional dropLabels list may contain label names, which must be dropped before de-duplicating samples.
// Common case is to drop `replica`-like labels from samples received from HA datasources.
//
// MustStop must be called on the returned deduplicator in order to free up occupied resources.
func NewDeduplicator(pushFunc PushFunc, dedupInterval time.Duration, dropLabels []string, alias string) *Deduplicator {
	d := &Deduplicator{
		da:         newDedupAggr(),
		dropLabels: dropLabels,

		stopCh: make(chan struct{}),
		ms:     metrics.NewSet(),
	}

	ms := d.ms

	metricLabels := fmt.Sprintf(`url=%q`, alias)
	_ = ms.NewGauge(fmt.Sprintf(`vm_streamaggr_dedup_state_size_bytes{%s}`, metricLabels), func() float64 {
		return float64(d.da.sizeBytes())
	})
	_ = ms.NewGauge(fmt.Sprintf(`vm_streamaggr_dedup_state_items_count{%s}`, metricLabels), func() float64 {
		return float64(d.da.itemsCount())
	})

	d.dedupFlushDuration = ms.GetOrCreateHistogram(fmt.Sprintf(`vm_streamaggr_dedup_flush_duration_seconds{%s}`, metricLabels))
	d.dedupFlushTimeouts = ms.GetOrCreateCounter(fmt.Sprintf(`vm_streamaggr_dedup_flush_timeouts_total{%s}`, metricLabels))

	metrics.RegisterSet(ms)

	d.wg.Add(1)
	go func() {
		defer d.wg.Done()
		d.runFlusher(pushFunc, dedupInterval)
	}()

	return d
}

// MustStop stops d.
func (d *Deduplicator) MustStop() {
	metrics.UnregisterSet(d.ms)
	d.ms = nil

	close(d.stopCh)
	d.wg.Wait()
}

// Push pushes tss to d.
func (d *Deduplicator) Push(tss []prompbmarshal.TimeSeries) {
	ctx := getDeduplicatorPushCtx()
	pss := ctx.pss
	labels := &ctx.labels
	buf := ctx.buf

	dropLabels := d.dropLabels
	for _, ts := range tss {
		if len(dropLabels) > 0 {
			labels.Labels = dropSeriesLabels(labels.Labels[:0], ts.Labels, dropLabels)
		} else {
			labels.Labels = append(labels.Labels[:0], ts.Labels...)
		}
		if len(labels.Labels) == 0 {
			continue
		}
		labels.Sort()

		bufLen := len(buf)
		buf = lc.Compress(buf, labels.Labels)
		key := bytesutil.ToUnsafeString(buf[bufLen:])
		for _, s := range ts.Samples {
			pss = append(pss, pushSample{
				key:       key,
				value:     s.Value,
				timestamp: s.Timestamp,
			})
		}
	}

	d.da.pushSamples(pss)

	ctx.pss = pss
	ctx.buf = buf
	putDeduplicatorPushCtx(ctx)
}

func dropSeriesLabels(dst, src []prompbmarshal.Label, labelNames []string) []prompbmarshal.Label {
	for _, label := range src {
		if !slices.Contains(labelNames, label.Name) {
			dst = append(dst, label)
		}
	}
	return dst
}

func (d *Deduplicator) runFlusher(pushFunc PushFunc, dedupInterval time.Duration) {
	t := time.NewTicker(dedupInterval)
	defer t.Stop()
	for {
		select {
		case <-d.stopCh:
			return
		case <-t.C:
			d.flush(pushFunc, dedupInterval)
		}
	}
}

func (d *Deduplicator) flush(pushFunc PushFunc, dedupInterval time.Duration) {
	startTime := time.Now()

	timestamp := startTime.UnixMilli()
	d.da.flush(func(pss []pushSample) {
		ctx := getDeduplicatorFlushCtx()

		tss := ctx.tss
		labels := ctx.labels
		samples := ctx.samples
		for _, ps := range pss {
			labelsLen := len(labels)
			labels = decompressLabels(labels, ps.key)

			samplesLen := len(samples)
			samples = append(samples, prompbmarshal.Sample{
				Value:     ps.value,
				Timestamp: timestamp,
			})

			tss = append(tss, prompbmarshal.TimeSeries{
				Labels:  labels[labelsLen:],
				Samples: samples[samplesLen:],
			})
		}
		pushFunc(tss)

		ctx.tss = tss
		ctx.labels = labels
		ctx.samples = samples
		putDeduplicatorFlushCtx(ctx)
	})

	duration := time.Since(startTime)
	d.dedupFlushDuration.Update(duration.Seconds())
	if duration > dedupInterval {
		d.dedupFlushTimeouts.Inc()
		logger.Warnf("deduplication couldn't be finished in the configured dedupInterval=%s; it took %.03fs; "+
			"possible solutions: increase dedupInterval; reduce samples' ingestion rate", dedupInterval, duration.Seconds())
	}

}

type deduplicatorPushCtx struct {
	pss    []pushSample
	labels promutils.Labels
	buf    []byte
}

func (ctx *deduplicatorPushCtx) reset() {
	clear(ctx.pss)
	ctx.pss = ctx.pss[:0]

	ctx.labels.Reset()

	ctx.buf = ctx.buf[:0]
}

func getDeduplicatorPushCtx() *deduplicatorPushCtx {
	v := deduplicatorPushCtxPool.Get()
	if v == nil {
		return &deduplicatorPushCtx{}
	}
	return v.(*deduplicatorPushCtx)
}

func putDeduplicatorPushCtx(ctx *deduplicatorPushCtx) {
	ctx.reset()
	deduplicatorPushCtxPool.Put(ctx)
}

var deduplicatorPushCtxPool sync.Pool

type deduplicatorFlushCtx struct {
	tss     []prompbmarshal.TimeSeries
	labels  []prompbmarshal.Label
	samples []prompbmarshal.Sample
}

func (ctx *deduplicatorFlushCtx) reset() {
	clear(ctx.tss)
	ctx.tss = ctx.tss[:0]

	clear(ctx.labels)
	ctx.labels = ctx.labels[:0]

	clear(ctx.samples)
	ctx.samples = ctx.samples[:0]
}

func getDeduplicatorFlushCtx() *deduplicatorFlushCtx {
	v := deduplicatorFlushCtxPool.Get()
	if v == nil {
		return &deduplicatorFlushCtx{}
	}
	return v.(*deduplicatorFlushCtx)
}

func putDeduplicatorFlushCtx(ctx *deduplicatorFlushCtx) {
	ctx.reset()
	deduplicatorFlushCtxPool.Put(ctx)
}

var deduplicatorFlushCtxPool sync.Pool
