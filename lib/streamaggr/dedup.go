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

type pushSamplesFunc func(b []byte, labels *promutils.Labels, tmpLabels *promutils.Labels, value float64)

type deduplicator struct {
	interval       time.Duration
	wg             sync.WaitGroup
	stopCh         chan struct{}
	m              *shardedMap
	pushSamplesAgg pushSamplesFunc
	bm             atomic.Pointer[bimap]
}

func newDeduplicator(
	dedupInterval time.Duration,
) *deduplicator {
	d := &deduplicator{
		interval: dedupInterval,
		stopCh:   make(chan struct{}),
		m:        newShardedMap(),
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
	sv, ok := d.m.load(key)
	if !ok {
		// The entry is missing in the map. Try creating it.
		sv = &dedupStateValue{
			value:     value,
			timestamp: timestamp,
		}
		vNew, loaded := d.m.loadOrStore(key, sv)
		if !loaded {
			// The new entry has been successfully created.
			return
		}
		// Use the entry created by a concurrent goroutine.
		sv = vNew
	}
	sv.mu.Lock()
	if timestamp > sv.timestamp ||
		(timestamp == sv.timestamp && value > sv.value) {
		sv.value = value
		sv.timestamp = timestamp
	}
	sv.mu.Unlock()
}

func (d *deduplicator) flush() {
	if d == nil {
		return
	}

	labels := promutils.GetLabels()
	tmpLabels := promutils.GetLabels()
	bb := bbPool.Get()

	obm := d.bm.Swap(bm.Load())
	d.m.rangeAndDelete(func(k string, v *dedupStateValue) bool {
		value := v.value
		labels.Labels = labels.Labels[:0]
		labels = obm.decompress(labels, k)

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
}

type shardedMap struct {
	shards []*shard
}

type shard struct {
	mu   sync.Mutex
	data map[string]*dedupStateValue
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
			data: make(map[string]*dedupStateValue, 0),
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

func (sm *shardedMap) load(key string) (*dedupStateValue, bool) {
	s := sm.getShard(key)
	return s.getValue(key)
}

func (sm *shardedMap) loadOrStore(key string, value *dedupStateValue) (*dedupStateValue, bool) {
	s := sm.getShard(key)
	return s.loadOrStore(key, value)
}

func (sm *shardedMap) delete(key string) {
	s := sm.getShard(key)
	s.delete(key)
}

func (sm *shardedMap) rangeAndDelete(f func(k string, v *dedupStateValue) bool) {
	for _, s := range sm.shards {
		s.rangeAndDelete(f)
	}
}

func (s *shard) getValue(k string) (*dedupStateValue, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.data[k]
	return v, ok
}

func (s *shard) loadOrStore(k string, v *dedupStateValue) (*dedupStateValue, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	vv, ok := s.data[k]
	if ok {
		return vv, true
	}
	s.data[k] = v
	return v, false
}

func (s *shard) delete(k string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, k)
}

func (s *shard) rangeAndDelete(f func(k string, v *dedupStateValue) bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for k, v := range s.data {
		v.mu.Lock()
		delete(s.data, k)
		if !f(k, v) {
			v.mu.Unlock()
			return
		}
		v.mu.Unlock()
	}
}

func hashUint64(s string) uint64 {
	b := bytesutil.ToUnsafeBytes(s)
	return xxhash.Sum64(b)
}
