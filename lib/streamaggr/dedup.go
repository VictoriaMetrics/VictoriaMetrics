package streamaggr

import (
	"sync"
	"sync/atomic"
	"unsafe"

	"github.com/cespare/xxhash/v2"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/slicesutil"
)

const dedupAggrShardsCount = 128

type dedupAggr struct {
	shards     []dedupAggrShard
	currentIdx atomic.Int32
}

type dedupAggrShard struct {
	dedupAggrShardNopad

	// The padding prevents false sharing on widespread platforms with
	// 128 mod (cache line size) = 0 .
	_ [128 - unsafe.Sizeof(dedupAggrShardNopad{})%128]byte
}

type dedupAggrState struct {
	m          map[string]*dedupAggrSample
	samplesBuf []dedupAggrSample
	sizeBytes  atomic.Uint64
	itemsCount atomic.Uint64
}

type dedupAggrShardNopad struct {
	mu    sync.RWMutex
	state [aggrStateSize]*dedupAggrState
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
	currentIdx := da.currentIdx.Load()
	for i := range da.shards {
		if da.shards[i].state[currentIdx] != nil {
			n += da.shards[i].state[currentIdx].sizeBytes.Load()
		}
	}
	return n
}

func (da *dedupAggr) itemsCount() uint64 {
	n := uint64(0)
	currentIdx := da.currentIdx.Load()
	for i := range da.shards {
		if da.shards[i].state[currentIdx] != nil {
			n += da.shards[i].state[currentIdx].itemsCount.Load()
		}
	}
	return n
}

func (da *dedupAggr) pushSamples(samples []pushSample, dedupIdx int) {
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
		da.shards[i].pushSamples(shardSamples, dedupIdx)
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

func (da *dedupAggr) flush(f aggrPushFunc, deleteDeadline int64, dedupIdx, flushIdx int) {
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
			shard.flush(ctx, f, deleteDeadline, dedupIdx, flushIdx)
			putDedupFlushCtx(ctx)
		}(&da.shards[i])
	}
	da.currentIdx.Store((da.currentIdx.Load() + 1) % aggrStateSize)
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

func (das *dedupAggrShard) pushSamples(samples []pushSample, dedupIdx int) {
	das.mu.Lock()
	defer das.mu.Unlock()

	state := das.state[dedupIdx]
	if state == nil {
		state = &dedupAggrState{
			m: make(map[string]*dedupAggrSample, len(samples)),
		}
		das.state[dedupIdx] = state
	}
	samplesBuf := state.samplesBuf
	for _, sample := range samples {
		s, ok := state.m[sample.key]
		if !ok {
			samplesBuf = slicesutil.SetLength(samplesBuf, len(samplesBuf)+1)
			s = &samplesBuf[len(samplesBuf)-1]
			s.value = sample.value
			s.timestamp = sample.timestamp

			key := bytesutil.InternString(sample.key)
			state.m[key] = s

			das.state[dedupIdx].itemsCount.Add(1)
			das.state[dedupIdx].sizeBytes.Add(uint64(len(key)) + uint64(unsafe.Sizeof(key)+unsafe.Sizeof(s)+unsafe.Sizeof(*s)))
			continue
		}
		// Update the existing value according to logic described at https://docs.victoriametrics.com/#deduplication
		if sample.timestamp > s.timestamp || (sample.timestamp == s.timestamp && sample.value > s.value) {
			s.value = sample.value
			s.timestamp = sample.timestamp
		}
	}
	das.state[dedupIdx].samplesBuf = samplesBuf
}

func (das *dedupAggrShard) flush(ctx *dedupFlushCtx, f aggrPushFunc, deleteDeadline int64, dedupIdx, flushIdx int) {
	das.mu.Lock()

	var m map[string]*dedupAggrSample
	state := das.state[dedupIdx]
	if state != nil && len(state.m) > 0 {
		m = state.m
		das.state[dedupIdx].m = make(map[string]*dedupAggrSample, len(state.m))
		das.state[dedupIdx].samplesBuf = make([]dedupAggrSample, 0, len(das.state[dedupIdx].samplesBuf))
		das.state[dedupIdx].sizeBytes.Store(0)
		das.state[dedupIdx].itemsCount.Store(0)
	}

	das.mu.Unlock()

	if len(m) == 0 {
		return
	}

	dstSamples := ctx.samples
	for key, s := range m {
		dstSamples = append(dstSamples, pushSample{
			key:       key,
			value:     s.value,
			timestamp: s.timestamp,
		})

		// Limit the number of samples per each flush in order to limit memory usage.
		if len(dstSamples) >= 10_000 {
			f(dstSamples, deleteDeadline, flushIdx)
			clear(dstSamples)
			dstSamples = dstSamples[:0]
		}
	}
	f(dstSamples, deleteDeadline, flushIdx)
	ctx.samples = dstSamples
}
