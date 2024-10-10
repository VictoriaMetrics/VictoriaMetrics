package streamaggr

import (
	"sync"
	"sync/atomic"
	"unsafe"

	"github.com/cespare/xxhash/v2"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/decimal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/slicesutil"
)

const dedupAggrShardsCount = 128

type dedupAggr struct {
	shards    []dedupAggrShard
	stateSize int
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
	mu    sync.Mutex
	state []*dedupAggrState
}

type dedupAggrSample struct {
	value     float64
	timestamp int64
}

func newDedupAggr(stateSize int) *dedupAggr {
	shards := make([]dedupAggrShard, dedupAggrShardsCount)
	return &dedupAggr{
		shards:    shards,
		stateSize: stateSize,
	}
}

func (da *dedupAggr) sizeBytes() uint64 {
	n := uint64(unsafe.Sizeof(*da))
	for i := range da.shards {
		for _, state := range da.shards[i].state {
			if state != nil {
				n += state.sizeBytes.Load()
			}
		}
	}
	return n
}

func (da *dedupAggr) itemsCount() uint64 {
	n := uint64(0)
	for i := range da.shards {
		for _, state := range da.shards[i].state {
			if state != nil {
				n += state.itemsCount.Load()
			}
		}
	}
	return n
}

func (da *dedupAggr) pushSamples(data *pushCtxData) {
	pss := getPerShardSamples()
	shards := pss.shards
	for _, sample := range data.samples {
		h := xxhash.Sum64(bytesutil.ToUnsafeBytes(sample.key))
		idx := h % uint64(len(shards))
		shards[idx] = append(shards[idx], sample)
	}
	for i, shardSamples := range shards {
		if len(shardSamples) == 0 {
			continue
		}
		da.shards[i].pushSamples(shardSamples, da.stateSize, data.idx)
	}
	putPerShardSamples(pss)
}

func getDedupFlushCtx(deleteDeadline int64, idx int) *dedupFlushCtx {
	v := dedupFlushCtxPool.Get()
	if v == nil {
		v = &dedupFlushCtx{}
	}
	ctx := v.(*dedupFlushCtx)
	ctx.deleteDeadline = deleteDeadline
	ctx.idx = idx
	return ctx
}

func putDedupFlushCtx(ctx *dedupFlushCtx) {
	ctx.reset()
	dedupFlushCtxPool.Put(ctx)
}

var dedupFlushCtxPool sync.Pool

type dedupFlushCtx struct {
	samples        []pushSample
	deleteDeadline int64
	idx            int
}

func (ctx *dedupFlushCtx) getPushCtxData(samples []pushSample) *pushCtxData {
	return &pushCtxData{
		samples:        samples,
		deleteDeadline: ctx.deleteDeadline,
		idx:            0,
	}
}

func (ctx *dedupFlushCtx) reset() {
	clear(ctx.samples)
	ctx.samples = ctx.samples[:0]
	ctx.deleteDeadline = 0
	ctx.idx = 0
}

func (da *dedupAggr) flush(f aggrPushFunc, deleteDeadline int64, idx int) {
	var wg sync.WaitGroup
	for i := range da.shards {
		flushConcurrencyCh <- struct{}{}
		wg.Add(1)
		go func(shard *dedupAggrShard) {
			defer func() {
				<-flushConcurrencyCh
				wg.Done()
			}()
			ctx := getDedupFlushCtx(deleteDeadline, idx)
			shard.flush(ctx, f)
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

func (das *dedupAggrShard) pushSamples(samples []pushSample, stateSize, idx int) {
	das.mu.Lock()
	defer das.mu.Unlock()

	if len(das.state) == 0 {
		das.state = make([]*dedupAggrState, stateSize)
	}
	state := das.state[idx]
	if state == nil {
		state = &dedupAggrState{
			m: make(map[string]*dedupAggrSample, len(samples)),
		}
		das.state[idx] = state
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

			das.state[idx].itemsCount.Add(1)
			das.state[idx].sizeBytes.Add(uint64(len(key)) + uint64(unsafe.Sizeof(key)+unsafe.Sizeof(s)+unsafe.Sizeof(*s)))
			continue
		}
		if !isDuplicate(s, sample) {
			s.value = sample.value
			s.timestamp = sample.timestamp
		}
	}
	das.state[idx].samplesBuf = samplesBuf
}

// isDuplicate returns true if b is duplicate of a
// See https://docs.victoriametrics.com/#deduplication
func isDuplicate(a *dedupAggrSample, b pushSample) bool {
	if b.timestamp > a.timestamp {
		return false
	}
	if b.timestamp == a.timestamp {
		if decimal.IsStaleNaN(b.value) {
			return false
		}
		if b.value > a.value {
			return false
		}
	}
	return true
}

func (das *dedupAggrShard) flush(ctx *dedupFlushCtx, f aggrPushFunc) {
	das.mu.Lock()

	var m map[string]*dedupAggrSample
	if len(das.state) == 0 {
		das.mu.Unlock()
		return
	}
	state := das.state[ctx.idx]
	if state != nil && len(state.m) > 0 {
		m = state.m
		das.state[ctx.idx].m = make(map[string]*dedupAggrSample, len(state.m))
		das.state[ctx.idx].samplesBuf = make([]dedupAggrSample, 0, len(das.state[ctx.idx].samplesBuf))
		das.state[ctx.idx].sizeBytes.Store(0)
		das.state[ctx.idx].itemsCount.Store(0)
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
			data := ctx.getPushCtxData(dstSamples)
			f(data)
			clear(dstSamples)
			dstSamples = dstSamples[:0]
		}
	}
	data := ctx.getPushCtxData(dstSamples)
	f(data)
	ctx.samples = dstSamples
}
