package streamaggr

import (
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutils"
	"github.com/VictoriaMetrics/metrics"
)

// Deduplicator deduplicates samples per each time series.
type Deduplicator struct {
	da *dedupAggr
	lc promutils.LabelsCompressor

	wg     sync.WaitGroup
	stopCh chan struct{}

	ms *metrics.Set
}

// NewDeduplicator returns new deduplicator, which deduplicates samples per each time series.
//
// The de-duplicated samples are passed to pushFunc once per dedupInterval.
//
// MustStop must be called on the returned deduplicator in order to free up occupied resources.
func NewDeduplicator(pushFunc PushFunc, dedupInterval time.Duration) *Deduplicator {
	d := &Deduplicator{
		da:     newDedupAggr(),
		stopCh: make(chan struct{}),
		ms:     metrics.NewSet(),
	}

	ms := d.ms
	_ = ms.NewGauge(`vm_streamaggr_dedup_state_size_bytes`, func() float64 {
		return float64(d.da.sizeBytes())
	})
	_ = ms.NewGauge(`vm_streamaggr_dedup_state_items_count`, func() float64 {
		return float64(d.da.itemsCount())
	})

	_ = ms.NewGauge(`vm_streamaggr_labels_compressor_size_bytes`, func() float64 {
		return float64(d.lc.SizeBytes())
	})
	_ = ms.NewGauge(`vm_streamaggr_labels_compressor_items_count`, func() float64 {
		return float64(d.lc.ItemsCount())
	})
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
	buf := ctx.buf

	for _, ts := range tss {
		buf = d.lc.Compress(buf[:0], ts.Labels)
		key := bytesutil.InternBytes(buf)
		for _, s := range ts.Samples {
			pss = append(pss, pushSample{
				key:   key,
				value: s.Value,
			})
		}
	}

	d.da.pushSamples(pss)

	ctx.pss = pss
	ctx.buf = buf
	putDeduplicatorPushCtx(ctx)
}

func (d *Deduplicator) runFlusher(pushFunc PushFunc, dedupInterval time.Duration) {
	t := time.NewTicker(dedupInterval)
	defer t.Stop()
	for {
		select {
		case <-d.stopCh:
			return
		case <-t.C:
			d.flush(pushFunc)
		}
	}
}

func (d *Deduplicator) flush(pushFunc PushFunc) {
	timestamp := time.Now().UnixMilli()
	d.da.flush(func(pss []pushSample) {
		ctx := getDeduplicatorFlushCtx()

		tss := ctx.tss
		labels := ctx.labels
		samples := ctx.samples
		for _, ps := range pss {
			labelsLen := len(labels)
			labels = decompressLabels(labels, &d.lc, ps.key)

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
	}, true)
}

type deduplicatorPushCtx struct {
	pss []pushSample
	buf []byte
}

func (ctx *deduplicatorPushCtx) reset() {
	clear(ctx.pss)
	ctx.pss = ctx.pss[:0]

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
