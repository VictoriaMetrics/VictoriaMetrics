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
type Cache[T any] struct {
	requests          uint64
	misses            uint64
	cleanerMustStopCh chan struct{}
	cleanerStoppedCh  chan struct{}
	mu                sync.Mutex
	m                 map[string]*cacheEntry[T]
	getMaxSize        func() int
	lah               lastAccessHeap[T]
}

type cacheEntry[T any] struct {
	lastAccessTime uint64
	heapIdx        int

	key   string
	value *T
}

// NewCache returns new Cache with limited entries count
func NewCache[T any](getMaxSize func() int) *Cache[T] {
	c := &Cache[T]{
		m:                 make(map[string]*cacheEntry[T]),
		getMaxSize:        getMaxSize,
		cleanerStoppedCh:  make(chan struct{}),
		cleanerMustStopCh: make(chan struct{}),
	}
	go c.cleaner()
	return c
}

// MustStop frees up resources occupied by c.
func (c *Cache[T]) MustStop() {
	close(c.cleanerMustStopCh)
	<-c.cleanerStoppedCh
}

func (c *Cache[T]) cleaner() {
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

func (c *Cache[T]) cleanByTimeout() {
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

// Requests returns cache requests
func (c *Cache[T]) Requests() uint64 {
	return atomic.LoadUint64(&c.requests)
}

// Misses returns cache misses
func (c *Cache[T]) Misses() uint64 {
	return atomic.LoadUint64(&c.misses)
}

// Len returns number of entries at cache
func (c *Cache[T]) Len() int {
	c.mu.Lock()
	n := len(c.m)
	c.mu.Unlock()
	return n
}

// Put puts the given value under the given key k into c,
// but only if the key k was already requested via Get at least twice.
func (c *Cache[T]) Put(key string, value *T) {
	c.mu.Lock()
	defer c.mu.Unlock()
	entry := &cacheEntry[T]{
		lastAccessTime: fasttime.UnixTimestamp(),
		key:            key,
		value:          value,
	}
	heap.Push(&c.lah, entry)
	c.m[key] = entry
	maxSize := c.getMaxSize()
	// release 10% of current cache capacity
	if overflow := len(c.m); overflow > maxSize {
		overflow = int(0.1 * float64(maxSize))
		for overflow > 0 {
			e := c.lah[0]
			delete(c.m, e.key)
			heap.Pop(&c.lah)
			overflow--
		}
	}
}

// Get returns cached value for the given key k from c and updates access time.
func (c *Cache[T]) Get(key string) *T {
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

type lastAccessHeap[T any] []*cacheEntry[T]

// Len implements interface
func (lah *lastAccessHeap[T]) Len() int {
	return len(*lah)
}

// Swap  implements interface
func (lah *lastAccessHeap[T]) Swap(i, j int) {
	h := *lah
	a := h[i]
	b := h[j]
	a.heapIdx = j
	b.heapIdx = i
	h[i] = b
	h[j] = a
}

// Less  implements interface
func (lah *lastAccessHeap[T]) Less(i, j int) bool {
	h := *lah
	return h[i].lastAccessTime < h[j].lastAccessTime
}

// Push  implements interface
func (lah *lastAccessHeap[T]) Push(x interface{}) {
	e := x.(*cacheEntry[T])
	h := *lah
	e.heapIdx = len(h)
	*lah = append(h, e)
}

// Pop  implements interface
func (lah *lastAccessHeap[T]) Pop() interface{} {
	h := *lah
	e := h[len(h)-1]
	// Remove the reference to deleted entry, so Go GC could free up memory occupied by the deleted entry.
	h[len(h)-1] = nil
	*lah = h[:len(h)-1]
	return e
}
