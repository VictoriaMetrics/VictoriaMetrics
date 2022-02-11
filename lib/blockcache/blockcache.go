package blockcache

import (
	"sync"
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/cgroup"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	xxhash "github.com/cespare/xxhash/v2"
)

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
func (c *Cache) RemoveBlocksForPart(p interface{}) {
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

// PutBlock puts the given block b under the given key k into c.
func (c *Cache) PutBlock(k Key, b Block) {
	idx := uint64(0)
	if len(c.shards) > 1 {
		h := k.hashUint64()
		idx = h % uint64(len(c.shards))
	}
	shard := c.shards[idx]
	shard.PutBlock(k, b)
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
	ticker := time.NewTicker(57 * time.Second)
	defer ticker.Stop()
	perKeyMissesTicker := time.NewTicker(7 * time.Minute)
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
	// Atomically updated fields must go first in the struct, so they are properly
	// aligned to 8 bytes on 32-bit architectures.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/212
	requests uint64
	misses   uint64

	// sizeBytes contains an approximate size for all the blocks stored in the cache.
	sizeBytes int64

	// getMaxSizeBytes() is a callback, which returns the maximum allowed cache size in bytes.
	getMaxSizeBytes func() int

	// mu protects all the fields below.
	mu sync.RWMutex

	// m contains cached blocks keyed by Key.Part and then by Key.Offset
	m map[interface{}]map[uint64]*cacheEntry

	// perKeyMisses contains per-block cache misses.
	//
	// Blocks with less than 2 cache misses aren't stored in the cache in order to prevent from eviction for frequently accessed items.
	perKeyMisses map[Key]int
}

// Key represents a key, which uniquely identifies the Block.
type Key struct {
	// Part must contain a pointer to part structure where the block belongs to.
	Part interface{}

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
	// Atomically updated fields must go first in the struct, so they are properly
	// aligned to 8 bytes on 32-bit architectures.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/212
	lastAccessTime uint64

	// block contains the cached block.
	block Block
}

func newCache(getMaxSizeBytes func() int) *cache {
	var c cache
	c.getMaxSizeBytes = getMaxSizeBytes
	c.m = make(map[interface{}]map[uint64]*cacheEntry)
	c.perKeyMisses = make(map[Key]int)
	return &c
}

func (c *cache) RemoveBlocksForPart(p interface{}) {
	c.mu.Lock()
	sizeBytes := 0
	for _, e := range c.m[p] {
		sizeBytes += e.block.SizeBytes()
		// do not delete the entry from c.perKeyMisses, since it is removed by cache.cleaner later.
	}
	c.updateSizeBytes(-sizeBytes)
	delete(c.m, p)
	c.mu.Unlock()
}

func (c *cache) updateSizeBytes(n int) {
	atomic.AddInt64(&c.sizeBytes, int64(n))
}

func (c *cache) cleanPerKeyMisses() {
	c.mu.Lock()
	c.perKeyMisses = make(map[Key]int, len(c.perKeyMisses))
	c.mu.Unlock()
}

func (c *cache) cleanByTimeout() {
	// Delete items accessed more than five minutes ago.
	// This time should be enough for repeated queries.
	lastAccessTime := fasttime.UnixTimestamp() - 5*60
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, pes := range c.m {
		for offset, e := range pes {
			if lastAccessTime > atomic.LoadUint64(&e.lastAccessTime) {
				c.updateSizeBytes(-e.block.SizeBytes())
				delete(pes, offset)
				// do not delete the entry from c.perKeyMisses, since it is removed by cache.cleaner later.
			}
		}
	}
}

func (c *cache) GetBlock(k Key) Block {
	atomic.AddUint64(&c.requests, 1)
	var e *cacheEntry
	c.mu.RLock()
	pes := c.m[k.Part]
	if pes != nil {
		e = pes[k.Offset]
	}
	c.mu.RUnlock()
	if e != nil {
		// Fast path - the block already exists in the cache, so return it to the caller.
		currentTime := fasttime.UnixTimestamp()
		if atomic.LoadUint64(&e.lastAccessTime) != currentTime {
			atomic.StoreUint64(&e.lastAccessTime, currentTime)
		}
		return e.block
	}
	// Slow path - the entry is missing in the cache.
	c.mu.Lock()
	c.perKeyMisses[k]++
	c.mu.Unlock()
	atomic.AddUint64(&c.misses, 1)
	return nil
}

func (c *cache) PutBlock(k Key, b Block) {
	c.mu.RLock()
	// If the entry wasn't accessed yet (e.g. c.perKeyMisses[k] == 0), then cache it, since it is likely it will be accessed soon.
	// Do not cache the entry only if there was only a single unsuccessful attempt to access it.
	// This may be one-time-wonders entry, which won't be accessed more, so there is no need in caching it.
	doNotCache := c.perKeyMisses[k] == 1
	c.mu.RUnlock()
	if doNotCache {
		// Do not cache b if it has been requested only once (aka one-time-wonders items).
		// This should reduce memory usage for the cache.
		return
	}

	// Store b in the cache.
	c.mu.Lock()
	defer c.mu.Unlock()

	pes := c.m[k.Part]
	if pes == nil {
		pes = make(map[uint64]*cacheEntry)
		c.m[k.Part] = pes
	} else if pes[k.Offset] != nil {
		// The block has been already registered by concurrent goroutine.
		return
	}
	e := &cacheEntry{
		lastAccessTime: fasttime.UnixTimestamp(),
		block:          b,
	}
	pes[k.Offset] = e
	c.updateSizeBytes(e.block.SizeBytes())
	maxSizeBytes := c.getMaxSizeBytes()
	if c.SizeBytes() > maxSizeBytes {
		// Entries in the cache occupy too much space. Free up space by deleting some entries.
		for _, pes := range c.m {
			for offset, e := range pes {
				c.updateSizeBytes(-e.block.SizeBytes())
				delete(pes, offset)
				// do not delete the entry from c.perKeyMisses, since it is removed by cache.cleaner later.
				if c.SizeBytes() < maxSizeBytes {
					return
				}
			}
		}
	}
}

func (c *cache) Len() int {
	c.mu.RLock()
	n := 0
	for _, m := range c.m {
		n += len(m)
	}
	c.mu.RUnlock()
	return n
}

func (c *cache) SizeBytes() int {
	return int(atomic.LoadInt64(&c.sizeBytes))
}

func (c *cache) SizeMaxBytes() int {
	return c.getMaxSizeBytes()
}

func (c *cache) Requests() uint64 {
	return atomic.LoadUint64(&c.requests)
}

func (c *cache) Misses() uint64 {
	return atomic.LoadUint64(&c.misses)
}
