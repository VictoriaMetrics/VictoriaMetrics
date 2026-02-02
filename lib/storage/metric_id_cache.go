package storage

import (
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/atomicutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/cgroup"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/timeutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/uint64set"
)

// metricIDCache stores metricIDs that have been added to the index. It is used
// during data ingestion to decide whether a new entry needs to be added to the
// global index.
//
// The cache consists of multiple shards and avoids synchronization on the read
// path if possible to reduce contention.
type metricIDCache struct {
	shards []metricIDCacheShard

	// The shards are rotated in groups, one group at a time.
	// rotationGroupSize tells the number of shards in one group,
	// rotationGroupCount tells how many groups to rotate, and
	// rotationGroupPeriod tells how often a group is rotated.
	rotationGroupSize   int
	rotationGroupCount  int
	rotationGroupPeriod time.Duration

	stopCh            chan struct{}
	rotationStoppedCh chan struct{}
}

func newMetricIDCache() *metricIDCache {
	// Shards based on the number of CPUs are taken from
	// lib/blockcache/blockcache.go.
	rotationGroupSize := 1
	rotationGroupCount := cgroup.AvailableCPUs()
	if rotationGroupCount > 16 {
		rotationGroupCount = 16
	}
	numShards := rotationGroupSize * rotationGroupCount

	c := metricIDCache{
		shards:              make([]metricIDCacheShard, numShards),
		rotationGroupSize:   rotationGroupSize,
		rotationGroupCount:  rotationGroupCount,
		rotationGroupPeriod: timeutil.AddJitterToDuration(1 * time.Minute),
		stopCh:              make(chan struct{}),
		rotationStoppedCh:   make(chan struct{}),
	}
	for i := range numShards {
		c.shards[i].prev = &uint64set.Set{}
		c.shards[i].next = &uint64set.Set{}
		c.shards[i].curr.Store(&uint64set.Set{})
	}
	go c.startRotation()
	return &c
}

func (c *metricIDCache) MustStop() {
	close(c.stopCh)
	<-c.rotationStoppedCh
}

func (c *metricIDCache) numShards() uint64 {
	return uint64(len(c.shards))
}

func (c *metricIDCache) fullRotationPeriod() time.Duration {
	return time.Duration(c.rotationGroupCount) * c.rotationGroupPeriod
}

func (c *metricIDCache) Stats() metricIDCacheStats {
	var stats metricIDCacheStats
	for i := range len(c.shards) {
		s := c.shards[i].Stats()
		stats.Size += s.Size
		stats.SizeBytes += s.SizeBytes
		stats.SyncsCount += s.SyncsCount
		stats.RotationsCount += s.RotationsCount
	}
	return stats
}

func (c *metricIDCache) Has(metricID uint64) bool {
	shardIdx := fastHashUint64(metricID) % uint64(len(c.shards))
	return c.shards[shardIdx].Has(metricID)
}

func (c *metricIDCache) Set(metricID uint64) {
	shardIdx := fastHashUint64(metricID) % uint64(len(c.shards))
	c.shards[shardIdx].Set(metricID)
}

func (c *metricIDCache) rotate(rotationGroup int) {
	for i := range len(c.shards) {
		if i/c.rotationGroupSize == rotationGroup {
			c.shards[i].rotate()
		}
	}
}

func (c *metricIDCache) startRotation() {
	ticker := time.NewTicker(c.rotationGroupPeriod)
	defer ticker.Stop()
	rotationGroup := 0
	for {
		select {
		case <-c.stopCh:
			close(c.rotationStoppedCh)
			return
		case <-ticker.C:
			// each tick rotate only subset of size metricIDCacheRotationGroupSize
			// to avoid slow access for all shards at once
			rotationGroup = (rotationGroup + 1) % c.rotationGroupCount
			c.rotate(rotationGroup)
		}
	}
}

type metricIDCacheStats struct {
	Size           uint64
	SizeBytes      uint64
	SyncsCount     uint64
	RotationsCount uint64
}

type metricIDCacheShardNopad struct {
	// Contains immutable set of metricIDs.
	curr atomic.Pointer[uint64set.Set]

	// Contains immutable set of metricIDs that used to be current before cache
	// rotation. It is used to implement periodic cache clean-up. Protected by
	// mu.
	prev *uint64set.Set

	// Contains the mutable set of metricIDs that either have been added to the
	// cache recently or migrated from prev. Protected by mu.
	next *uint64set.Set

	// Contains the number of slow accesses to next. Is used for deciding when
	// to merge next to curr. Protected by mu.
	slowHits int

	// Contains the number times the next was merged into curr. Protected by mu.
	syncsCount uint64

	// Contains the number times the cache has been rotated. Protected by mu.
	rotationsCount uint64

	mu sync.Mutex
}

type metricIDCacheShard struct {
	metricIDCacheShardNopad

	// The padding prevents false sharing
	_ [atomicutil.CacheLineSize - unsafe.Sizeof(metricIDCacheShardNopad{})%atomicutil.CacheLineSize]byte
}

func (c *metricIDCacheShard) Stats() metricIDCacheStats {
	c.mu.Lock()
	defer c.mu.Unlock()

	var s metricIDCacheStats
	curr := c.curr.Load()
	s.Size = uint64(curr.Len() + c.prev.Len() + c.next.Len())
	if curr.Len() > 0 {
		// empty uint64set.Set still occupies a few bytes. Ignore them.
		s.SizeBytes = curr.SizeBytes()
	}
	if c.prev.Len() > 0 {
		s.SizeBytes += c.prev.SizeBytes()
	}
	if c.next.Len() > 0 {
		s.SizeBytes += c.next.SizeBytes()
	}
	s.SyncsCount = c.syncsCount
	s.RotationsCount = c.rotationsCount

	return s
}

func (c *metricIDCacheShard) Has(metricID uint64) bool {
	if c.curr.Load().Has(metricID) {
		// Fast path. The majority of calls must go here.
		return true
	}
	// Slow path. Acquire the lock and search the curr again and then also
	// search prev and next.
	return c.hasSlow(metricID)
}

func (c *metricIDCacheShard) hasSlow(metricID uint64) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	// First, check curr again because the entry may have been moved to curr by
	// the time the caller acquires the lock.
	curr := c.curr.Load()
	if curr.Has(metricID) {
		return true
	}

	// Then check next and prev sets.
	ok := c.next.Has(metricID)
	if !ok && c.prev.Has(metricID) {
		// The metricID is in prev but is still in use. Migrate it to next.
		c.next.Add(metricID)
		ok = true
	}

	if ok {
		c.slowHits++
		if c.slowHits > (curr.Len()+c.next.Len())/2 {
			// It is cheaper to merge next into curr than to pay inter-cpu sync
			// costs when accessing next.
			c.syncLocked()
			c.slowHits = 0
		}
	}
	return ok
}

func (c *metricIDCacheShard) Set(metricID uint64) {
	c.mu.Lock()
	c.next.Add(metricID)
	c.mu.Unlock()
}

// syncLocked merges data from curr into next and atomically replaces curr with
// next.
func (c *metricIDCacheShard) syncLocked() {
	curr := c.curr.Load()
	c.next.Union(curr)
	c.curr.Store(c.next)
	c.next = &uint64set.Set{}
	c.syncsCount++
}

// rotate atomically rotates next, curr, and prev cache parts.
func (c *metricIDCacheShard) rotate() {
	c.mu.Lock()
	defer c.mu.Unlock()
	curr := c.curr.Load()
	c.prev = curr
	c.curr.Store(c.next)
	c.next = &uint64set.Set{}
	c.rotationsCount++
}
