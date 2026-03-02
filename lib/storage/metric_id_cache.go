package storage

import (
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/atomicutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/timeutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/uint64set"
)

const (
	metricIDCacheShardCount = 16

	// The number of consecutive metricIDs that will be stored in a single cache
	// shard. This is 2^16 and corresponds to the size of a 16-bit bucket of the
	// uint64set. That way the metricIDs end up in one uint64set bucket instead
	// of being spread across multiple buckets. This reduces the memory size of
	// the cache and allows for faster access.
	metricIDCacheShardBucketSize = 65536
)

// metricIDCache stores metricIDs that have been added to the index. It is used
// during data ingestion to decide whether a new entry needs to be added to the
// global index.
//
// The cache consists of multiple shards and avoids synchronization on the read
// path if possible to reduce contention.
type metricIDCache struct {
	shards [metricIDCacheShardCount]metricIDCacheShard

	// The shards are rotated, one shard at a time. rotationPeriod defines the
	// time interval between two successive rotations.
	rotationPeriod time.Duration

	stopCh            chan struct{}
	rotationStoppedCh chan struct{}
}

func newMetricIDCache() *metricIDCache {
	c := metricIDCache{
		rotationPeriod:    timeutil.AddJitterToDuration(1 * time.Minute),
		stopCh:            make(chan struct{}),
		rotationStoppedCh: make(chan struct{}),
	}
	for i := range metricIDCacheShardCount {
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

func (c *metricIDCache) Stats() metricIDCacheStats {
	var stats metricIDCacheStats
	for i := range metricIDCacheShardCount {
		s := c.shards[i].Stats()
		stats.Size += s.Size
		stats.SizeBytes += s.SizeBytes
		stats.SyncsCount += s.SyncsCount
		stats.RotationsCount += s.RotationsCount
	}
	return stats
}

func (c *metricIDCache) Has(metricID uint64) bool {
	shardIdx := (metricID / metricIDCacheShardBucketSize) % metricIDCacheShardCount
	return c.shards[shardIdx].Has(metricID)
}

func (c *metricIDCache) Set(metricID uint64) {
	shardIdx := (metricID / metricIDCacheShardBucketSize) % metricIDCacheShardCount
	c.shards[shardIdx].Set(metricID)
}

func (c *metricIDCache) startRotation() {
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
			shardIdx %= metricIDCacheShardCount
			c.shards[shardIdx].rotate()
			shardIdx++
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
