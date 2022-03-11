package regexpcache

import (
	"container/heap"
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
	cleanerMustStopCh chan struct{}
	cleanerStoppedCh  chan struct{}
	mu                sync.Mutex
	m                 map[string]*cacheEntry
	perKeyMisses      map[string]int
	getMaxSize        func() int
	lah               lastAccessHeap
}

type cacheEntry struct {
	lastAccessTime uint64
	heapIdx        int

	key   string
	value interface{}
}

// NewCache returns new Cache with limited entries count
func NewCache(getMaxSize func() int) *Cache {
	c := &Cache{
		m:                 make(map[string]*cacheEntry),
		perKeyMisses:      make(map[string]int),
		getMaxSize:        getMaxSize,
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
	ticker := time.NewTicker(55 * time.Second)
	defer ticker.Stop()
	perKeyMissesTicker := time.NewTicker(4 * time.Minute)
	defer perKeyMissesTicker.Stop()
	for {
		select {
		case <-c.cleanerMustStopCh:
			close(c.cleanerStoppedCh)
			return
		case <-ticker.C:
			c.cleanByTimeout()
		case <-perKeyMissesTicker.C:
			c.cleanPerKeyMisses()
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

func (c *Cache) cleanPerKeyMisses() {
	c.mu.Lock()
	c.perKeyMisses = make(map[string]int, len(c.perKeyMisses))
	c.mu.Unlock()
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

// Put puts the given value under the given key k into c,
// but only if the key k was already requested via Get at least twice. 
func (c *Cache) Put(key string, value interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.perKeyMisses[key] == 1 {
		// Do not cache entry if it has been requested only once (aka one-time-wonders items).
		// This should reduce memory usage for the cache.
		return
	}
	entry := &cacheEntry{
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
func (c *Cache) Get(key string) interface{} {
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

	// Slow path - the entry is missing in the cache.
	c.perKeyMisses[key]++
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
	*lah = h[:len(h)-1]
	return e
}
