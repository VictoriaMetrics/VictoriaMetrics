package streamaggr

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/VictoriaMetrics/metrics"
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

	// encoder encodes/decodes prompbmarshal.Label into a string key
	// and vice-versa. On each flush, encoder is replaced with new empty
	// encoder to avoid memory usage growth.
	encoder atomic.Pointer[dedupEncoder]
}

// dedupEncoder represents a snapshot of shardedMap and associated labelsEncoder.
// It is assumed that all entries in shardedMap object were encoded and could be decoded
// via this specific labelsEncoder object.
type dedupEncoder struct {
	sm *shardedMap
	le *labelsEncoder
}

func newDeduplicator(interval time.Duration, cb pushAggrFn) *deduplicator {
	d := &deduplicator{
		interval: interval,
		stopCh:   make(chan struct{}),
		callback: cb,
	}
	encoder := &dedupEncoder{
		sm: newShardedMap(),
		le: &labelsEncoder{},
	}
	d.encoder.Store(encoder)
	return d
}

func (d *deduplicator) run() {
	if d == nil {
		return
	}

	d.wg.Add(1)
	// aggregation correctness will be compromised if flush takes longer than interval
	flushDuration := metrics.GetOrCreateSummary(fmt.Sprintf(`vmagent_streamaggr_dedup_duration_seconds{interval="%s"}`, d.interval))
	flushDurationExceeded := metrics.GetOrCreateCounter(fmt.Sprintf(`vmagent_streamaggr_dedup_duration_exceeds_aggregation_interval_total{interval="%s"}`, d.interval))
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
			if time.Since(start) > d.interval {
				flushDurationExceeded.Inc()
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

func (d *deduplicator) push(ts prompbmarshal.TimeSeries) {
	if len(ts.Samples) == 0 {
		panic(fmt.Sprintf("received time series with 0 samples: %v", ts))
	}

	lastSample := ts.Samples[0]
	// if there are more than 1 sample, then find the most recent sample
	// as previous samples will be deduplicated anyway
	for i, sample := range ts.Samples[1:] {
		if sample.Timestamp > lastSample.Timestamp ||
			(sample.Timestamp == lastSample.Timestamp && sample.Value > lastSample.Value) {
			lastSample = ts.Samples[i]
		}
	}

	bb := bbPool.Get()
	defer bbPool.Put(bb)

again:
	// acquire snapshot of the current encoder object
	// to ensure we use associated versions of shardedMap and labelsEncoder.
	// The version of shardedMap and labelsEncoder are updated on each flush call.
	de := d.encoder.Load()

	bb.B = de.le.encode(bb.B[:0], ts.Labels)
	key := bytesutil.ToUnsafeString(bb.B)

	s := de.sm.getShard(key)
	s.mu.Lock()
	if s.drained {
		// the shard was already drained and disposed during the flush,
		// try to get a new shard instead.
		s.mu.Unlock()
		goto again
	}

	dsv, ok := s.data[key]
	if !ok {
		// the entry is missing in the map - try creating it
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
	// safely convert bb.B to string, as bb can be reused by other goroutines
	s.data[string(bb.B)] = dsv
	s.mu.Unlock()
}

var decodeErrors = metrics.GetOrCreateCounter(`vmagent_streamaggr_dedup_decode_errors_total`)

func (d *deduplicator) flush() {
	newEncoder := &dedupEncoder{
		sm: newShardedMap(),
		le: &labelsEncoder{},
	}

	// atomically replace encoder snapshot with empty encoder:
	// 1. acquired encoder snapshot will be used to decode labels during flush.
	//    It shouldn't be used after this function ends.
	// 2. empty encoder replaces the previous encoder to keep memory usage under control.
	encoder := d.encoder.Swap(newEncoder)

	// limit number of concurrently executing processShard
	// to avoid excessive memory usage during the flush
	syncCh := make(chan struct{}, cgroup.AvailableCPUs())
	processShard := func(s *shard) {
		syncCh <- struct{}{}
		defer func() {
			<-syncCh
		}()

		ptr := getTimeSeries()
		tss := *ptr

		s.mu.Lock()
		if cap(tss) < len(s.data) {
			tss = make([]prompbmarshal.TimeSeries, 0, len(s.data))
		}
		tss = tss[:len(s.data)]
		var i int
		for k, v := range s.data {
			ts := &tss[i]
			i++

			var err error
			ts.Labels, err = encoder.le.decode(ts.Labels[:0], k)
			if err != nil {
				decodeErrors.Inc()
				continue
			}
			if cap(ts.Samples) < 1 {
				ts.Samples = make([]prompbmarshal.Sample, 0, 1)
			}
			ts.Samples = ts.Samples[:1]
			ts.Samples[0].Value = v.value
			ts.Samples[0].Timestamp = v.timestamp
		}
		// mark shard as drained, so goroutines which concurrently execute push
		// could do a retry.
		s.drained = true
		s.mu.Unlock()

		d.callback(tss, nil)
		putTimeSeries(ptr)
	}

	wg := sync.WaitGroup{}
	for _, sh := range encoder.sm.shards {
		wg.Add(1)
		go func(s *shard) {
			processShard(s)
			wg.Done()
		}(sh)
	}
	wg.Wait()
}

var timeSeriesPool sync.Pool

func getTimeSeries() *[]prompbmarshal.TimeSeries {
	v := timeSeriesPool.Get()
	if v == nil {
		s := make([]prompbmarshal.TimeSeries, 0)
		return &s
	}
	return v.(*[]prompbmarshal.TimeSeries)
}

func putTimeSeries(tss *[]prompbmarshal.TimeSeries) {
	timeSeriesPool.Put(tss)
}

type shardedMap struct {
	shards []*shard
}

type shard struct {
	mu      sync.Mutex
	data    map[string]dedupStateValue
	drained bool
}

type dedupStateValue struct {
	timestamp int64
	value     float64
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
