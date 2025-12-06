package storage

import (
	"sort"
	"sync"
	"sync/atomic"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/memory"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/uint64set"
)

var maxDateMetricIDCacheSize uint64

// SetDateMetricIDCacheSize overrides the default size of dateMetricIDCache
func SetDateMetricIDCacheSize(size int) {
	maxDateMetricIDCacheSize = uint64(size)
}

func getDateMetricIDCacheSize() uint64 {
	if maxDateMetricIDCacheSize <= 0 {
		return uint64(float64(memory.Allowed()) / 256)
	}
	return maxDateMetricIDCacheSize
}

// dateMetricIDCache is fast cache for holding (date, metricID) entries.
//
// It should be faster than map[date]*uint64set.Set on multicore systems.
type dateMetricIDCache struct {
	// Contains immutable map
	byDate atomic.Pointer[byDateMetricIDMap]

	// Contains mutable map protected by mu
	byDateMutable *byDateMetricIDMap

	// Contains the number of slow accesses to byDateMutable.
	// Is used for deciding when to merge byDateMutable to byDate.
	// Protected by mu.
	slowHits int

	// Protected by mu.
	syncsCount uint64

	// Protected by mu.
	resetsCount uint64

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

	dmc.resetsCount++
}

type dateMetricIDCacheStats struct {
	Size         uint64
	SizeBytes    uint64
	SizeMaxBytes uint64
	ResetsCount  uint64
	SyncsCount   uint64
}

func (dmc *dateMetricIDCache) Stats() dateMetricIDCacheStats {
	s := dateMetricIDCacheStats{
		SizeMaxBytes: getDateMetricIDCacheSize(),
	}

	dmc.mu.Lock()
	defer dmc.mu.Unlock()

	for _, metricIDs := range dmc.byDate.Load().m {
		s.Size += uint64(metricIDs.Len())
		s.SizeBytes += metricIDs.SizeBytes()
	}
	for _, metricIDs := range dmc.byDateMutable.m {
		s.Size += uint64(metricIDs.Len())
		s.SizeBytes += metricIDs.SizeBytes()
	}

	s.ResetsCount = dmc.resetsCount
	s.SyncsCount = dmc.syncsCount

	return s
}

func (dmc *dateMetricIDCache) Has(date, metricID uint64) bool {
	if byDate := dmc.byDate.Load(); byDate.get(date).Has(metricID) {
		// Fast path. The majority of calls must go here.
		return true
	}
	// Slow path. Acquire the lock and search the immutable map again and then
	// also search the mutable map.
	return dmc.hasSlow(date, metricID)
}

func (dmc *dateMetricIDCache) hasSlow(date, metricID uint64) bool {
	dmc.mu.Lock()
	defer dmc.mu.Unlock()

	// First, check immutable map again because the entry may have been moved to
	// the immutable map by the time the caller acquires the lock.
	byDate := dmc.byDate.Load()
	v := byDate.get(date)
	if v.Has(metricID) {
		return true
	}

	// Then check mutable map.
	vMutable := dmc.byDateMutable.get(date)
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

func (dmc *dateMetricIDCache) Set(date, metricID uint64) {
	dmc.mu.Lock()
	v := dmc.byDateMutable.getOrCreate(date)
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
	for date, metricIDsMutable := range byDateMutable.m {
		keepDatesMap[date] = struct{}{}
		metricIDs := byDate.get(date)
		if metricIDs == nil {
			// Nothing to merge
			continue
		}
		metricIDs = metricIDs.Clone()
		metricIDs.Union(metricIDsMutable)
		byDateMutable.m[date] = metricIDs
	}

	// Copy entries from byDate, which are missing in byDateMutable
	allDatesMap := make(map[uint64]struct{}, len(byDate.m))
	for date, metricIDs := range byDate.m {
		allDatesMap[date] = struct{}{}
		v := byDateMutable.get(date)
		if v != nil {
			continue
		}
		byDateMutable.m[date] = metricIDs
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
		for date := range byDateMutable.m {
			if _, ok := keepDatesMap[date]; !ok {
				delete(byDateMutable.m, date)
			}
		}
	}

	var sizeBytes uint64
	for _, v := range dmc.byDateMutable.m {
		sizeBytes += v.SizeBytes()
	}

	// Atomically replace byDate with byDateMutable
	dmc.byDate.Store(dmc.byDateMutable)
	dmc.byDateMutable = newByDateMetricIDMap()

	dmc.syncsCount++

	if sizeBytes > getDateMetricIDCacheSize() {
		dmc.resetLocked()
	}
}

// dateMetricIDs holds the date and corresponding metricIDs together and is used
// for implementing hot entry fast path in byDateMetricIDMap.
type dateMetricIDs struct {
	date      uint64
	metricIDs *uint64set.Set
}

type byDateMetricIDMap struct {
	hotEntry atomic.Pointer[dateMetricIDs]
	m        map[uint64]*uint64set.Set
}

func newByDateMetricIDMap() *byDateMetricIDMap {
	dmm := &byDateMetricIDMap{
		m: make(map[uint64]*uint64set.Set),
	}
	return dmm
}

func (dmm *byDateMetricIDMap) get(date uint64) *uint64set.Set {
	hotEntry := dmm.hotEntry.Load()
	if hotEntry != nil && hotEntry.date == date {
		// Fast path
		return hotEntry.metricIDs
	}
	// Slow path
	metricIDs := dmm.m[date]
	if metricIDs == nil {
		return nil
	}
	e := &dateMetricIDs{
		date:      date,
		metricIDs: metricIDs,
	}
	dmm.hotEntry.Store(e)
	return metricIDs
}

func (dmm *byDateMetricIDMap) getOrCreate(date uint64) *uint64set.Set {
	metricIDs := dmm.get(date)
	if metricIDs != nil {
		return metricIDs
	}
	metricIDs = &uint64set.Set{}
	dmm.m[date] = metricIDs
	return metricIDs
}
