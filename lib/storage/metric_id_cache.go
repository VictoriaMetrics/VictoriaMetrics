package storage

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/timeutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/uint64set"
)

// metricIDCache stores metricIDs that have been added to the index. It is used
// during data ingestion to decide whether a new entry needs to be added to the
// global index.
//
// The cache avoids synchronization on the read path if possible to reduce
// contention. Based on dateMetricIDCache ideas.
type metricIDCache struct {
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

	// Contains the number times the cache was rotated. Protected by mu.
	syncsCount uint64

	mu sync.Mutex

	stopCh           chan struct{}
	cleanerStoppedCh chan struct{}
}

func newMetricIDCache() *metricIDCache {
	c := metricIDCache{
		prev:             &uint64set.Set{},
		next:             &uint64set.Set{},
		stopCh:           make(chan struct{}),
		cleanerStoppedCh: make(chan struct{}),
	}
	c.curr.Store(&uint64set.Set{})
	go c.startCleaner()
	return &c
}

func (c *metricIDCache) Stop() {
	close(c.stopCh)
	<-c.cleanerStoppedCh
}

type metricIDCacheStats struct {
	Size       uint64
	SizeBytes  uint64
	SyncsCount uint64
}

func (c *metricIDCache) Stats() metricIDCacheStats {
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

	return s
}

func (c *metricIDCache) Has(metricID uint64) bool {
	if c.curr.Load().Has(metricID) {
		// Fast path. The majority of calls must go here.
		return true
	}
	// Slow path. Acquire the lock and search the curr again and then also
	// search prev and next.
	return c.hasSlow(metricID)
}

func (c *metricIDCache) hasSlow(metricID uint64) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	// First, check curr again because the entry may have been moved to curr by
	// the time the caller acquires the lock.
	curr := c.curr.Load()
	if curr.Has(metricID) {
		return true
	}

	// Then check prev and next sets.
	var ok bool
	next := c.next
	if c.prev.Has(metricID) {
		// the metricID is in prev but is still in use. Thus, migrate it to
		// next.
		next.Add(metricID)
		ok = true
	} else {
		ok = next.Has(metricID)
	}

	if ok {
		c.slowHits++
		if c.slowHits > (curr.Len()+next.Len())/2 {
			// It is cheaper to merge next into curr than to pay inter-cpu sync
			// costs when accessing next.
			c.syncLocked()
			c.slowHits = 0
		}
	}
	return ok
}

func (c *metricIDCache) Set(metricID uint64) {
	c.mu.Lock()
	v := c.next
	v.Add(metricID)
	c.mu.Unlock()
}

func (c *metricIDCache) syncLocked() {
	// Merge data from curr into next.
	curr := c.curr.Load()
	if c.next.Len() > 0 {
		c.next.Union(curr)
	}

	// Atomically replace curr with next and prev with curr.
	c.curr.Store(c.next)
	c.prev = curr
	c.next = &uint64set.Set{}

	c.syncsCount++
}

func (c *metricIDCache) startCleaner() {
	d := timeutil.AddJitterToDuration(time.Hour)
	ticker := time.NewTicker(d)
	defer ticker.Stop()
	for {
		select {
		case <-c.stopCh:
			close(c.cleanerStoppedCh)
			return
		case <-ticker.C:
			c.mu.Lock()
			c.syncLocked()
			c.mu.Unlock()
		}
	}
}
