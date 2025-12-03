package storage

import (
	"sort"
	"sync"
	"sync/atomic"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/memory"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/uint64set"
)

// dateMetricIDCache is fast cache for holding (idb, date) -> metricID entries.
//
// It should be faster than map[date]*uint64set.Set on multicore systems.
//
// One instance of this cache is supposed to be shared by all indexDB instances.
// See #TODO
type dateMetricIDCache struct {
	syncsCount  atomic.Uint64
	resetsCount atomic.Uint64

	// Contains immutable map
	byDate atomic.Pointer[byDateMetricIDMap]

	// Contains mutable map protected by mu
	byDateMutable *byDateMetricIDMap

	// Contains the number of slow accesses to byDateMutable.
	// Is used for deciding when to merge byDateMutable to byDate.
	// Protected by mu.
	slowHits int

	mu sync.Mutex
}

func newDateMetricIDCache() *dateMetricIDCache {
	var dmc dateMetricIDCache
	dmc.resetLocked()
	return &dmc
}

func (dmc *dateMetricIDCache) resetLocked() {
	// Do not reset syncsCount and resetsCount
	dmc.byDate.Store(newByDateMetricIDMap())
	dmc.byDateMutable = newByDateMetricIDMap()
	dmc.slowHits = 0

	dmc.resetsCount.Add(1)
}

func (dmc *dateMetricIDCache) EntriesCount() int {
	byDate := dmc.byDate.Load()
	n := 0
	for _, metricIDs := range byDate.m {
		n += metricIDs.Len()
	}
	return n
}

func (dmc *dateMetricIDCache) SizeBytes() uint64 {
	byDate := dmc.byDate.Load()
	n := uint64(0)
	for _, metricIDs := range byDate.m {
		n += metricIDs.SizeBytes()
	}
	return n
}

func (dmc *dateMetricIDCache) Has(idbID, date, metricID uint64) bool {
	if byDate := dmc.byDate.Load(); byDate.get(idbID, date).Has(metricID) {
		// Fast path. The majority of calls must go here.
		return true
	}
	// Slow path. Acquire the lock and search the immutable map again and then
	// also search the mutable map.
	return dmc.hasSlow(idbID, date, metricID)
}

func (dmc *dateMetricIDCache) hasSlow(idbID, date, metricID uint64) bool {
	dmc.mu.Lock()
	defer dmc.mu.Unlock()

	// First, check immutable map again because the entry may have been moved to
	// the immutable map by the time the caller acquires the lock.
	byDate := dmc.byDate.Load()
	v := byDate.get(idbID, date)
	if v.Has(metricID) {
		return true
	}

	// Then check mutable map.
	vMutable := dmc.byDateMutable.get(idbID, date)
	ok := vMutable.Has(metricID)
	if ok {
		dmc.slowHits++
		if dmc.slowHits > (v.Len()+vMutable.Len())/2 {
			// It is cheaper to merge byDateMutable into byDate than to pay inter-cpu sync costs when accessing vMutable.
			dmc.syncLocked()
			dmc.slowHits = 0
		}
	}
	return ok
}

func (dmc *dateMetricIDCache) Set(idbID, date, metricID uint64) {
	dmc.mu.Lock()
	v := dmc.byDateMutable.getOrCreate(idbID, date)
	v.Add(metricID)
	dmc.mu.Unlock()
}

func (dmc *dateMetricIDCache) syncLocked() {
	if len(dmc.byDateMutable.m) == 0 {
		// Nothing to sync.
		return
	}

	// Merge data from byDate into byDateMutable and then atomically replace byDate with the merged data.
	byDate := dmc.byDate.Load()
	byDateMutable := dmc.byDateMutable
	byDateMutable.hotEntry.Store(nil)

	keepDatesMap := make(map[uint64]struct{}, len(byDateMutable.m))
	for k, vMutable := range byDateMutable.m {
		keepDatesMap[k.date] = struct{}{}
		v := byDate.get(k.idbID, k.date)
		if v == nil {
			// Nothing to merge
			continue
		}
		v = v.Clone()
		v.Union(vMutable)
		byDateMutable.m[k] = v
	}

	// Copy entries from byDate, which are missing in byDateMutable
	allDatesMap := make(map[uint64]struct{}, len(byDate.m))
	for k, v := range byDate.m {
		allDatesMap[k.date] = struct{}{}
		vMutable := byDateMutable.get(k.idbID, k.date)
		if vMutable != nil {
			continue
		}
		byDateMutable.m[k] = v
	}

	if len(byDateMutable.m) > 2 {
		// Keep only entries for the last two dates from allDatesMap plus all the entries for byDateMutable.
		dates := make([]uint64, 0, len(allDatesMap))
		for date := range allDatesMap {
			dates = append(dates, date)
		}
		sort.Slice(dates, func(i, j int) bool {
			return dates[i] < dates[j]
		})
		if len(dates) > 2 {
			dates = dates[len(dates)-2:]
		}
		for _, date := range dates {
			keepDatesMap[date] = struct{}{}
		}
		for k := range byDateMutable.m {
			if _, ok := keepDatesMap[k.date]; !ok {
				delete(byDateMutable.m, k)
			}
		}
	}

	// Atomically replace byDate with byDateMutable
	dmc.byDate.Store(dmc.byDateMutable)
	dmc.byDateMutable = newByDateMetricIDMap()

	dmc.syncsCount.Add(1)

	if dmc.SizeBytes() > uint64(memory.Allowed())/256 {
		dmc.resetLocked()
	}
}

// idbDate is used as the key in byDateMetricIDMap.
type idbDate struct {
	idbID uint64
	date  uint64
}

// idbDateMetricIDs holds the date and corresponding metricIDs together and is used
// for implementing hot entry fast path in byDateMetricIDMap.
type idbDateMetricIDs struct {
	k idbDate
	v *uint64set.Set
}

type byDateMetricIDMap struct {
	hotEntry atomic.Pointer[idbDateMetricIDs]
	m        map[idbDate]*uint64set.Set
}

func newByDateMetricIDMap() *byDateMetricIDMap {
	dmm := &byDateMetricIDMap{
		m: make(map[idbDate]*uint64set.Set),
	}
	return dmm
}

func (dmm *byDateMetricIDMap) get(idbID, date uint64) *uint64set.Set {
	hotEntry := dmm.hotEntry.Load()
	if hotEntry != nil && hotEntry.k.idbID == idbID && hotEntry.k.date == date {
		// Fast path
		return hotEntry.v
	}
	// Slow path
	k := idbDate{
		idbID: idbID,
		date:  date,
	}
	v := dmm.m[k]
	if v == nil {
		return nil
	}
	e := &idbDateMetricIDs{k, v}
	dmm.hotEntry.Store(e)
	return v
}

func (dmm *byDateMetricIDMap) getOrCreate(idbID, date uint64) *uint64set.Set {
	v := dmm.get(idbID, date)
	if v != nil {
		return v
	}
	k := idbDate{
		idbID: idbID,
		date:  date,
	}
	v = &uint64set.Set{}
	dmm.m[k] = v
	return v
}
