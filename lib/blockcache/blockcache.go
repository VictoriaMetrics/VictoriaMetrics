package blockcache

import (
	"container/heap"
	"flag"
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/cgroup"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/timeutil"
	"github.com/cespare/xxhash/v2"
)

var missesBeforeCaching = flag.Int("blockcache.missesBeforeCaching", 2, "The number of cache misses before putting the block into cache. "+
	"Higher values may reduce indexdb/dataBlocks cache size at the cost of higher CPU and disk read usage")

// Cache caches Block entries.
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

// RemoveBlocksForPart removes all the blocks for the given part from the cache.
func (c *Cache) RemoveBlocksForPart(p any) {
	for _, shard := range c.shards {
		shard.RemoveBlocksForPart(p)
	}
}

// GetBlock returns a block for the given key k from c.
func (c *Cache) GetBlock(k Key) Block {
	idx := uint64(0)
	if len(c.shards) > 1 {
		h := k.hashUint64()
		idx = h % uint64(len(c.shards))
	}
	shard := c.shards[idx]
	return shard.GetBlock(k)
}

// TryPutBlock puts the given block b under the given key k into c.
//
// returns true if block was added to the cache
func (c *Cache) TryPutBlock(k Key, b Block) bool {
	idx := uint64(0)
	if len(c.shards) > 1 {
		h := k.hashUint64()
		idx = h % uint64(len(c.shards))
	}
	shard := c.shards[idx]
	return shard.TryPutBlock(k, b)
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
	d := timeutil.AddJitterToDuration(time.Minute)
	ticker := time.NewTicker(d)
	defer ticker.Stop()

	d = timeutil.AddJitterToDuration(time.Minute * 3)
	perKeyMissesTicker := time.NewTicker(d)
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
	for _, shard := range c.shards {
		shard.cleanByTimeout()
	}
}

func (c *Cache) cleanPerKeyMisses() {
	for _, shard := range c.shards {
		shard.cleanPerKeyMisses()
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

	// m contains cached blocks keyed by Key.Part and then by Key.Offset
	m map[any]map[uint64]*cacheEntry

	// perKeyMisses contains per-block cache misses.
	//
	// Blocks with up to *missesBeforeCaching cache misses aren't stored in the cache in order to prevent from eviction for frequently accessed items.
	perKeyMisses map[Key]int

	// The heap for removing the least recently used entries from m.
	lah lastAccessHeap
}

// Key represents a key, which uniquely identifies the Block.
type Key struct {
	// Part must contain a pointer to part structure where the block belongs to.
	Part any

	// Offset is the offset of the block in the part.
	Offset uint64
}

func (k *Key) hashUint64() uint64 {
	buf := (*[unsafe.Sizeof(*k)]byte)(unsafe.Pointer(k))
	return xxhash.Sum64(buf[:])
}

// Block is an item, which may be cached in the Cache.
type Block interface {
	// SizeBytes must return the approximate size of the given block in bytes
	SizeBytes() int
}

type cacheEntry struct {
	// The timestamp in seconds for the last access to the given entry.
	lastAccessTime uint64

	// heapIdx is the index for the entry in lastAccessHeap.
	heapIdx int

	// k contains the associated key for the given block.
	k Key

	// b contains the cached block.
	b Block
}

func newCache(getMaxSizeBytes func() int) *cache {
	var c cache
	c.getMaxSizeBytes = getMaxSizeBytes
	c.m = make(map[any]map[uint64]*cacheEntry)
	c.perKeyMisses = make(map[Key]int)
	return &c
}

func (c *cache) RemoveBlocksForPart(p any) {
	c.mu.Lock()
	defer c.mu.Unlock()

	sizeBytes := 0
	for _, e := range c.m[p] {
		sizeBytes += e.b.SizeBytes()
		heap.Remove(&c.lah, e.heapIdx)
		// do not delete the entry from c.perKeyMisses, since it is removed by cache.cleaner later.
	}
	c.updateSizeBytes(-sizeBytes)
	delete(c.m, p)
}

func (c *cache) updateSizeBytes(n int) {
	c.sizeBytes.Add(int64(n))
}

func (c *cache) cleanPerKeyMisses() {
	c.mu.Lock()
	c.perKeyMisses = make(map[Key]int, len(c.perKeyMisses))
	c.mu.Unlock()
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

func (c *cache) GetBlock(k Key) Block {
	c.requests.Add(1)
	var e *cacheEntry
	c.mu.Lock()
	defer c.mu.Unlock()

	pes := c.m[k.Part]
	if pes != nil {
		e = pes[k.Offset]
		if e != nil {
			// Fast path - the block already exists in the cache, so return it to the caller.
			currentTime := fasttime.UnixTimestamp()
			if e.lastAccessTime != currentTime {
				e.lastAccessTime = currentTime
				heap.Fix(&c.lah, e.heapIdx)
			}
			return e.b
		}
	}
	// Slow path - the entry is missing in the cache.
	c.perKeyMisses[k]++
	c.misses.Add(1)
	return nil
}

func (c *cache) TryPutBlock(k Key, b Block) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	misses := c.perKeyMisses[k]
	if misses > 0 && misses <= *missesBeforeCaching {
		// If the entry wasn't accessed yet (e.g. misses == 0), then cache it,
		// since it has been just created without consulting the cache and will be accessed soon.
		//
		// Do not cache the entry if there were up to *missesBeforeCaching unsuccessful attempts to access it.
		// This may be one-time-wonders entry, which won't be accessed more, so do not cache it
		// in order to save memory for frequently accessed items.
		return false
	}

	// reset key misses counter
	// it should help to reduce memory on fast cache eviction
	c.perKeyMisses[k] = 0
	// Store b in the cache.
	pes := c.m[k.Part]
	if pes == nil {
		pes = make(map[uint64]*cacheEntry)
		c.m[k.Part] = pes
	} else if pes[k.Offset] != nil {
		// The block has been already registered by concurrent goroutine.
		return true
	}
	e := &cacheEntry{
		lastAccessTime: fasttime.UnixTimestamp(),
		k:              k,
		b:              b,
	}
	heap.Push(&c.lah, e)
	pes[k.Offset] = e
	c.updateSizeBytes(e.b.SizeBytes())
	maxSizeBytes := c.getMaxSizeBytes()
	for c.SizeBytes() > maxSizeBytes && len(c.lah) > 0 {
		c.removeLeastRecentlyAccessedItem()
	}
	return true
}

func (c *cache) removeLeastRecentlyAccessedItem() {
	e := c.lah[0]
	c.updateSizeBytes(-e.b.SizeBytes())
	p := e.k.Part
	pes := c.m[p]
	delete(pes, e.k.Offset)
	if len(pes) == 0 {
		// Remove reference to p from c.m in order to free up memory occupied by p.
		delete(c.m, p)
	}
	heap.Pop(&c.lah)
}

func (c *cache) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()

	n := 0
	for _, m := range c.m {
		n += len(m)
	}
	return n
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
