package logstorage

import (
	"sync"
	"unsafe"

	"github.com/cespare/xxhash/v2"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
)

type hitsMap struct {
	stateSizeBudget *int

	u64        map[uint64]*uint64
	negative64 map[uint64]*uint64
	strings    map[string]*uint64

	// a reduces memory allocations when counting the number of hits over big number of unique values.
	a chunkedAllocator
}

func (hm *hitsMap) reset() {
	hm.stateSizeBudget = nil

	hm.u64 = nil
	hm.negative64 = nil
	hm.strings = nil
}

func (hm *hitsMap) clear() {
	*hm.stateSizeBudget += hm.stateSize()
	hm.init(hm.stateSizeBudget)
}

func (hm *hitsMap) init(stateSizeBudget *int) {
	hm.stateSizeBudget = stateSizeBudget

	hm.u64 = make(map[uint64]*uint64)
	hm.negative64 = make(map[uint64]*uint64)
	hm.strings = make(map[string]*uint64)
}

func (hm *hitsMap) entriesCount() uint64 {
	n := len(hm.u64) + len(hm.negative64) + len(hm.strings)
	return uint64(n)
}

func (hm *hitsMap) stateSize() int {
	n := 24*(len(hm.u64)+len(hm.negative64)) + 40*len(hm.strings)
	for k := range hm.strings {
		n += len(k)
	}
	return n
}

func (hm *hitsMap) updateStateGeneric(key string, hits uint64) {
	if n, ok := tryParseUint64(key); ok {
		hm.updateStateUint64(n, hits)
		return
	}
	if len(key) > 0 && key[0] == '-' {
		if n, ok := tryParseInt64(key); ok {
			hm.updateStateNegativeInt64(n, hits)
			return
		}
	}
	hm.updateStateString(bytesutil.ToUnsafeBytes(key), hits)
}

func (hm *hitsMap) updateStateInt64(n int64, hits uint64) {
	if n >= 0 {
		hm.updateStateUint64(uint64(n), hits)
	} else {
		hm.updateStateNegativeInt64(n, hits)
	}
}

func (hm *hitsMap) updateStateUint64(n, hits uint64) {
	pHits := hm.u64[n]
	if pHits != nil {
		*pHits += hits
		return
	}

	pHits = hm.a.newUint64()
	*pHits = hits
	hm.u64[n] = pHits

	*hm.stateSizeBudget -= 24
}

func (hm *hitsMap) updateStateNegativeInt64(n int64, hits uint64) {
	pHits := hm.negative64[uint64(n)]
	if pHits != nil {
		*pHits += hits
		return
	}

	pHits = hm.a.newUint64()
	*pHits = hits
	hm.negative64[uint64(n)] = pHits

	*hm.stateSizeBudget -= 24
}

func (hm *hitsMap) updateStateString(key []byte, hits uint64) {
	pHits := hm.strings[string(key)]
	if pHits != nil {
		*pHits += hits
		return
	}

	keyCopy := hm.a.cloneBytesToString(key)
	pHits = hm.a.newUint64()
	*pHits = hits
	hm.strings[keyCopy] = pHits

	*hm.stateSizeBudget -= len(keyCopy) + 40
}

func (hm *hitsMap) mergeState(src *hitsMap, stopCh <-chan struct{}) {
	for n, pHitsSrc := range src.u64 {
		if needStop(stopCh) {
			return
		}
		pHitsDst := hm.u64[n]
		if pHitsDst == nil {
			hm.u64[n] = pHitsSrc
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
			hm.negative64[n] = pHitsSrc
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
			hm.strings[k] = pHitsSrc
		} else {
			*pHitsDst += *pHitsSrc
		}
	}
}

// hitsMapMergeParallel merges hms in parallel on the given cpusCount
//
// The mered disjoint parts of hms are passed to f.
// The function may be interrupted by closing stopCh.
// The caller must check for closed stopCh after returning from the function.
func hitsMapMergeParallel(hms []*hitsMap, cpusCount int, stopCh <-chan struct{}, f func(hm *hitsMap)) {
	srcLen := len(hms)
	if srcLen < 2 {
		// Nothing to merge
		if len(hms) == 1 {
			f(hms[0])
		}
		return
	}

	var wg sync.WaitGroup
	perShardMaps := make([][]hitsMap, srcLen)
	for i := range hms {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			stateSizeBudget := 0
			perCPU := make([]hitsMap, cpusCount)
			for i := range perCPU {
				perCPU[i].init(&stateSizeBudget)
			}

			hm := hms[idx]

			for n, pHits := range hm.u64 {
				if needStop(stopCh) {
					return
				}
				k := unsafe.Slice((*byte)(unsafe.Pointer(&n)), 8)
				h := xxhash.Sum64(k)
				cpuIdx := h % uint64(len(perCPU))
				perCPU[cpuIdx].u64[n] = pHits
			}
			for n, pHits := range hm.negative64 {
				if needStop(stopCh) {
					return
				}
				k := unsafe.Slice((*byte)(unsafe.Pointer(&n)), 8)
				h := xxhash.Sum64(k)
				cpuIdx := h % uint64(len(perCPU))
				perCPU[cpuIdx].negative64[n] = pHits
			}
			for k, pHits := range hm.strings {
				if needStop(stopCh) {
					return
				}
				h := xxhash.Sum64(bytesutil.ToUnsafeBytes(k))
				cpuIdx := h % uint64(len(perCPU))
				perCPU[cpuIdx].strings[k] = pHits
			}

			perShardMaps[idx] = perCPU
			hm.reset()
		}(i)
	}
	wg.Wait()
	if needStop(stopCh) {
		return
	}

	// Merge per-shard entries into perShardMaps[0]
	for i := 0; i < cpusCount; i++ {
		wg.Add(1)
		go func(cpuIdx int) {
			defer wg.Done()

			hm := &perShardMaps[0][cpuIdx]
			for _, perCPU := range perShardMaps[1:] {
				hm.mergeState(&perCPU[cpuIdx], stopCh)
				perCPU[cpuIdx].reset()
			}
			f(hm)
		}(i)
	}
	wg.Wait()
}
