package blockcache

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
)

// Cache caches Block entries.
//
// Call NewCache() for creating new Cache.
type Cache struct {
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

// NewCache creates new cache.
//
// Cache size in bytes is limited by the value returned by getMaxSizeBytes() callback.
func NewCache(getMaxSizeBytes func() int) *Cache {
	var c Cache
	c.getMaxSizeBytes = getMaxSizeBytes
	c.m = make(map[interface{}]map[uint64]*cacheEntry)
	c.perKeyMisses = make(map[Key]int)
	go c.cleaner()
	return &c
}

// RemoveBlocksForPart removes all the blocks for the given part from the cache.
func (c *Cache) RemoveBlocksForPart(p interface{}) {
	c.mu.Lock()
	sizeBytes := 0
	for _, e := range c.m[p] {
		sizeBytes += e.block.SizeBytes()
		// do not delete the entry from c.perKeyMisses, since it is removed by Cache.cleaner later.
	}
	c.updateSizeBytes(-sizeBytes)
	delete(c.m, p)
	c.mu.Unlock()
}

func (c *Cache) updateSizeBytes(n int) {
	atomic.AddInt64(&c.sizeBytes, int64(n))
}

// cleaner periodically cleans least recently used entries in c.
func (c *Cache) cleaner() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	perKeyMissesTicker := time.NewTicker(2 * time.Minute)
	defer perKeyMissesTicker.Stop()
	for {
		select {
		case <-ticker.C:
			c.cleanByTimeout()
		case <-perKeyMissesTicker.C:
			c.mu.Lock()
			c.perKeyMisses = make(map[Key]int, len(c.perKeyMisses))
			c.mu.Unlock()
		}
	}
}

func (c *Cache) cleanByTimeout() {
	currentTime := fasttime.UnixTimestamp()
	c.mu.Lock()
	for _, pes := range c.m {
		for offset, e := range pes {
			// Delete items accessed more than two minutes ago.
			// This time should be enough for repeated queries.
			if currentTime-atomic.LoadUint64(&e.lastAccessTime) > 2*60 {
				c.updateSizeBytes(-e.block.SizeBytes())
				delete(pes, offset)
				// do not delete the entry from c.perKeyMisses, since it is removed by Cache.cleaner later.
			}
		}
	}
	c.mu.Unlock()
}

// GetBlock returns a block for the given key k from c.
func (c *Cache) GetBlock(k Key) Block {
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

// PutBlock puts the given block b under the given key k into c.
func (c *Cache) PutBlock(k Key, b Block) {
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
	e := &cacheEntry{
		lastAccessTime: fasttime.UnixTimestamp(),
		block:          b,
	}
	pes := c.m[k.Part]
	if pes == nil {
		pes = make(map[uint64]*cacheEntry)
		c.m[k.Part] = pes
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
				// do not delete the entry from c.perKeyMisses, since it is removed by Cache.cleaner later.
				if c.SizeBytes() < maxSizeBytes {
					goto end
				}
			}
		}
	}
end:
	c.mu.Unlock()
}

// Len returns the number of blocks in the cache c.
func (c *Cache) Len() int {
	c.mu.RLock()
	n := len(c.m)
	c.mu.RUnlock()
	return n
}

// SizeBytes returns an approximate size in bytes of all the blocks stored in the cache c.
func (c *Cache) SizeBytes() int {
	return int(atomic.LoadInt64(&c.sizeBytes))
}

// SizeMaxBytes returns the max allowed size in bytes for c.
func (c *Cache) SizeMaxBytes() int {
	return c.getMaxSizeBytes()
}

// Requests returns the number of requests served by c.
func (c *Cache) Requests() uint64 {
	return atomic.LoadUint64(&c.requests)
}

// Misses returns the number of cache misses for c.
func (c *Cache) Misses() uint64 {
	return atomic.LoadUint64(&c.misses)
}
