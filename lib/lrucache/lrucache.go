package lrucache

import (
	"container/heap"
	"sync"
	"sync/atomic"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/cgroup"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/timeutil"
	"github.com/cespare/xxhash/v2"
)

// Cache caches Entry entries.
//
// Call NewCache() for creating new Cache.
type Cache struct {
	shards []*cache

	cleanerMustStopCh chan struct{}
	cleanerStoppedCh  chan struct{}
}

// NewCache creates new cache.
//
// Cache size in bytes is limited by the value returned by getMaxSizeBytes() callback.
// Call MustStop() in order to free up resources occupied by Cache.
func NewCache(getMaxSizeBytes func() int) *Cache {
	cpusCount := cgroup.AvailableCPUs()
	shardsCount := cgroup.AvailableCPUs()
	// Increase the number of shards with the increased number of available CPU cores.
	// This should reduce contention on per-shard mutexes.
	multiplier := cpusCount
	if multiplier > 16 {
		multiplier = 16
	}
	shardsCount *= multiplier
	shards := make([]*cache, shardsCount)
	getMaxShardBytes := func() int {
		n := getMaxSizeBytes()
		return n / shardsCount
	}
	for i := range shards {
		shards[i] = newCache(getMaxShardBytes)
	}
	c := &Cache{
		shards:            shards,
		cleanerMustStopCh: make(chan struct{}),
		cleanerStoppedCh:  make(chan struct{}),
	}
	go c.cleaner()
	return c
}

// MustStop frees up resources occupied by c.
func (c *Cache) MustStop() {
	close(c.cleanerMustStopCh)
	<-c.cleanerStoppedCh
}

// GetEntry returns an Entry for the given key k from c.
func (c *Cache) GetEntry(k string) Entry {
	idx := uint64(0)
	if len(c.shards) > 1 {
		h := hashUint64(k)
		idx = h % uint64(len(c.shards))
	}
	shard := c.shards[idx]
	return shard.GetEntry(k)
}

// PutEntry puts the given Entry e under the given key k into c.
func (c *Cache) PutEntry(k string, e Entry) {
	idx := uint64(0)
	if len(c.shards) > 1 {
		h := hashUint64(k)
		idx = h % uint64(len(c.shards))
	}
	shard := c.shards[idx]
	shard.PutEntry(k, e)
}

// Len returns the number of blocks in the cache c.
func (c *Cache) Len() int {
	n := 0
	for _, shard := range c.shards {
		n += shard.Len()
	}
	return n
}

// SizeBytes returns an approximate size in bytes of all the blocks stored in the cache c.
func (c *Cache) SizeBytes() int {
	n := 0
	for _, shard := range c.shards {
		n += shard.SizeBytes()
	}
	return n
}

// SizeMaxBytes returns the max allowed size in bytes for c.
func (c *Cache) SizeMaxBytes() int {
	n := 0
	for _, shard := range c.shards {
		n += shard.SizeMaxBytes()
	}
	return n
}

// Requests returns the number of requests served by c.
func (c *Cache) Requests() uint64 {
	n := uint64(0)
	for _, shard := range c.shards {
		n += shard.Requests()
	}
	return n
}

// Misses returns the number of cache misses for c.
func (c *Cache) Misses() uint64 {
	n := uint64(0)
	for _, shard := range c.shards {
		n += shard.Misses()
	}
	return n
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
	for _, shard := range c.shards {
		shard.cleanByTimeout()
	}
}

type cache struct {
	requests atomic.Uint64
	misses   atomic.Uint64

	// sizeBytes contains an approximate size for all the blocks stored in the cache.
	sizeBytes atomic.Int64

	// getMaxSizeBytes() is a callback, which returns the maximum allowed cache size in bytes.
	getMaxSizeBytes func() int

	// mu protects all the fields below.
	mu sync.Mutex

	// m contains cached entries
	m map[string]*cacheEntry

	// The heap for removing the least recently used entries from m.
	lah lastAccessHeap
}

func hashUint64(s string) uint64 {
	b := bytesutil.ToUnsafeBytes(s)
	return xxhash.Sum64(b)
}

// Entry is an item, which may be cached in the Cache.
type Entry interface {
	// SizeBytes must return the approximate size of the given entry in bytes
	SizeBytes() int
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

func newCache(getMaxSizeBytes func() int) *cache {
	var c cache
	c.getMaxSizeBytes = getMaxSizeBytes
	c.m = make(map[string]*cacheEntry)
	return &c
}

func (c *cache) updateSizeBytes(n int) {
	c.sizeBytes.Add(int64(n))
}

func (c *cache) cleanByTimeout() {
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

func (c *cache) GetEntry(k string) Entry {
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

func (c *cache) PutEntry(k string, e Entry) {
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
	c.updateSizeBytes(e.SizeBytes())
	maxSizeBytes := c.getMaxSizeBytes()
	for c.SizeBytes() > maxSizeBytes && len(c.lah) > 0 {
		c.removeLeastRecentlyAccessedItem()
	}
}

func (c *cache) removeLeastRecentlyAccessedItem() {
	ce := c.lah[0]
	c.updateSizeBytes(-ce.e.SizeBytes())
	delete(c.m, ce.k)
	heap.Pop(&c.lah)
}

func (c *cache) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.m)
}

func (c *cache) SizeBytes() int {
	return int(c.sizeBytes.Load())
}

func (c *cache) SizeMaxBytes() int {
	return c.getMaxSizeBytes()
}

func (c *cache) Requests() uint64 {
	return c.requests.Load()
}

func (c *cache) Misses() uint64 {
	return c.misses.Load()
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

	// Remove the reference to deleted entry, so Go GC could free up memory occupied by the deleted entry.
	h[len(h)-1] = nil

	*lah = h[:len(h)-1]
	return e
}
