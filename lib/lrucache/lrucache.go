package lrucache

import (
	"container/heap"
	"sync"
	"sync/atomic"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/timeutil"
)

// Cache caches Entry entries.
//
// If the cache is full the least recently used entries are evicted to make room
// for new entries. Additionally, entries are evicted if not retrieved within
// the last three minutes.
//
// The cache should be used only if the rate of requests is in the range of
// thousands. I.e. it is suitable for use in data retrieval paths and should not
// be used in data ingestion path since its qps is often in the range of
// millions.
//
// Call NewCache() for creating new Cache.
type Cache struct {
	resets   atomic.Uint64
	requests atomic.Uint64
	misses   atomic.Uint64

	// sizeBytes contains an approximate size for all the blocks stored in the cache.
	sizeBytes atomic.Uint64

	// getMaxSizeBytes() is a callback, which returns the maximum allowed cache size in bytes.
	getMaxSizeBytes func() uint64

	// mu protects m and lah
	mu sync.Mutex

	// m contains cached entries
	m map[string]*cacheEntry

	// The heap for removing the least recently used entries from m.
	lah lastAccessHeap

	cleanerMustStopCh chan struct{}
	cleanerStoppedCh  chan struct{}
}

// Entry is an item, which may be cached in the Cache.
type Entry interface {
	// SizeBytes must return the approximate size of the given entry in bytes
	SizeBytes() uint64
}

type cacheEntry struct {
	// The timestamp in seconds for the last access to the given entry.
	lastAccessTime uint64

	// heapIdx is the index for the entry in lastAccessHeap.
	heapIdx int

	// k contains the associated key for the given entry.
	k string

	// e contains the cached entry.
	e Entry
}

// NewCache creates new cache.
//
// Cache size in bytes is limited by the value returned by getMaxSizeBytes()
// callback.
// Call MustStop() in order to free up resources occupied by Cache.
func NewCache(getMaxSizeBytes func() uint64) *Cache {
	c := Cache{
		getMaxSizeBytes:   getMaxSizeBytes,
		m:                 make(map[string]*cacheEntry),
		cleanerMustStopCh: make(chan struct{}),
		cleanerStoppedCh:  make(chan struct{}),
	}
	go c.cleaner()
	return &c
}

// MustStop frees up resources occupied by cache.
func (c *Cache) MustStop() {
	close(c.cleanerMustStopCh)
	<-c.cleanerStoppedCh
}

// Reset resets the cache.
func (c *Cache) Reset() {
	c.resets.Add(1)
	c.mu.Lock()
	defer c.mu.Unlock()

	c.m = make(map[string]*cacheEntry)
	c.lah = nil
	c.sizeBytes.Store(0)
}

func (c *Cache) updateSizeBytes(n uint64) {
	c.sizeBytes.Add(n)
}

func (c *Cache) cleaner() {
	d := timeutil.AddJitterToDuration(time.Second * 53)
	ticker := time.NewTicker(d)
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
	// Delete items accessed more than three minutes ago.
	// This time should be enough for repeated queries.
	lastAccessTime := fasttime.UnixTimestamp() - 3*60
	c.mu.Lock()
	defer c.mu.Unlock()

	for len(c.lah) > 0 {
		if lastAccessTime < c.lah[0].lastAccessTime {
			break
		}
		c.removeLeastRecentlyAccessedItem()
	}
}

// GetEntry returns an Entry for the given key k from c.
func (c *Cache) GetEntry(k string) Entry {
	c.requests.Add(1)
	c.mu.Lock()
	defer c.mu.Unlock()

	ce := c.m[k]
	if ce == nil {
		c.misses.Add(1)
		return nil
	}
	currentTime := fasttime.UnixTimestamp()
	if ce.lastAccessTime != currentTime {
		ce.lastAccessTime = currentTime
		heap.Fix(&c.lah, ce.heapIdx)
	}
	return ce.e
}

// PutEntry puts the given Entry e under the given key k into c.
func (c *Cache) PutEntry(k string, e Entry) {
	c.mu.Lock()
	defer c.mu.Unlock()

	ce := c.m[k]
	if ce != nil {
		// The entry has been already registered by concurrent goroutine.
		return
	}
	ce = &cacheEntry{
		lastAccessTime: fasttime.UnixTimestamp(),
		k:              k,
		e:              e,
	}
	heap.Push(&c.lah, ce)
	c.m[k] = ce
	c.updateSizeBytes(uint64(len(k)) + e.SizeBytes())
	maxSizeBytes := c.getMaxSizeBytes()
	for c.SizeBytes() > maxSizeBytes && len(c.lah) > 0 {
		c.removeLeastRecentlyAccessedItem()
	}
}

func (c *Cache) removeLeastRecentlyAccessedItem() {
	ce := c.lah[0]
	c.updateSizeBytes(-(uint64(len(ce.k)) + ce.e.SizeBytes()))
	delete(c.m, ce.k)
	heap.Pop(&c.lah)
}

// Len returns the number of blocks in the cache c.
func (c *Cache) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.m)
}

// SizeBytes returns an approximate size in bytes of all the blocks stored in the cache c.
func (c *Cache) SizeBytes() uint64 {
	return c.sizeBytes.Load()
}

// SizeMaxBytes returns the max allowed size in bytes for c.
func (c *Cache) SizeMaxBytes() uint64 {
	return c.getMaxSizeBytes()
}

// Requests returns the number of requests served by c since cache creation or
// last reset.
func (c *Cache) Requests() uint64 {
	return c.requests.Load()
}

// Misses returns the number of cache misses for c since cache creation or last
// reset.
func (c *Cache) Misses() uint64 {
	return c.misses.Load()
}

// Resets returns the number of cache resets since its creation.
func (c *Cache) Resets() uint64 {
	return c.resets.Load()
}

// lastAccessHeap implements heap.Interface
type lastAccessHeap []*cacheEntry

func (lah *lastAccessHeap) Len() int {
	return len(*lah)
}
func (lah *lastAccessHeap) Swap(i, j int) {
	h := *lah
	a := h[i]
	b := h[j]
	a.heapIdx = j
	b.heapIdx = i
	h[i] = b
	h[j] = a
}
func (lah *lastAccessHeap) Less(i, j int) bool {
	h := *lah
	return h[i].lastAccessTime < h[j].lastAccessTime
}
func (lah *lastAccessHeap) Push(x any) {
	e := x.(*cacheEntry)
	h := *lah
	e.heapIdx = len(h)
	*lah = append(h, e)
}
func (lah *lastAccessHeap) Pop() any {
	h := *lah
	e := h[len(h)-1]

	// Remove the reference to deleted entry, so Go GC could free up memory
	// occupied by the deleted entry.
	h[len(h)-1] = nil

	*lah = h[:len(h)-1]
	return e
}
