package storage

import (
	"slices"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/atomicutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/timeutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/uint64set"
)

const (
	dateMetricIDCacheShardCount = 16

	// The number of consecutive metricIDs that will be stored in one shard.
	// This is 2^16 and corresponds to the size of a 16-bit bucket of the
	// uint64set. That way the metricIDs end up in one uint64set bucket instead
	// of being spread across multiple buckets. This reduces the memory size of
	// the cache and allows for faster access.
	dateMetricIDCacheShardBucketSize = 65536
)

// dateMetricIDCache stores (date, metricIDs) entries that have been added to
// the index. It is used during data ingestion to decide whether a new entry
// needs to be added to the per-day index.
//
// It should be faster than map[date]*uint64set.Set on multicore systems.
type dateMetricIDCache struct {
	shards [dateMetricIDCacheShardCount]dateMetricIDCacheShard

	// The shards are rotated, one shard at a time. rotationPeriod defines the
	// time interval between two successive rotations.
	rotationPeriod time.Duration

	stopCh            chan struct{}
	rotationStoppedCh chan struct{}
}

func newDateMetricIDCache() *dateMetricIDCache {
	c := dateMetricIDCache{
		rotationPeriod:    timeutil.AddJitterToDuration(1 * time.Hour),
		stopCh:            make(chan struct{}),
		rotationStoppedCh: make(chan struct{}),
	}
	for i := range dateMetricIDCacheShardCount {
		c.shards[i].prev = newByDateMetricIDMap()
		c.shards[i].next = newByDateMetricIDMap()
		c.shards[i].curr.Store(newByDateMetricIDMap())
	}
	go c.startRotation()
	return &c
}

func (dmc *dateMetricIDCache) MustStop() {
	close(dmc.stopCh)
	<-dmc.rotationStoppedCh
}

func (c *dateMetricIDCache) Stats() dateMetricIDCacheStats {
	var stats dateMetricIDCacheStats
	for i := range dateMetricIDCacheShardCount {
		s := c.shards[i].Stats()
		stats.Size += s.Size
		stats.SizeBytes += s.SizeBytes
		stats.SyncsCount += s.SyncsCount
		stats.RotationsCount += s.RotationsCount
	}
	return stats
}

func (c *dateMetricIDCache) Has(date, metricID uint64) bool {
	shardIdx := (metricID / dateMetricIDCacheShardBucketSize) % dateMetricIDCacheShardCount
	return c.shards[shardIdx].Has(date, metricID)
}

func (c *dateMetricIDCache) Set(date, metricID uint64) {
	shardIdx := (metricID / dateMetricIDCacheShardBucketSize) % dateMetricIDCacheShardCount
	c.shards[shardIdx].Set(date, metricID)
}

func (c *dateMetricIDCache) startRotation() {
	ticker := time.NewTicker(c.rotationPeriod)
	defer ticker.Stop()
	var shardIdx int
	for {
		select {
		case <-c.stopCh:
			close(c.rotationStoppedCh)
			return
		case <-ticker.C:
			// Each tick rotate only one shard at a time to avoid slow access
			// for all shards at once.
			shardIdx %= dateMetricIDCacheShardCount
			c.shards[shardIdx].rotate()
			shardIdx++
		}
	}
}

type dateMetricIDCacheStats struct {
	Size           uint64
	SizeBytes      uint64
	SyncsCount     uint64
	RotationsCount uint64
}

type dateMetricIDCacheShardNopad struct {
	// Contains immutable (date, metricIDs) entries.
	curr atomic.Pointer[byDateMetricIDMap]

	// Contains immutable (date, metricIDs) entries that used to be current
	// before cache rotation. It is used to implement periodic cache clean-up.
	// Protected by mu.
	prev *byDateMetricIDMap

	// Contains mutable (date metricIDs) entries that either have been added to
	// the cache recently or migrated from prev. Protected by mu.
	next *byDateMetricIDMap

	// Contains the number of slow accesses to next. Is used for deciding when
	// to merge next to curr. Protected by mu.
	slowHits int

	// Contains the number times the next was merged into curr. Protected by mu.
	syncsCount uint64

	// Contains the number times the cache has been rotated. Protected by mu.
	rotationsCount uint64

	mu sync.Mutex
}

type dateMetricIDCacheShard struct {
	dateMetricIDCacheShardNopad

	// The padding prevents false sharing
	_ [atomicutil.CacheLineSize - unsafe.Sizeof(dateMetricIDCacheShardNopad{})%atomicutil.CacheLineSize]byte
}

func (dmc *dateMetricIDCacheShard) Stats() dateMetricIDCacheStats {
	dmc.mu.Lock()
	defer dmc.mu.Unlock()

	var s dateMetricIDCacheStats
	for _, metricIDs := range dmc.curr.Load().m {
		if metricIDs.Len() > 0 {
			// empty uint64set.Set still occupies a few bytes. Ignore them.
			s.Size += uint64(metricIDs.Len())
			s.SizeBytes += metricIDs.SizeBytes()
		}
	}
	for _, metricIDs := range dmc.prev.m {
		if metricIDs.Len() > 0 {
			s.Size += uint64(metricIDs.Len())
			s.SizeBytes += metricIDs.SizeBytes()
		}
	}
	for _, metricIDs := range dmc.next.m {
		if metricIDs.Len() > 0 {
			s.Size += uint64(metricIDs.Len())
			s.SizeBytes += metricIDs.SizeBytes()
		}
	}
	s.SyncsCount = dmc.syncsCount
	s.RotationsCount = dmc.rotationsCount

	return s
}

func (dmc *dateMetricIDCacheShard) Has(date, metricID uint64) bool {
	curr := dmc.curr.Load()
	vCurr := curr.get(date)
	if vCurr.Has(metricID) {
		// Fast path. The majority of calls must go here.
		return true
	}
	// Slow path. Acquire the lock and search the immutable map again and then
	// also search the mutable map.
	return dmc.hasSlow(date, metricID)
}

func (dmc *dateMetricIDCacheShard) hasSlow(date, metricID uint64) bool {
	dmc.mu.Lock()
	defer dmc.mu.Unlock()

	// First, check immutable map again because the entry may have been moved to
	// the immutable map by the time the caller acquires the lock.
	curr := dmc.curr.Load()
	vCurr := curr.get(date)
	if vCurr.Has(metricID) {
		return true
	}

	// Then check next and prev.
	vNext := dmc.next.getOrCreate(date)
	ok := vNext.Has(metricID)
	if !ok {
		vPrev := dmc.prev.get(date)
		ok = vPrev.Has(metricID)
		if ok {
			// The metricID is in prev but is still in use. Migrate it to next.
			vNext.Add(metricID)
		}
	}

	if ok {
		dmc.slowHits++
		if dmc.slowHits > (vCurr.Len()+vNext.Len())/2 {
			// It is cheaper to merge next into curr than to pay inter-cpu sync
			// costs when accessing next.
			dmc.syncLocked()
			dmc.slowHits = 0
		}
	}
	return ok
}

func (dmc *dateMetricIDCacheShard) Set(date, metricID uint64) {
	dmc.mu.Lock()
	v := dmc.next.getOrCreate(date)
	v.Add(metricID)
	dmc.mu.Unlock()
}

func (dmc *dateMetricIDCacheShard) syncLocked() {
	if len(dmc.next.m) == 0 {
		// Nothing to sync.
		return
	}

	// Merge data from curr into next and then atomically replace curr with the
	// merged data.
	curr := dmc.curr.Load()
	next := dmc.next
	next.hotEntry.Store(nil)

	keepDatesMap := make(map[uint64]struct{}, len(next.m))
	for date, vNext := range next.m {
		keepDatesMap[date] = struct{}{}
		vCurr := curr.get(date)
		if vCurr == nil {
			// Nothing to merge
			continue
		}
		vCurr = vCurr.Clone()
		vCurr.Union(vNext)
		next.m[date] = vCurr
	}

	// Copy entries from curr, which are missing in next
	allDatesMap := make(map[uint64]struct{}, len(curr.m))
	for date, vCurr := range curr.m {
		allDatesMap[date] = struct{}{}
		vNext := next.get(date)
		if vNext != nil {
			continue
		}
		next.m[date] = vCurr
	}

	if len(next.m) > 2 {
		// Keep only entries for the last two dates from allDatesMap plus all
		// the entries for next.
		dates := make([]uint64, 0, len(allDatesMap))
		for date := range allDatesMap {
			dates = append(dates, date)
		}
		slices.Sort(dates)
		if len(dates) > 2 {
			dates = dates[len(dates)-2:]
		}
		for _, date := range dates {
			keepDatesMap[date] = struct{}{}
		}
		for date := range next.m {
			if _, ok := keepDatesMap[date]; !ok {
				delete(next.m, date)
			}
		}
	}

	// Atomically replace curr with next.
	dmc.curr.Store(dmc.next)
	dmc.next = newByDateMetricIDMap()

	dmc.syncsCount++
}

// rotate atomically rotates next, curr, and prev cache parts.
func (dmc *dateMetricIDCacheShard) rotate() {
	dmc.mu.Lock()
	defer dmc.mu.Unlock()
	curr := dmc.curr.Load()
	dmc.prev = curr
	dmc.curr.Store(dmc.next)
	dmc.next = newByDateMetricIDMap()
	dmc.rotationsCount++
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
