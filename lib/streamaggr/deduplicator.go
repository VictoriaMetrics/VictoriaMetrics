package streamaggr

import (
	"fmt"
	"slices"
	"sync"
	"sync/atomic"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutils"
	"github.com/VictoriaMetrics/metrics"
	"github.com/valyala/histogram"
)

// Deduplicator deduplicates samples per each time series.
type Deduplicator struct {
	da *dedupAggr

	current       atomic.Pointer[currentState]
	enableWindows bool
	dropLabels    []string
	interval      time.Duration
	minDeadline   atomic.Int64

	wg     sync.WaitGroup
	stopCh chan struct{}

	ms *metrics.Set

	// time to wait after interval end before flush
	flushAfter atomic.Pointer[histogram.Fast]

	flushDuration *metrics.Histogram
	flushTimeouts *metrics.Counter
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
	current := &currentState{
		deadline: startTime.Add(interval).UnixMilli(),
	}
	d.current.Store(current)
	d.flushAfter.Store(histogram.GetFast())
	if enableWindows {
		d.minDeadline.Store(startTime.UnixMilli())
	}

	ms := d.ms

	metricLabels := fmt.Sprintf(`name="dedup",url=%q`, alias)

	_ = ms.NewGauge(fmt.Sprintf(`vm_streamaggr_dedup_state_size_bytes{%s}`, metricLabels), func() float64 {
		return float64(d.da.sizeBytes())
	})
	_ = ms.NewGauge(fmt.Sprintf(`vm_streamaggr_dedup_state_items_count{%s}`, metricLabels), func() float64 {
		return float64(d.da.itemsCount())
	})

	d.flushDuration = ms.NewHistogram(fmt.Sprintf(`vm_streamaggr_dedup_flush_duration_seconds{%s}`, metricLabels))
	d.flushTimeouts = ms.NewCounter(fmt.Sprintf(`vm_streamaggr_dedup_flush_timeouts_total{%s}`, metricLabels))

	metrics.RegisterSet(ms)

	d.wg.Add(1)
	go func() {
		defer d.wg.Done()
		d.runFlusher(pushFunc)
	}()

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
func (d *Deduplicator) Push(tss []prompbmarshal.TimeSeries) {
	ctx := getDeduplicatorPushCtx()
	labels := &ctx.labels
	buf := ctx.buf
	current := d.current.Load()
	minDeadline := d.minDeadline.Load()
	nowMsec := time.Now().UnixMilli()
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
			} else if d.enableWindows && s.Timestamp <= current.deadline == current.isGreen {
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

	if d.enableWindows {
		d.flushAfter.Load().Update(float64(maxLagMsec))
	}

	if ctx.blue != nil {
		d.da.pushSamples(ctx.blue, 0, false)
	}
	if ctx.green != nil {
		d.da.pushSamples(ctx.green, 0, true)
	}

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

func (d *Deduplicator) runFlusher(pushFunc PushFunc) {
	t := time.NewTicker(d.interval)
	defer t.Stop()
	for {
		select {
		case <-d.stopCh:
			return
		case <-t.C:
			if d.enableWindows {
				// Calculate delay and wait
				fa := d.flushAfter.Swap(histogram.GetFast())
				flushAfter := time.Duration(fa.Quantile(0.95)) * time.Millisecond
				histogram.PutFast(fa)
				time.Sleep(flushAfter)
			}
			d.flush(pushFunc)
		}
	}
}

func (d *Deduplicator) flush(pushFunc PushFunc) {
	startTime := time.Now()
	current := d.current.Load()
	deadlineTime := time.UnixMilli(current.deadline)
	d.minDeadline.Store(current.deadline)
	d.da.flush(func(samples []pushSample, _ int64, _ bool) {
		ctx := getDeduplicatorFlushCtx()

		tss := ctx.tss
		labels := ctx.labels
		dstSamples := ctx.samples
		for _, ps := range samples {
			labelsLen := len(labels)
			labels = decompressLabels(labels, ps.key)

			dstSamplesLen := len(dstSamples)
			dstSamples = append(dstSamples, prompbmarshal.Sample{
				Value:     ps.value,
				Timestamp: ps.timestamp,
			})

			tss = append(tss, prompbmarshal.TimeSeries{
				Labels:  labels[labelsLen:],
				Samples: dstSamples[dstSamplesLen:],
			})
		}
		pushFunc(tss)

		ctx.tss = tss
		ctx.labels = labels
		ctx.samples = dstSamples
		putDeduplicatorFlushCtx(ctx)
	}, current.deadline, current.isGreen)

	for time.Now().After(deadlineTime) {
		deadlineTime = deadlineTime.Add(d.interval)
	}
	current.deadline = deadlineTime.UnixMilli()
	d.current.Store(current)

	duration := time.Since(startTime)
	d.flushDuration.Update(duration.Seconds())
	if duration > d.interval {
		d.flushTimeouts.Inc()
		logger.Warnf("deduplication couldn't be finished in the configured dedupInterval=%s; it took %.03fs; "+
			"possible solutions: increase dedupInterval; reduce samples' ingestion rate", d.interval, duration.Seconds())
	}

}

type deduplicatorPushCtx struct {
	blue   []pushSample
	green  []pushSample
	labels promutils.Labels
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
