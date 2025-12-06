package storage

import (
	"sync"
	"sync/atomic"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/uint64set"
)

// metricIDCache stores metricIDs that have been added to the index. It is used
// during data ingestion do decide whether a new entry needs to be added to the
// global index.
//
// The cache avoids synchronization on the read path if possible to reduce
// contention. Based on dateMetricIDCache ideas.
type metricIDCache struct {
	// Contains vImmutable map
	vImmutable atomic.Pointer[uint64set.Set]

	// Contains vMutable map protected by mu
	vMutable *uint64set.Set

	// Contains the number of slow accesses to vMutable.
	// Is used for deciding when to merge vMutable to vImmutable.
	// Protected by mu.
	slowHits int

	mu sync.Mutex
}

func newMetricIDCache() *metricIDCache {
	var mc metricIDCache
	mc.vImmutable.Store(&uint64set.Set{})
	mc.vMutable = &uint64set.Set{}
	return &mc
}

func (mc *metricIDCache) Has(metricID uint64) bool {
	if mc.vImmutable.Load().Has(metricID) {
		// Fast path. The majority of calls must go here.
		return true
	}
	// Slow path. Acquire the lock and search the vImmutable map again and then
	// also search the vMutable map.
	return mc.hasSlow(metricID)
}

func (mc *metricIDCache) hasSlow(metricID uint64) bool {
	mc.mu.Lock()
	defer mc.mu.Unlock()

	// First, check vImmutable map again because the entry may have been moved to
	// the vImmutable map by the time the caller acquires the lock.
	vImmutable := mc.vImmutable.Load()
	if vImmutable.Has(metricID) {
		return true
	}

	// Then check vMutable map.
	vMutable := mc.vMutable
	ok := vMutable.Has(metricID)
	if ok {
		mc.slowHits++
		if mc.slowHits > (vImmutable.Len()+vMutable.Len())/2 {
			// It is cheaper to merge vMutable part into vImmutable than to pay inter-cpu sync costs when accessing vMutable.
			mc.syncLocked()
			mc.slowHits = 0
		}
	}
	return ok
}

func (mc *metricIDCache) Set(metricID uint64) {
	mc.mu.Lock()
	v := mc.vMutable
	v.Add(metricID)
	mc.mu.Unlock()
}

func (mc *metricIDCache) syncLocked() {
	if mc.vMutable.Len() == 0 {
		// Nothing to sync.
		return
	}

	// Merge data from vImmutable into vMutable and then atomically replace vImmutable with the merged data.
	vImmutable := mc.vImmutable.Load()
	vMutable := mc.vMutable
	vMutable.Union(vImmutable)

	// Atomically replace vImmutable with vMutable
	mc.vImmutable.Store(mc.vMutable)
	mc.vMutable = &uint64set.Set{}
}
