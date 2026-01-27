package streamaggr

import (
	"fmt"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutil"
	"github.com/VictoriaMetrics/metrics"
	"github.com/valyala/histogram"
)

// Deduplicator deduplicates samples per each time series.
type Deduplicator struct {
	da *dedupAggr

	cs            atomic.Pointer[currentState]
	enableWindows bool
	dropLabels    []string
	interval      time.Duration
	minDeadline   atomic.Int64

	wg     sync.WaitGroup
	stopCh chan struct{}

	ms *metrics.Set
	// time to wait after interval end before flush
	flushAfter   *histogram.Fast
	muFlushAfter sync.Mutex
}

// NewDeduplicator returns new deduplicator, which deduplicates samples per each time series.
//
// The de-duplicated samples are passed to pushFunc once per interval.
//
// An optional dropLabels list may contain label names, which must be dropped before de-duplicating samples.
// Common case is to drop `replica`-like labels from samples received from HA datasources.
//
// alias is url label used in metrics exposed by the returned Deduplicator.
//
// MustStop must be called on the returned deduplicator in order to free up occupied resources.
func NewDeduplicator(pushFunc PushFunc, enableWindows bool, interval time.Duration, dropLabels []string, alias string) *Deduplicator {
	d := &Deduplicator{
		da:            newDedupAggr(),
		dropLabels:    dropLabels,
		interval:      interval,
		enableWindows: enableWindows,
		stopCh:        make(chan struct{}),
		ms:            metrics.NewSet(),
	}
	startTime := time.Now()
	cs := &currentState{
		maxDeadline: startTime.Add(interval).UnixMilli(),
	}
	d.cs.Store(cs)
	if enableWindows {
		d.flushAfter = histogram.GetFast()
		d.minDeadline.Store(startTime.UnixMilli())
	}
	d.cs.Store(cs)

	ms := d.ms

	metricLabels := fmt.Sprintf(`name="dedup",url=%q`, alias)

	_ = ms.NewGauge(fmt.Sprintf(`vm_streamaggr_dedup_state_size_bytes{%s}`, metricLabels), func() float64 {
		return float64(d.da.sizeBytes())
	})
	_ = ms.NewGauge(fmt.Sprintf(`vm_streamaggr_dedup_state_items_count{%s}`, metricLabels), func() float64 {
		return float64(d.da.itemsCount())
	})

	d.da.flushDuration = ms.NewHistogram(fmt.Sprintf(`vm_streamaggr_dedup_flush_duration_seconds{%s}`, metricLabels))
	d.da.flushTimeouts = ms.NewCounter(fmt.Sprintf(`vm_streamaggr_dedup_flush_timeouts_total{%s}`, metricLabels))

	metrics.RegisterSet(ms)

	d.wg.Go(func() {
		d.runFlusher(pushFunc)
	})

	return d
}

// MustStop stops d.
func (d *Deduplicator) MustStop() {
	metrics.UnregisterSet(d.ms, true)
	d.ms = nil

	close(d.stopCh)
	d.wg.Wait()
}

// Push pushes tss to d.
func (d *Deduplicator) Push(tss []prompb.TimeSeries) {
	ctx := getDeduplicatorPushCtx()
	labels := &ctx.labels
	buf := ctx.buf
	cs := d.cs.Load()
	nowMsec := time.Now().UnixMilli()
	minDeadline := d.minDeadline.Load()
	var maxLagMsec int64

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
			if d.enableWindows && minDeadline > s.Timestamp {
				continue
			} else if d.enableWindows && s.Timestamp <= cs.maxDeadline == cs.isGreen {
				ctx.green = append(ctx.green, pushSample{
					key:       key,
					value:     s.Value,
					timestamp: s.Timestamp,
				})
			} else {
				ctx.blue = append(ctx.blue, pushSample{
					key:       key,
					value:     s.Value,
					timestamp: s.Timestamp,
				})
			}
			lagMsec := nowMsec - s.Timestamp
			if lagMsec > maxLagMsec {
				maxLagMsec = lagMsec
			}
		}
	}

	if d.enableWindows && maxLagMsec > 0 {
		d.muFlushAfter.Lock()
		d.flushAfter.Update(float64(maxLagMsec))
		d.muFlushAfter.Unlock()
	}

	if len(ctx.blue) > 0 {
		d.da.pushSamples(ctx.blue, 0, false)
	}
	if len(ctx.green) > 0 {
		d.da.pushSamples(ctx.green, 0, true)
	}

	ctx.buf = buf
	putDeduplicatorPushCtx(ctx)
}

func dropSeriesLabels(dst, src []prompb.Label, labelNames []string) []prompb.Label {
	for _, label := range src {
		if !slices.Contains(labelNames, label.Name) {
			dst = append(dst, label)
		}
	}
	return dst
}

func (d *Deduplicator) runFlusher(pushFunc PushFunc) {
	t := time.NewTicker(d.interval)
	var fa *histogram.Fast
	defer t.Stop()
	for {
		select {
		case <-d.stopCh:
			return
		case <-t.C:
			if d.enableWindows {
				// Calculate delay and wait
				d.muFlushAfter.Lock()
				fa, d.flushAfter = d.flushAfter, histogram.GetFast()
				d.muFlushAfter.Unlock()
				delay := time.Duration(fa.Quantile(flushQuantile)) * time.Millisecond
				histogram.PutFast(fa)
				time.Sleep(delay)
			}
			d.flush(pushFunc)
		}
	}
}

func (d *Deduplicator) flush(pushFunc PushFunc) {
	cs := d.cs.Load().newState()
	d.minDeadline.Store(cs.maxDeadline)
	startTime := time.Now()
	deadlineTime := time.UnixMilli(cs.maxDeadline)
	d.da.flush(func(samples []pushSample, _ int64, _ bool) {
		ctx := getDeduplicatorFlushCtx()

		tss := ctx.tss
		labels := ctx.labels
		dstSamples := ctx.samples
		for _, ps := range samples {
			labelsLen := len(labels)
			labels = decompressLabels(labels, ps.key)

			dstSamplesLen := len(dstSamples)
			dstSamples = append(dstSamples, prompb.Sample{
				Value:     ps.value,
				Timestamp: ps.timestamp,
			})

			tss = append(tss, prompb.TimeSeries{
				Labels:  labels[labelsLen:],
				Samples: dstSamples[dstSamplesLen:],
			})
		}
		pushFunc(tss)

		ctx.tss = tss
		ctx.labels = labels
		ctx.samples = dstSamples
		putDeduplicatorFlushCtx(ctx)
	}, cs.maxDeadline, cs.isGreen)

	duration := time.Since(startTime)
	d.da.flushDuration.Update(duration.Seconds())
	if duration > d.interval {
		d.da.flushTimeouts.Inc()
		logger.Warnf("deduplication couldn't be finished in the configured dedupInterval=%s; it took %.03fs; "+
			"possible solutions: increase dedupInterval; reduce samples' ingestion rate", d.interval, duration.Seconds())
	}
	for time.Now().After(deadlineTime) {
		deadlineTime = deadlineTime.Add(d.interval)
	}
	cs.maxDeadline = deadlineTime.UnixMilli()
	if d.enableWindows {
		cs.isGreen = !cs.isGreen
	}
	d.cs.Store(cs)
}

type deduplicatorPushCtx struct {
	blue   []pushSample
	green  []pushSample
	labels promutil.Labels
	buf    []byte
}

func (ctx *deduplicatorPushCtx) reset() {
	ctx.blue = ctx.blue[:0]
	ctx.green = ctx.green[:0]
	ctx.buf = ctx.buf[:0]
	ctx.labels.Reset()
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
	tss     []prompb.TimeSeries
	labels  []prompb.Label
	samples []prompb.Sample
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
