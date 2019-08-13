package workingsetcache

import (
	"flag"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/VictoriaMetrics/fastcache"
)

var oldBehavior = flag.Bool("cache.oldBehavior", false, "Whether to use old behaviour for caches. Old behavior can give better resuts "+
	"for low-RAM systems serving big number of time series. Systems with enough RAM would consume more RAM when `-cache.oldBehavior` is enabled")

// Cache is a cache for working set entries.
//
// The cache evicts inactive entries after the given expireDuration.
// Recently accessed entries survive expireDuration.
//
// Comparing to fastcache, this cache minimizes the required RAM size
// to values smaller than maxBytes.
type Cache struct {
	curr atomic.Value
	prev atomic.Value

	wg     sync.WaitGroup
	stopCh chan struct{}

	misses uint64
}

// Load loads the cache from filePath and limits its size to maxBytes
// and evicts inactive entires after expireDuration.
//
// Stop must be called on the returned cache when it is no longer needed.
func Load(filePath string, maxBytes int, expireDuration time.Duration) *Cache {
	if !*oldBehavior {
		// Split maxBytes between curr and prev caches.
		maxBytes /= 2
	}
	curr := fastcache.LoadFromFileOrNew(filePath, maxBytes)
	return newWorkingSetCache(curr, maxBytes, expireDuration)
}

// New creates new cache with the given maxBytes size and the given expireDuration
// for inactive entries.
//
// Stop must be called on the returned cache when it is no longer needed.
func New(maxBytes int, expireDuration time.Duration) *Cache {
	if !*oldBehavior {
		// Split maxBytes between curr and prev caches.
		maxBytes /= 2
	}
	curr := fastcache.New(maxBytes)
	return newWorkingSetCache(curr, maxBytes, expireDuration)
}

func newWorkingSetCache(curr *fastcache.Cache, maxBytes int, expireDuration time.Duration) *Cache {
	prev := fastcache.New(1024)
	var c Cache
	c.curr.Store(curr)
	c.prev.Store(prev)
	c.stopCh = make(chan struct{})
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		t := time.NewTicker(expireDuration / 2)
		for {
			select {
			case <-c.stopCh:
				return
			case <-t.C:
			}
			if *oldBehavior {
				// Keep the curr cache for old behavior.
				continue
			}

			// Do not reuse prev cache, since it can have too big capacity.
			prev := c.prev.Load().(*fastcache.Cache)
			prev.Reset()
			curr := c.curr.Load().(*fastcache.Cache)
			c.prev.Store(curr)
			curr = fastcache.New(maxBytes)
			c.curr.Store(curr)
		}
	}()
	return &c
}

// Save safes the cache to filePath.
func (c *Cache) Save(filePath string) error {
	curr := c.curr.Load().(*fastcache.Cache)
	concurrency := runtime.GOMAXPROCS(-1)
	return curr.SaveToFileConcurrent(filePath, concurrency)
}

// Stop stops the cache.
//
// The cache cannot be used after the Stop call.
func (c *Cache) Stop() {
	close(c.stopCh)
	c.wg.Wait()

	c.Reset()
}

// Reset resets the cache.
func (c *Cache) Reset() {
	prev := c.prev.Load().(*fastcache.Cache)
	prev.Reset()
	curr := c.curr.Load().(*fastcache.Cache)
	curr.Reset()

	c.misses = 0
}

// UpdateStats updates fcs with cache stats.
func (c *Cache) UpdateStats(fcs *fastcache.Stats) {
	curr := c.curr.Load().(*fastcache.Cache)
	fcsOrig := *fcs
	curr.UpdateStats(fcs)
	if *oldBehavior {
		return
	}
	fcs.Misses = fcsOrig.Misses + atomic.LoadUint64(&c.misses)

	fcsOrig.Reset()
	prev := c.prev.Load().(*fastcache.Cache)
	prev.UpdateStats(&fcsOrig)
	fcs.EntriesCount += fcsOrig.EntriesCount
	fcs.BytesSize += fcsOrig.BytesSize
}

// Get appends the found value for the given key to dst and returns the result.
func (c *Cache) Get(dst, key []byte) []byte {
	curr := c.curr.Load().(*fastcache.Cache)
	result := curr.Get(dst, key)
	if len(result) > len(dst) {
		// Fast path - the entry is found in the current cache.
		return result
	}
	if *oldBehavior {
		return result
	}

	// Search for the entry in the previous cache.
	prev := c.prev.Load().(*fastcache.Cache)
	result = prev.Get(dst, key)
	if len(result) <= len(dst) {
		// Nothing found.
		atomic.AddUint64(&c.misses, 1)
		return result
	}
	// Cache the found entry in the current cache.
	curr.Set(key, result[len(dst):])
	return result
}

// Has verifies whether the cahce contains the given key.
func (c *Cache) Has(key []byte) bool {
	curr := c.curr.Load().(*fastcache.Cache)
	if curr.Has(key) {
		return true
	}
	if *oldBehavior {
		return false
	}
	prev := c.prev.Load().(*fastcache.Cache)
	return prev.Has(key)
}

// Set sets the given value for the given key.
func (c *Cache) Set(key, value []byte) {
	curr := c.curr.Load().(*fastcache.Cache)
	curr.Set(key, value)
}

// GetBig appends the found value for the given key to dst and returns the result.
func (c *Cache) GetBig(dst, key []byte) []byte {
	curr := c.curr.Load().(*fastcache.Cache)
	result := curr.GetBig(dst, key)
	if len(result) > len(dst) {
		// Fast path - the entry is found in the current cache.
		return result
	}
	if *oldBehavior {
		return result
	}

	// Search for the entry in the previous cache.
	prev := c.prev.Load().(*fastcache.Cache)
	result = prev.GetBig(dst, key)
	if len(result) <= len(dst) {
		// Nothing found.
		atomic.AddUint64(&c.misses, 1)
		return result
	}
	// Cache the found entry in the current cache.
	curr.SetBig(key, result[len(dst):])
	return result
}

// SetBig sets the given value for the given key.
func (c *Cache) SetBig(key, value []byte) {
	curr := c.curr.Load().(*fastcache.Cache)
	curr.SetBig(key, value)
}
