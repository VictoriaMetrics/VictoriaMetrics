package streamaggr

import (
	"sync"
	"sync/atomic"
	"unsafe"

	"github.com/cespare/xxhash/v2"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/slicesutil"
)

const dedupAggrShardsCount = 128

type dedupAggr struct {
	lc     *promutils.LabelsCompressor
	shards []dedupAggrShard
}

type dedupAggrShard struct {
	dedupAggrShardNopad

	// The padding prevents false sharing on widespread platforms with
	// 128 mod (cache line size) = 0 .
	_ [128 - unsafe.Sizeof(dedupAggrShardNopad{})%128]byte
}

type dedupAggrShardNopad struct {
	mu sync.RWMutex
	m  map[string]*dedupAggrSample

	samplesBuf []dedupAggrSample

	sizeBytes  atomic.Uint64
	itemsCount atomic.Uint64
}

type dedupAggrSample struct {
	value          float64
	timestamp      int64
	deleteDeadline int64
}

func newDedupAggr(lc *promutils.LabelsCompressor) *dedupAggr {
	shards := make([]dedupAggrShard, dedupAggrShardsCount)
	return &dedupAggr{
		shards: shards,
		lc:     lc,
	}
}

func (da *dedupAggr) sizeBytes() uint64 {
	n := uint64(unsafe.Sizeof(*da))
	for i := range da.shards {
		n += da.shards[i].sizeBytes.Load()
	}
	return n
}

func (da *dedupAggr) itemsCount() uint64 {
	n := uint64(0)
	for i := range da.shards {
		n += da.shards[i].itemsCount.Load()
	}
	return n
}

func (da *dedupAggr) pushSamples(samples []pushSample, deleteDeadline int64) {
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
		da.shards[i].pushSamples(shardSamples, deleteDeadline)
	}
	putPerShardSamples(pss)
}

func getDedupFlushCtx(flushTimestamp, deleteDeadline int64, lc *promutils.LabelsCompressor) *dedupFlushCtx {
	v := dedupFlushCtxPool.Get()
	if v == nil {
		v = &dedupFlushCtx{}
	}
	ctx := v.(*dedupFlushCtx)
	ctx.lc = lc
	ctx.deleteDeadline = deleteDeadline
	ctx.flushTimestamp = flushTimestamp
	return ctx
}

func putDedupFlushCtx(ctx *dedupFlushCtx) {
	ctx.reset()
	dedupFlushCtxPool.Put(ctx)
}

var dedupFlushCtxPool sync.Pool

type dedupFlushCtx struct {
	keysToDelete   []string
	samples        []pushSample
	deleteDeadline int64
	flushTimestamp int64
	lc             *promutils.LabelsCompressor
}

func (ctx *dedupFlushCtx) reset() {
	ctx.deleteDeadline = 0
	ctx.flushTimestamp = 0
	ctx.lc = nil
	clear(ctx.keysToDelete)
	ctx.keysToDelete = ctx.keysToDelete[:0]
	clear(ctx.samples)
	ctx.samples = ctx.samples[:0]
}

func (da *dedupAggr) flush(f func(samples []pushSample, deleteDeadline int64), flushTimestamp, deleteDeadline int64) {
	var wg sync.WaitGroup
	for i := range da.shards {
		flushConcurrencyCh <- struct{}{}
		wg.Add(1)
		go func(shard *dedupAggrShard) {
			defer func() {
				<-flushConcurrencyCh
				wg.Done()
			}()

			ctx := getDedupFlushCtx(flushTimestamp, deleteDeadline, da.lc)
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

func (das *dedupAggrShard) pushSamples(samples []pushSample, deleteDeadline int64) {
	das.mu.Lock()
	defer das.mu.Unlock()

	m := das.m
	if m == nil {
		m = make(map[string]*dedupAggrSample, len(samples))
		das.m = m
	}
	samplesBuf := das.samplesBuf
	for _, sample := range samples {
		s, ok := m[sample.key]
		if !ok {
			samplesBuf = slicesutil.SetLength(samplesBuf, len(samplesBuf)+1)
			s = &samplesBuf[len(samplesBuf)-1]
			s.value = sample.value
			s.timestamp = sample.timestamp
			s.deleteDeadline = deleteDeadline

			key := bytesutil.InternString(sample.key)
			m[key] = s

			das.itemsCount.Add(1)
			das.sizeBytes.Add(uint64(len(key)) + uint64(unsafe.Sizeof(key)+unsafe.Sizeof(s)+unsafe.Sizeof(*s)))
			continue
		}
		// Update the existing value according to logic described at https://docs.victoriametrics.com/#deduplication
		if sample.timestamp > s.timestamp || (sample.timestamp == s.timestamp && sample.value > s.value) {
			s.value = sample.value
			s.timestamp = sample.timestamp
			s.deleteDeadline = deleteDeadline
		}
	}
	das.samplesBuf = samplesBuf
}

func (das *dedupAggrShard) flush(ctx *dedupFlushCtx, f func(samples []pushSample, deleteDeadline int64)) {
	if len(das.m) == 0 {
		return
	}

	keysToDelete := ctx.keysToDelete
	dstSamples := ctx.samples
	das.mu.RLock()
	for key, s := range das.m {
		if ctx.flushTimestamp > s.deleteDeadline {
			das.itemsCount.Add(^uint64(0))
			//ctx.lc.Delete(key)
			das.sizeBytes.Add(^(uint64(len(key)) + uint64(unsafe.Sizeof(key)+unsafe.Sizeof(s)+unsafe.Sizeof(*s)) - 1))
			keysToDelete = append(keysToDelete, key)
			continue
		}
		dstSamples = append(dstSamples, pushSample{
			key:       key,
			value:     s.value,
			timestamp: s.timestamp,
		})

		// Limit the number of samples per each flush in order to limit memory usage.
		if len(dstSamples) >= 10_000 {
			f(dstSamples, ctx.deleteDeadline)
			clear(dstSamples)
			dstSamples = dstSamples[:0]
		}
	}
	das.mu.RUnlock()
	das.mu.Lock()
	for _, key := range keysToDelete {
		delete(das.m, key)
	}
	das.mu.Unlock()
	f(dstSamples, ctx.deleteDeadline)
	ctx.samples = dstSamples
}
