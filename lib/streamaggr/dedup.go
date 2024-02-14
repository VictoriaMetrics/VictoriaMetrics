package streamaggr

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/cespare/xxhash/v2"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/cgroup"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutils"
)

type pushSamplesFunc func(b []byte, labels *promutils.Labels, tmpLabels *promutils.Labels, value float64, ts int64)

type deduplicator struct {
	interval time.Duration
	wg       sync.WaitGroup
	stopCh   chan struct{}

	pushSamplesAgg pushSamplesFunc
	ddr            atomic.Pointer[dedupRegistry]
}

type dedupRegistry struct {
	sm *shardedMap
	bm *bimap
}

func newDeduplicator(
	dedupInterval time.Duration,
) *deduplicator {
	d := &deduplicator{
		interval: dedupInterval,
		stopCh:   make(chan struct{}),
	}
	ddr := &dedupRegistry{
		sm: newShardedMap(),
		bm: bm.Load(),
	}
	d.ddr.Store(ddr)
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
	for _, sample := range ts.Samples {
		ddr := d.ddr.Load()
		key = ddr.bm.compress(key[:0], labels)
		sKey := string(key)

		s := ddr.sm.getShard(sKey)
		s.mu.Lock()

		sv, ok := s.data[sKey]
		if !ok {
			// The entry is missing in the map. Try creating it.
			sv = dedupStateValue{
				value:     sample.Value,
				timestamp: sample.Timestamp,
			}
		}
		if sample.Timestamp > sv.timestamp ||
			(sample.Timestamp == sv.timestamp && sample.Value > sv.value) {
			sv.value = sample.Value
			sv.timestamp = sample.Timestamp
		}
		s.data[sKey] = sv

		s.mu.Unlock()
	}
}

func (d *deduplicator) flush() {
	if d == nil {
		return
	}

	labels := promutils.GetLabels()
	tmpLabels := promutils.GetLabels()
	bb := bbPool.Get()

	ddr := &dedupRegistry{
		sm: newShardedMap(),
		bm: bm.Load(),
	}
	oddr := d.ddr.Swap(ddr)
	for _, sh := range oddr.sm.shards {
		sh.mu.Lock()
		for k, v := range sh.data {
			value := v.value
			ts := v.timestamp

			labels.Labels = labels.Labels[:0]
			labels = oddr.bm.decompress(labels, k)

			d.pushSamplesAgg(bb.B[:0], labels, tmpLabels, value, ts)
		}
		sh.mu.Unlock()
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
	mu   sync.Mutex
	data map[string]dedupStateValue
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
