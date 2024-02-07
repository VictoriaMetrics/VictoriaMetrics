package streamaggr

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutils"
)

type pushSamplesFunc func(b []byte, labels *promutils.Labels, tmpLabels *promutils.Labels, value float64)

type deduplicator struct {
	interval       time.Duration
	wg             sync.WaitGroup
	stopCh         chan struct{}
	m              sync.Map
	pushSamplesAgg pushSamplesFunc
	bm             atomic.Pointer[bimap]
}

func newDeduplicator(
	dedupInterval time.Duration,
) *deduplicator {
	d := &deduplicator{
		interval: dedupInterval,
		stopCh:   make(chan struct{}),
	}
	d.bm.Store(bm.Load())
	return d
}

func (d *deduplicator) run(pushSamplesAgg pushSamplesFunc) {
	d.pushSamplesAgg = pushSamplesAgg
	d.wg.Add(1)
	go func() {
		defer d.wg.Done()
		t := time.NewTicker(d.interval)
		defer t.Stop()
		for {
			select {
			case <-d.stopCh:
				return
			case <-t.C:
			}
			d.flush()
		}
	}()
}

func (d *deduplicator) stop() {
	if d == nil {
		return
	}
	close(d.stopCh)
	d.wg.Wait()
	d.flush()
}

func (d *deduplicator) pushSamples(key []byte, labels []prompbmarshal.Label, ts prompbmarshal.TimeSeries) {
	b := d.bm.Load()
	for _, sample := range ts.Samples {
		key = b.compress(key[:0], labels)
		d.pushSample(string(key), sample.Value, sample.Timestamp)
	}
}

func (d *deduplicator) pushSample(key string, value float64, timestamp int64) {
again:
	v, ok := d.m.Load(key)
	if !ok {
		// The entry is missing in the map. Try creating it.
		v = &dedupStateValue{
			value:     value,
			timestamp: timestamp,
		}
		vNew, loaded := d.m.LoadOrStore(key, v)
		if !loaded {
			// The new entry has been successfully created.
			return
		}
		// Use the entry created by a concurrent goroutine.
		v = vNew
	}
	sv := v.(*dedupStateValue)
	sv.mu.Lock()
	deleted := sv.deleted
	if !deleted {
		if timestamp > sv.timestamp ||
			(timestamp == sv.timestamp && value > sv.value) {
			sv.value = value
			sv.timestamp = timestamp
		}
	}
	sv.mu.Unlock()
	if deleted {
		// The entry has been deleted by the concurrent call to appendSeriesForFlush
		// Try obtaining and updating the entry again.
		goto again
	}
}

func (d *deduplicator) flush() {
	if d == nil {
		return
	}

	labels := promutils.GetLabels()
	tmpLabels := promutils.GetLabels()
	bb := bbPool.Get()

	obm := d.bm.Swap(bm.Load())
	d.m.Range(func(k, v interface{}) bool {
		// Atomically delete the entry from the map, so new entry is created for the next flush.
		d.m.Delete(k)

		sv := v.(*dedupStateValue)
		sv.mu.Lock()
		value := sv.value
		// Mark the entry as deleted, so it won't be updated anymore by concurrent pushSample() calls.
		sv.deleted = true
		sv.mu.Unlock()

		labels.Labels = labels.Labels[:0]
		labels = obm.decompress(labels, k.(string))

		//key, err := zstdDecoder.DecodeAll([]byte(k.(string)), nil)
		//if err != nil {
		//	panic(err)
		//}

		d.pushSamplesAgg(bb.B[:0], labels, tmpLabels, value)
		return true
	})

	bbPool.Put(bb)
	promutils.PutLabels(tmpLabels)
	promutils.PutLabels(labels)
}

type dedupStateValue struct {
	mu        sync.Mutex
	timestamp int64
	value     float64
	deleted   bool
}
