package storage

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/timeutil"
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
	vCurrImmutable atomic.Pointer[uint64set.Set]

	vPrevImmutable *uint64set.Set

	// Contains vMutable map protected by mu
	vMutable *uint64set.Set

	// Contains the number of slow accesses to vMutable.
	// Is used for deciding when to merge vMutable to vImmutable.
	// Protected by mu.
	slowHits int

	mu sync.Mutex

	stopCh           chan struct{}
	cleanerStoppedCh chan struct{}
}

func newMetricIDCache() *metricIDCache {
	mc := metricIDCache{
		vPrevImmutable:   &uint64set.Set{},
		vMutable:         &uint64set.Set{},
		stopCh:           make(chan struct{}),
		cleanerStoppedCh: make(chan struct{}),
	}
	mc.vCurrImmutable.Store(&uint64set.Set{})
	go mc.startCleaner()
	return &mc
}

func (mc *metricIDCache) Stop() {
	close(mc.stopCh)
	<-mc.cleanerStoppedCh
}

func (mc *metricIDCache) Has(metricID uint64) bool {
	if mc.vCurrImmutable.Load().Has(metricID) {
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
	vCurrImmutable := mc.vCurrImmutable.Load()
	if vCurrImmutable.Has(metricID) {
		return true
	}

	// Then check vPrevImmutable and vMutable maps.
	var ok bool
	vMutable := mc.vMutable
	if mc.vPrevImmutable.Has(metricID) {
		vMutable.Add(metricID)
		ok = true
	} else {
		ok = vMutable.Has(metricID)
	}

	if ok {
		mc.slowHits++
		if mc.slowHits > (vCurrImmutable.Len()+vMutable.Len())/2 {
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
	// Merge data from vCurrImmutable into vMutable.
	vCurrImmutable := mc.vCurrImmutable.Load()
	if mc.vMutable.Len() > 0 {
		mc.vMutable.Union(vCurrImmutable)
	}

	// Atomically replace vCurrImmutable with vMutable and
	// vPrevImmutable with vCurrMutable.
	mc.vCurrImmutable.Store(mc.vMutable)
	mc.vPrevImmutable = vCurrImmutable
	mc.vMutable = &uint64set.Set{}
}

func (mc *metricIDCache) startCleaner() {
	d := timeutil.AddJitterToDuration(time.Hour)
	ticker := time.NewTicker(d)
	defer ticker.Stop()
	for {
		select {
		case <-mc.stopCh:
			close(mc.cleanerStoppedCh)
			return
		case <-ticker.C:
			mc.clean()
		}
	}
}

func (mc *metricIDCache) clean() {
	mc.mu.Lock()
	defer mc.mu.Unlock()
	mc.syncLocked()
}
