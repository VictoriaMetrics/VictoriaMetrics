package logstorage

import (
	"sync"
	"unsafe"

	"github.com/cespare/xxhash/v2"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/atomicutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
)

type hitsMapAdaptive struct {
	stateSizeBudget *int

	// concurrency is the number of parallel workers to use when merging hmShards.
	//
	// this field must be updated by the caller via init() before using hitsMapAdaptive.
	concurrency uint

	// hm tracks hits until the number of unique values reaches hitsMapAdaptiveMaxLen.
	// After that hits are tracked by hmShards.
	hm hitsMap

	// hmShards tracks hits for big number of unique values.
	//
	// Every shard contains hits for a share of unique values.
	hmShards []hitsMapShard

	// a reduces memory allocations when counting the number of hits over big number of unique values.
	a chunkedAllocator
}

type hitsMapShard struct {
	hitsMap

	// The padding prevents false sharing
	_ [atomicutil.CacheLineSize - unsafe.Sizeof(hitsMap{})%atomicutil.CacheLineSize]byte
}

// the maximum number of values to track in hitsMapAdaptive.hm before switching to hitsMapAdaptive.hmShards
//
// Too big value may slow down hitsMapMergeParallel() across big number of CPU cores.
// Too small value may significantly increase RAM usage when hits for big number of unique values are counted.
const hitsMapAdaptiveMaxLen = 4 << 10

func (hma *hitsMapAdaptive) reset() {
	*hma = hitsMapAdaptive{}
}

func (hma *hitsMapAdaptive) init(concurrency uint, stateSizeBudget *int) {
	hma.reset()
	hma.stateSizeBudget = stateSizeBudget
	hma.concurrency = concurrency
}

func (hma *hitsMapAdaptive) clear() {
	*hma.stateSizeBudget += hma.stateSize()
	hma.init(hma.concurrency, hma.stateSizeBudget)
}

func (hma *hitsMapAdaptive) stateSize() int {
	n := hma.hm.stateSize()

	shards := hma.hmShards
	for i := range shards {
		n += shards[i].stateSize()
	}
	return n
}

func (hma *hitsMapAdaptive) entriesCount() uint64 {
	if hma.hmShards == nil {
		return hma.hm.entriesCount()
	}

	shards := hma.hmShards
	n := uint64(0)
	for i := range shards {
		n += shards[i].entriesCount()
	}
	return n
}

func (hma *hitsMapAdaptive) updateStateGeneric(key string, hits uint64) {
	if n, ok := tryParseUint64(key); ok {
		hma.updateStateUint64(n, hits)
		return
	}
	if len(key) > 0 && key[0] == '-' {
		if n, ok := tryParseInt64(key); ok {
			hma.updateStateNegativeInt64(n, hits)
			return
		}
	}
	hma.updateStateString(bytesutil.ToUnsafeBytes(key), hits)
}

func (hma *hitsMapAdaptive) updateStateInt64(n int64, hits uint64) {
	if n >= 0 {
		hma.updateStateUint64(uint64(n), hits)
	} else {
		hma.updateStateNegativeInt64(n, hits)
	}
}

func (hma *hitsMapAdaptive) updateStateUint64(n, hits uint64) {
	if hma.hmShards == nil {
		stateSize := hma.hm.updateStateUint64(&hma.a, n, hits)
		if stateSize > 0 {
			*hma.stateSizeBudget -= stateSize
			hma.probablyMoveToShards(&hma.a)
		}
		return
	}
	hm := hma.getShardByUint64(n)
	*hma.stateSizeBudget -= hm.updateStateUint64(&hma.a, n, hits)
}

func (hma *hitsMapAdaptive) updateStateNegativeInt64(n int64, hits uint64) {
	if hma.hmShards == nil {
		stateSize := hma.hm.updateStateNegativeInt64(&hma.a, n, hits)
		if stateSize > 0 {
			*hma.stateSizeBudget -= stateSize
			hma.probablyMoveToShards(&hma.a)
		}
		return
	}
	hm := hma.getShardByUint64(uint64(n))
	*hma.stateSizeBudget -= hm.updateStateNegativeInt64(&hma.a, n, hits)
}

func (hma *hitsMapAdaptive) updateStateString(key []byte, hits uint64) {
	if hma.hmShards == nil {
		stateSize := hma.hm.updateStateString(&hma.a, key, hits)
		if stateSize > 0 {
			*hma.stateSizeBudget -= stateSize
			hma.probablyMoveToShards(&hma.a)
		}
		return
	}
	hm := hma.getShardByString(key)
	*hma.stateSizeBudget -= hm.updateStateString(&hma.a, key, hits)
}

func (hma *hitsMapAdaptive) probablyMoveToShards(a *chunkedAllocator) {
	if hma.hm.entriesCount() < hitsMapAdaptiveMaxLen {
		return
	}
	hma.moveToShards(a)
}

func (hma *hitsMapAdaptive) moveToShards(a *chunkedAllocator) {
	hma.hmShards = a.newHitsMapShards(hma.concurrency)

	for n, pHits := range hma.hm.u64 {
		hm := hma.getShardByUint64(n)
		hm.setStateUint64(n, pHits)
	}
	for n, pHits := range hma.hm.negative64 {
		hm := hma.getShardByUint64(n)
		hm.setStateNegativeInt64(int64(n), pHits)
	}
	for s, pHits := range hma.hm.strings {
		hm := hma.getShardByString(bytesutil.ToUnsafeBytes(s))
		hm.setStateString(s, pHits)
	}

	hma.hm.reset()
}

func (hma *hitsMapAdaptive) getShardByUint64(n uint64) *hitsMap {
	h := fastHashUint64(n)
	shardIdx := h % uint64(len(hma.hmShards))
	return &hma.hmShards[shardIdx].hitsMap
}

func (hma *hitsMapAdaptive) getShardByString(v []byte) *hitsMap {
	h := xxhash.Sum64(v)
	shardIdx := h % uint64(len(hma.hmShards))
	return &hma.hmShards[shardIdx].hitsMap
}

type hitsMap struct {
	u64        map[uint64]*uint64
	negative64 map[uint64]*uint64
	strings    map[string]*uint64
}

func (hm *hitsMap) reset() {
	*hm = hitsMap{}
}

func (hm *hitsMap) entriesCount() uint64 {
	n := len(hm.u64) + len(hm.negative64) + len(hm.strings)
	return uint64(n)
}

func (hm *hitsMap) stateSize() int {
	size := 0

	for n, pHits := range hm.u64 {
		size += int(unsafe.Sizeof(n) + unsafe.Sizeof(pHits) + unsafe.Sizeof(*pHits))
	}
	for n, pHits := range hm.negative64 {
		size += int(unsafe.Sizeof(n) + unsafe.Sizeof(pHits) + unsafe.Sizeof(*pHits))
	}
	for k, pHits := range hm.strings {
		size += len(k) + int(unsafe.Sizeof(k)+unsafe.Sizeof(pHits)+unsafe.Sizeof(*pHits))
	}

	return size
}

func (hm *hitsMap) updateStateUint64(a *chunkedAllocator, n, hits uint64) int {
	pHits := hm.u64[n]
	if pHits != nil {
		*pHits += hits
		return 0
	}

	pHits = a.newUint64()
	*pHits = hits
	return int(unsafe.Sizeof(*pHits)) + hm.setStateUint64(n, pHits)
}

func (hm *hitsMap) setStateUint64(n uint64, pHits *uint64) int {
	if hm.u64 == nil {
		hm.u64 = map[uint64]*uint64{
			n: pHits,
		}
		return int(unsafe.Sizeof(hm.u64) + unsafe.Sizeof(n) + unsafe.Sizeof(pHits))
	}
	hm.u64[n] = pHits
	return int(unsafe.Sizeof(n) + unsafe.Sizeof(pHits))
}

func (hm *hitsMap) updateStateNegativeInt64(a *chunkedAllocator, n int64, hits uint64) int {
	pHits := hm.negative64[uint64(n)]
	if pHits != nil {
		*pHits += hits
		return 0
	}

	pHits = a.newUint64()
	*pHits = hits
	return int(unsafe.Sizeof(*pHits)) + hm.setStateNegativeInt64(n, pHits)
}

func (hm *hitsMap) setStateNegativeInt64(n int64, pHits *uint64) int {
	if hm.negative64 == nil {
		hm.negative64 = map[uint64]*uint64{
			uint64(n): pHits,
		}
		return int(unsafe.Sizeof(hm.negative64) + unsafe.Sizeof(uint64(n)) + unsafe.Sizeof(pHits))
	}
	hm.negative64[uint64(n)] = pHits
	return int(unsafe.Sizeof(n) + unsafe.Sizeof(pHits))
}

func (hm *hitsMap) updateStateString(a *chunkedAllocator, key []byte, hits uint64) int {
	pHits := hm.strings[string(key)]
	if pHits != nil {
		*pHits += hits
		return 0
	}

	keyCopy := a.cloneBytesToString(key)
	pHits = a.newUint64()
	*pHits = hits
	return len(keyCopy) + int(unsafe.Sizeof(*pHits)) + hm.setStateString(keyCopy, pHits)
}

func (hm *hitsMap) setStateString(v string, pHits *uint64) int {
	if hm.strings == nil {
		hm.strings = map[string]*uint64{
			v: pHits,
		}
		return int(unsafe.Sizeof(hm.strings) + unsafe.Sizeof(v) + unsafe.Sizeof(pHits))
	}
	hm.strings[v] = pHits
	return int(unsafe.Sizeof(v) + unsafe.Sizeof(pHits))
}

func (hm *hitsMap) mergeState(src *hitsMap, stopCh <-chan struct{}) {
	for n, pHitsSrc := range src.u64 {
		if needStop(stopCh) {
			return
		}
		pHitsDst := hm.u64[n]
		if pHitsDst == nil {
			hm.setStateUint64(n, pHitsSrc)
		} else {
			*pHitsDst += *pHitsSrc
		}
	}
	for n, pHitsSrc := range src.negative64 {
		if needStop(stopCh) {
			return
		}
		pHitsDst := hm.negative64[n]
		if pHitsDst == nil {
			hm.setStateNegativeInt64(int64(n), pHitsSrc)
		} else {
			*pHitsDst += *pHitsSrc
		}
	}
	for k, pHitsSrc := range src.strings {
		if needStop(stopCh) {
			return
		}
		pHitsDst := hm.strings[k]
		if pHitsDst == nil {
			hm.setStateString(k, pHitsSrc)
		} else {
			*pHitsDst += *pHitsSrc
		}
	}
}

// hitsMapMergeParallel merges hmas in parallel
//
// The merged disjoint parts of hmas are passed to f.
// The function may be interrupted by closing stopCh.
// The caller must check for closed stopCh after returning from the function.
func hitsMapMergeParallel(hmas []*hitsMapAdaptive, stopCh <-chan struct{}, f func(hm *hitsMap)) {
	if len(hmas) == 0 {
		return
	}

	var wg sync.WaitGroup
	for i := range hmas {
		hma := hmas[i]
		if hma.hmShards != nil {
			continue
		}
		wg.Add(1)
		go func() {
			defer wg.Done()

			var a chunkedAllocator
			hma.moveToShards(&a)
		}()
	}
	wg.Wait()
	if needStop(stopCh) {
		return
	}

	cpusCount := len(hmas[0].hmShards)

	for i := 0; i < cpusCount; i++ {
		wg.Add(1)
		go func(cpuIdx int) {
			defer wg.Done()

			hm := &hmas[0].hmShards[cpuIdx].hitsMap
			for j := range hmas[1:] {
				src := &hmas[1+j].hmShards[cpuIdx].hitsMap
				hm.mergeState(src, stopCh)
				src.reset()
			}
			f(hm)
		}(i)
	}
	wg.Wait()
}
