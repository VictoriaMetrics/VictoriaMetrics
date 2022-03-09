package lrucache

import (
	"container/heap"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
)

// Cache caches any values with limited number of entries
// evicted in LRU order.
// Cache must be created only via NewCache function
type Cache struct {
	requests          uint64
	misses            uint64
	sizeBytes         int64
	cleanerMustStopCh chan struct{}
	cleanerStoppedCh  chan struct{}
	mu                sync.Mutex
	m                 map[string]*cacheEntry
	getMaxSizeBytes   func() int
	lah               lastAccessHeap
}

// SizedValue value with known size in bytes
type SizedValue interface {
	SizeBytes() int
}

type cacheEntry struct {
	lastAccessTime uint64
	heapIdx        int

	key   string
	value SizedValue
}

// NewCache returns new Cache with limited entries count
func NewCache(getMaxSizeBytes func() int) *Cache {
	c := &Cache{
		m:                 make(map[string]*cacheEntry),
		getMaxSizeBytes:   getMaxSizeBytes,
		cleanerStoppedCh:  make(chan struct{}),
		cleanerMustStopCh: make(chan struct{}),
	}
	go c.cleaner()
	return c
}

// MustStop frees up resources occupied by c.
func (c *Cache) MustStop() {
	close(c.cleanerMustStopCh)
	<-c.cleanerStoppedCh
}

func (c *Cache) cleaner() {
	// clean-up internal chosen randomly in 45-55 seconds range
	// it should shift background jobs
	tick := rand.Intn(10) + 45
	ticker := time.NewTicker(time.Duration(tick) * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-c.cleanerMustStopCh:
			close(c.cleanerStoppedCh)
			return
		case <-ticker.C:
			c.cleanByTimeout()
		}
	}
}

func (c *Cache) cleanByTimeout() {
	// Delete items accessed more than four minutes ago.
	// This time should be enough for repeated queries.
	lastAccessTime := fasttime.UnixTimestamp() - 4*60
	c.mu.Lock()
	defer c.mu.Unlock()

	for len(c.lah) > 0 {
		e := c.lah[0]
		if lastAccessTime < e.lastAccessTime {
			break
		}
		delete(c.m, e.key)
		heap.Pop(&c.lah)
	}
}

func (c *Cache) updateSizeBytes(n int) {
	atomic.AddInt64(&c.sizeBytes, int64(n))
}

// SizeBytes returns current cache size in bytes
func (c *Cache) SizeBytes() int {
	return int(atomic.LoadInt64(&c.sizeBytes))
}

// SizeMaxBytes returns max cache size in bytes
func (c *Cache) SizeMaxBytes() int {
	return c.getMaxSizeBytes()
}

// Requests returns cache requests
func (c *Cache) Requests() uint64 {
	return atomic.LoadUint64(&c.requests)
}

// Misses returns cache misses
func (c *Cache) Misses() uint64 {
	return atomic.LoadUint64(&c.misses)
}

// Len returns number of entries at cache
func (c *Cache) Len() int {
	c.mu.Lock()
	n := len(c.m)
	c.mu.Unlock()
	return n
}

// Put puts the given value under the given key k into c
func (c *Cache) Put(key string, value SizedValue) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.m[key] != nil {
		// fast path, key already in cache
		return
	}
	entry := &cacheEntry{
		lastAccessTime: fasttime.UnixTimestamp(),
		key:            key,
		value:          value,
	}
	c.updateSizeBytes(value.SizeBytes())
	heap.Push(&c.lah, entry)
	c.m[key] = entry
	maxSizeBytes := c.getMaxSizeBytes()
	for c.SizeBytes() > maxSizeBytes && len(c.lah) > 0 {
		e := c.lah[0]
		delete(c.m, e.key)
		c.updateSizeBytes(-e.value.SizeBytes())
		heap.Pop(&c.lah)
	}
}

// Get returns cached value for the given key k from c and updates access time.
func (c *Cache) Get(key string) SizedValue {
	atomic.AddUint64(&c.requests, 1)
	c.mu.Lock()
	defer c.mu.Unlock()
	if e, ok := c.m[key]; ok {
		currentTime := fasttime.UnixTimestamp()
		if e.lastAccessTime != currentTime {
			e.lastAccessTime = currentTime
			heap.Fix(&c.lah, e.heapIdx)
		}
		return e.value
	}

	atomic.AddUint64(&c.misses, 1)
	return nil
}

type lastAccessHeap []*cacheEntry

// Len implements interface
func (lah *lastAccessHeap) Len() int {
	return len(*lah)
}

// Swap  implements interface
func (lah *lastAccessHeap) Swap(i, j int) {
	h := *lah
	a := h[i]
	b := h[j]
	a.heapIdx = j
	b.heapIdx = i
	h[i] = b
	h[j] = a
}

// Less  implements interface
func (lah *lastAccessHeap) Less(i, j int) bool {
	h := *lah
	return h[i].lastAccessTime < h[j].lastAccessTime
}

// Push  implements interface
func (lah *lastAccessHeap) Push(x interface{}) {
	e := x.(*cacheEntry)
	h := *lah
	e.heapIdx = len(h)
	*lah = append(h, e)
}

// Pop  implements interface
func (lah *lastAccessHeap) Pop() interface{} {
	h := *lah
	e := h[len(h)-1]
	// Remove the reference to deleted entry, so Go GC could free up memory occupied by the deleted entry.
	h[len(h)-1] = nil
	*lah = h[:len(h)-1]
	return e
}
