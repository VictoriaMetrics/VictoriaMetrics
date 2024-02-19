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
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutils"
	"github.com/VictoriaMetrics/metrics"
)

type deduplicator struct {
	// list of aggregators where to flush data after deduplication
	// once per interval
	as       []*aggregator
	interval time.Duration
	wg       sync.WaitGroup
	stopCh   chan struct{}

	ddr atomic.Pointer[dedupRegistry]
}

type dedupRegistry struct {
	sm *shardedMap
	bm *bimap
}

func newDeduplicator(as []*aggregator, dedupInterval time.Duration) *deduplicator {
	d := &deduplicator{
		as:       as,
		interval: dedupInterval,
		stopCh:   make(chan struct{}),
	}
	ddr := &dedupRegistry{
		sm: newShardedMap(),
		bm: &bimap{},
	}
	d.ddr.Store(ddr)
	return d
}

func (d *deduplicator) run() {
	d.wg.Add(1)
	flushDuration := metrics.GetOrCreateSummary(fmt.Sprintf(`vmagent_streamaggr_dedup_duration_seconds{interval="%s"}`, d.interval))
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
			flushDuration.UpdateDuration(start)
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

func (d *deduplicator) pushSamples(key []byte, ts prompbmarshal.TimeSeries) {
	lastSample := ts.Samples[0]
	// find the most recent sample, since previous samples will be deduplicated anyway
	for i, sample := range ts.Samples {
		if sample.Timestamp > lastSample.Timestamp ||
			(sample.Timestamp == lastSample.Timestamp && sample.Value > lastSample.Value) {
			lastSample = ts.Samples[i]
		}
	}

again:
	ddr := d.ddr.Load()
	key = ddr.bm.compress(key[:0], ts.Labels)
	sKey := string(key)

	s := ddr.sm.getShard(sKey)
	s.mu.Lock()
	if s.drained {
		s.mu.Unlock()
		goto again
	}

	dsv, ok := s.data[sKey]
	if !ok {
		// The entry is missing in the map. Try creating it.
		dsv = dedupStateValue{
			value:     lastSample.Value,
			timestamp: lastSample.Timestamp,
		}
	}
	// verify if new sample is newer than currently saved sample
	if lastSample.Timestamp > dsv.timestamp ||
		(lastSample.Timestamp == dsv.timestamp && lastSample.Value > dsv.value) {
		dsv.value = lastSample.Value
		dsv.timestamp = lastSample.Timestamp
	}
	s.data[sKey] = dsv
	s.mu.Unlock()
}

func (d *deduplicator) flush() {
	if d == nil {
		return
	}

	newDdr := &dedupRegistry{
		sm: newShardedMap(),
		bm: &bimap{},
	}
	ddr := d.ddr.Swap(newDdr)

	labels := promutils.GetLabels()
	tmpLabels := promutils.GetLabels()
	bb := bbPool.Get()

	type sample struct {
		key string
		dsv dedupStateValue
	}
	samples := make([]sample, 0)
	for _, sh := range ddr.sm.shards {
		samples = samples[:0]

		sh.mu.Lock()
		// collect data before processing to release the lock ASAP
		for k, v := range sh.data {
			samples = append(samples, sample{
				key: k,
				dsv: v,
			})
		}
		sh.drained = true
		sh.mu.Unlock()

		for _, s := range samples {
			labels.Labels = labels.Labels[:0]
			labels = ddr.bm.decompress(labels, s.key)
			for _, a := range d.as {
				if !a.match.Match(labels.Labels) {
					return
				}

				// TODO: this mutates labels, need to make sure it doesn't corrupt for other a
				labels.Labels = a.inputRelabeling.Apply(labels.Labels, 0)
				if len(labels.Labels) == 0 {
					// The metric has been deleted by the relabeling
					return
				}
				// sort labels so they can be comparable during deduplication
				labels.Sort()

				inputKey, outputKey := a.extractKeys(bb.B[:0], labels, tmpLabels)
				a.pushSample(inputKey, outputKey, s.dsv.value)
			}
		}
	}

	bbPool.Put(bb)
	promutils.PutLabels(tmpLabels)
	promutils.PutLabels(labels)
}

type dedupStateValue struct {
	timestamp int64
	value     float64
}

type shardedMap struct {
	shards []*shard
}

type shard struct {
	mu      sync.Mutex
	data    map[string]dedupStateValue
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
			data: make(map[string]dedupStateValue),
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
