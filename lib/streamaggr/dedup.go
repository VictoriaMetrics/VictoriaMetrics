package streamaggr

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cespare/xxhash/v2"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/cgroup"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
)

// deduplicator accepts incoming time series via push function
// and once per interval it passes deduplicated time series to callback.
// Deduplication logic is aligned with https://docs.victoriametrics.com/#deduplication
type deduplicator struct {
	interval time.Duration
	wg       sync.WaitGroup
	stopCh   chan struct{}

	callback pushAggrFn

	sm atomic.Pointer[shardedMap]

	processShardDuration *summary
	callbackDuration     *summary
	flushDuration        *summary
	// aggregation correctness will be compromised if flush takes longer than interval
	flushDurationExceeded *counter
}

func newDeduplicator(aggregator string, interval time.Duration, cb pushAggrFn) *deduplicator {
	d := &deduplicator{
		interval: interval,
		stopCh:   make(chan struct{}),
		callback: cb,

		processShardDuration:  getOrCreateSummary(fmt.Sprintf(`vmagent_streamaggr_dedup_process_shard_duration{aggregator=%q}`, aggregator)),
		callbackDuration:      getOrCreateSummary(fmt.Sprintf(`vmagent_streamaggr_dedup_callback_duration{aggregator=%q}`, aggregator)),
		flushDuration:         getOrCreateSummary(fmt.Sprintf(`vmagent_streamaggr_dedup_duration_seconds{aggregator=%q}`, aggregator)),
		flushDurationExceeded: getOrCreateCounter(fmt.Sprintf(`vmagent_streamaggr_dedup_duration_exceeds_aggregation_interval_total{aggregator=%q}`, aggregator)),
	}
	d.sm.Store(newShardedMap())
	return d
}

func (d *deduplicator) unregisterMetrics() {
	d.processShardDuration.unregister()
	d.callbackDuration.unregister()
	d.flushDuration.unregister()
	d.flushDurationExceeded.unregister()
}

func (d *deduplicator) run() {
	if d == nil {
		return
	}

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
			start := time.Now()
			d.flush()
			d.flushDuration.UpdateDuration(start)
			if time.Since(start) > d.interval {
				d.flushDurationExceeded.Inc()
			}
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

func (d *deduplicator) push(tss []encodedTss) {
	for _, ts := range tss {
		if len(ts.samples) == 0 {
			continue
		}

		lastSample := ts.samples[0]
		for i, sample := range ts.samples[1:] {
			if sample.Timestamp > lastSample.Timestamp ||
				(sample.Timestamp == lastSample.Timestamp && sample.Value > lastSample.Value) {
				lastSample = ts.samples[i]
			}
		}

	again:
		// TODO: acquire snapshot of the current encoder object
		// to ensure we use associated versions of shardedMap and labelsEncoder.
		// The version of shardedMap and labelsEncoder are updated on each flush call.
		key := ts.labels
		sm := d.sm.Load()
		s := sm.getShard(key)

		s.mu.Lock()
		if s.drained {
			// the shard was already drained and disposed during the flush,
			// try to get a new shard instead.
			s.mu.Unlock()
			goto again
		}

		dsv, ok := s.data[key]
		if !ok {
			dsv = lastSample
		}
		// verify if new sample is newer than currently saved sample
		if lastSample.Timestamp > dsv.Timestamp ||
			(lastSample.Timestamp == dsv.Timestamp && lastSample.Value > dsv.Value) {
			dsv = lastSample
		}
		s.data[key] = dsv
		s.mu.Unlock()
	}
}

func (d *deduplicator) flush() {
	sm := d.sm.Swap(newShardedMap())

	var tss []encodedTss
	var samples []prompbmarshal.Sample
	for _, s := range sm.shards {
		startProcess := time.Now()
		samples = samples[:0]

		s.mu.Lock()
		if cap(tss) < len(s.data) {
			tss = make([]encodedTss, 0, len(s.data))
		}
		tss = tss[:len(s.data)]

		var i int
		for k, v := range s.data {
			ts := &tss[i]
			i++

			ts.labels = k
			samplesLen := len(samples)
			samples = append(samples, prompbmarshal.Sample{})
			sample := &samples[len(samples)-1]
			sample.Value = v.Value
			sample.Timestamp = v.Timestamp
			ts.samples = samples[samplesLen:]
		}
		// mark shard as drained, so goroutines which concurrently execute push
		// could do a retry.
		s.drained = true
		s.mu.Unlock()
		d.processShardDuration.UpdateDuration(startProcess)

		start := time.Now()
		d.callback(tss)
		d.callbackDuration.UpdateDuration(start)
	}
}

type shardedMap struct {
	shards []*shard
}

type shard struct {
	mu      sync.Mutex
	data    map[string]prompbmarshal.Sample
	drained bool
}

func newShardedMap() *shardedMap {
	cpusCount := cgroup.AvailableCPUs()
	shardsCount := cgroup.AvailableCPUs()
	// Increase the number of shards with the increased number of available CPU cores.
	// This should reduce contention on per-shard mutexes.
	multiplier := cpusCount
	if multiplier > 16 {
		multiplier = 16
	}
	shardsCount *= multiplier
	shards := make([]*shard, shardsCount)
	for i := range shards {
		shards[i] = &shard{
			data: make(map[string]prompbmarshal.Sample),
		}
	}
	return &shardedMap{shards: shards}
}

func (sm *shardedMap) getShard(key string) *shard {
	idx := uint64(0)
	if len(sm.shards) > 1 {
		h := hashUint64(key)
		idx = h % uint64(len(sm.shards))
	}
	return sm.shards[idx]
}

func hashUint64(s string) uint64 {
	b := bytesutil.ToUnsafeBytes(s)
	return xxhash.Sum64(b)
}
