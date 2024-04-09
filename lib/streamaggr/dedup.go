package streamaggr

import (
	"sync"
	"unsafe"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/cespare/xxhash/v2"
)

const dedupAggrShardsCount = 128

type dedupAggr struct {
	shards []dedupAggrShard
}

type dedupAggrShard struct {
	dedupAggrShardNopad

	// The padding prevents false sharing on widespread platforms with
	// 128 mod (cache line size) = 0 .
	_ [128 - unsafe.Sizeof(dedupAggrShardNopad{})%128]byte
}

type dedupAggrShardNopad struct {
	mu sync.Mutex
	m  map[int64]map[string]*dedupAggrSample
}

type dedupAggrSample struct {
	value     float64
	timestamp int64
}

func newDedupAggr() *dedupAggr {
	shards := make([]dedupAggrShard, dedupAggrShardsCount)
	return &dedupAggr{
		shards: shards,
	}
}

func (da *dedupAggr) sizeBytes() uint64 {
	n := uint64(unsafe.Sizeof(*da))
	for i := range da.shards {
		n += da.shards[i].sizeBytes()
	}
	return n
}

func (da *dedupAggr) itemsCount() uint64 {
	n := uint64(0)
	for i := range da.shards {
		n += da.shards[i].itemsCount()
	}
	return n
}

func (das *dedupAggrShard) sizeBytes() uint64 {
	das.mu.Lock()
	n := uint64(unsafe.Sizeof(*das))
	for _, s := range das.m {
		for k, m := range s {
			n += uint64(len(k)) + uint64(unsafe.Sizeof(k)+unsafe.Sizeof(m))
		}
	}
	das.mu.Unlock()
	return n
}

func (das *dedupAggrShard) itemsCount() uint64 {
	das.mu.Lock()
	var n uint64
	for _, m := range das.m {
		n += uint64(len(m))
	}
	das.mu.Unlock()
	return n
}

func (da *dedupAggr) pushSamples(windows map[int64][]pushSample) {
	pss := getPerShardSamples()
	shards := pss.shards
	for ts, samples := range windows {
		for _, sample := range samples {
			h := xxhash.Sum64(bytesutil.ToUnsafeBytes(sample.key))
			idx := h % uint64(len(shards))
			shards[idx] = append(shards[idx], sample)
		}
		for i, shardSamples := range shards {
			if len(shardSamples) == 0 {
				continue
			}
			da.shards[i].pushSamples(shardSamples, ts)
		}
	}
	putPerShardSamples(pss)
}

func getDedupFlushCtx() *dedupFlushCtx {
	v := dedupFlushCtxPool.Get()
	if v == nil {
		return &dedupFlushCtx{}
	}
	return v.(*dedupFlushCtx)
}

func putDedupFlushCtx(ctx *dedupFlushCtx) {
	ctx.reset()
	dedupFlushCtxPool.Put(ctx)
}

var dedupFlushCtxPool sync.Pool

type dedupFlushCtx struct {
	samples map[int64][]pushSample
}

func (ctx *dedupFlushCtx) reset() {
	clear(ctx.samples)
	ctx.samples = make(map[int64][]pushSample)
}

func (da *dedupAggr) flush(f func(samples map[int64][]pushSample), dedupTimestamp, flushTimestamp int64) {
	var wg sync.WaitGroup
	for i := range da.shards {
		flushConcurrencyCh <- struct{}{}
		wg.Add(1)
		go func(shard *dedupAggrShard) {
			defer func() {
				<-flushConcurrencyCh
				wg.Done()
			}()

			ctx := getDedupFlushCtx()
			shard.flush(ctx, f, dedupTimestamp, flushTimestamp)
			putDedupFlushCtx(ctx)
		}(&da.shards[i])
	}
	wg.Wait()
}

type perShardSamples struct {
	shards [][]pushSample
}

func (pss *perShardSamples) reset() {
	shards := pss.shards
	for i, shardSamples := range shards {
		if len(shardSamples) > 0 {
			clear(shardSamples)
			shards[i] = shardSamples[:0]
		}
	}
}

func getPerShardSamples() *perShardSamples {
	v := perShardSamplesPool.Get()
	if v == nil {
		return &perShardSamples{
			shards: make([][]pushSample, dedupAggrShardsCount),
		}
	}
	return v.(*perShardSamples)
}

func putPerShardSamples(pss *perShardSamples) {
	pss.reset()
	perShardSamplesPool.Put(pss)
}

var perShardSamplesPool sync.Pool

func (das *dedupAggrShard) pushSamples(samples []pushSample, ts int64) {
	das.mu.Lock()
	defer das.mu.Unlock()

	if das.m == nil {
		das.m = make(map[int64]map[string]*dedupAggrSample, 0)
	}
	for _, sample := range samples {
		if _, ok := das.m[ts]; !ok {
			das.m[ts] = map[string]*dedupAggrSample{}
		}
		if s, ok := das.m[ts][sample.key]; !ok {
			das.m[ts][sample.key] = &dedupAggrSample{
				value:     sample.value,
				timestamp: sample.timestamp,
			}
		} else if sample.timestamp > s.timestamp || (sample.timestamp == s.timestamp && sample.value > s.value) {
			// Update the existing value according to logic described at https://docs.victoriametrics.com/#deduplication
			das.m[ts][sample.key] = &dedupAggrSample{
				value:     sample.value,
				timestamp: sample.timestamp,
			}
		}
	}
}

func (das *dedupAggrShard) flush(ctx *dedupFlushCtx, f func(samples map[int64][]pushSample), dedupTimestamp, flushTimestamp int64) {
	fn := func(states map[int64]map[string]*dedupAggrSample) map[string]*dedupAggrSample {
		if dedupTimestamp > 0 && flushTimestamp > 0 {
			if state, ok := states[dedupTimestamp]; ok {
				delete(states, dedupTimestamp)
				return state
			}
		} else {
			for ts := range states {
				delete(states, ts)
			}
		}
		return states[-1]
	}
	das.mu.Lock()
	state := fn(das.m)
	das.mu.Unlock()
	ctx.reset()

	for key, s := range state {
		if _, ok := ctx.samples[flushTimestamp]; !ok {
			ctx.samples[flushTimestamp] = make([]pushSample, 0)
		}
		ctx.samples[flushTimestamp] = append(ctx.samples[flushTimestamp], pushSample{
			key:       key,
			value:     s.value,
			timestamp: s.timestamp,
		})
		// Limit the number of samples per each flush in order to limit memory usage.
		if len(ctx.samples[flushTimestamp]) >= 100_000 {
			f(ctx.samples)
			clear(ctx.samples)
		}
	}

	if len(ctx.samples) == 0 {
		return
	}

	f(ctx.samples)
}
