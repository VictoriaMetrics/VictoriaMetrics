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
	m  map[string]dedupAggrSample
}

type dedupAggrSample struct {
	value float64
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
	for k, s := range das.m {
		n += uint64(len(k)) + uint64(unsafe.Sizeof(k)+unsafe.Sizeof(s))
	}
	das.mu.Unlock()
	return n
}

func (das *dedupAggrShard) itemsCount() uint64 {
	das.mu.Lock()
	n := uint64(len(das.m))
	das.mu.Unlock()
	return n
}

func (da *dedupAggr) pushSamples(samples []pushSample) {
	pss := getPerShardSamples()
	shards := pss.shards
	for _, sample := range samples {
		h := xxhash.Sum64(bytesutil.ToUnsafeBytes(sample.key))
		idx := h % uint64(len(shards))
		shards[idx] = append(shards[idx], sample)
	}
	for i, shardSamples := range shards {
		if len(shardSamples) == 0 {
			continue
		}
		da.shards[i].pushSamples(shardSamples)
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
	samples []pushSample
}

func (ctx *dedupFlushCtx) reset() {
	clear(ctx.samples)
	ctx.samples = ctx.samples[:0]
}

func (da *dedupAggr) flush(f func(samples []pushSample), resetState bool) {
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
			shard.flush(ctx, f, resetState)
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

func (das *dedupAggrShard) pushSamples(samples []pushSample) {
	das.mu.Lock()
	defer das.mu.Unlock()

	m := das.m
	if m == nil {
		m = make(map[string]dedupAggrSample, len(samples))
		das.m = m
	}
	for _, sample := range samples {
		m[sample.key] = dedupAggrSample{
			value: sample.value,
		}
	}
}

func (das *dedupAggrShard) flush(ctx *dedupFlushCtx, f func(samples []pushSample), resetState bool) {
	das.mu.Lock()

	m := das.m
	if resetState && len(m) > 0 {
		das.m = make(map[string]dedupAggrSample, len(m))
	}

	das.mu.Unlock()

	if len(m) == 0 {
		return
	}

	dstSamples := ctx.samples
	for key, s := range m {
		dstSamples = append(dstSamples, pushSample{
			key:   key,
			value: s.value,
			// TODO: timestamp?
		})

		// Limit the number of samples per each flush in order to limit memory usage.
		if len(dstSamples) >= 100_000 {
			f(dstSamples)
			clear(dstSamples)
			dstSamples = dstSamples[:0]
		}
	}
	f(dstSamples)
	ctx.samples = dstSamples
}
