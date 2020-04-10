package workingsetcache

import (
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/VictoriaMetrics/fastcache"
)

// Cache modes.
const (
	split     = 0
	switching = 1
	whole     = 2
)

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

	// mode indicates whether to use only curr and skip prev.
	//
	// This flag is set to switching if curr is filled for more than 50% space.
	// In this case using prev would result in RAM waste,
	// it is better to use only curr cache with doubled size.
	// After the process of switching, this flag will be set to whole.
	mode uint64

	// mu serializes access to curr, prev and mode
	// in expirationWorker and cacheSizeWatcher.
	mu sync.Mutex

	wg     sync.WaitGroup
	stopCh chan struct{}

	// historicalStats keeps historical counters from fastcache.Stats
	historicalStats fastcache.Stats
}

// Load loads the cache from filePath and limits its size to maxBytes
// and evicts inactive entires after expireDuration.
//
// Stop must be called on the returned cache when it is no longer needed.
func Load(filePath string, maxBytes int, expireDuration time.Duration) *Cache {
	curr := fastcache.LoadFromFileOrNew(filePath, maxBytes)
	var cs fastcache.Stats
	curr.UpdateStats(&cs)
	if cs.EntriesCount == 0 {
		curr.Reset()
		// The cache couldn't be loaded with maxBytes size.
		// This may mean that the cache is split into curr and prev caches.
		// Try loading it again with maxBytes / 2 size.
		maxBytes /= 2
		curr = fastcache.LoadFromFileOrNew(filePath, maxBytes)
		return newWorkingSetCache(curr, maxBytes, expireDuration)
	}

	// The cache has been successfully loaded in full.
	// Set its' mode to `whole`.
	// There is no need in starting expirationWorker and cacheSizeWatcher.
	var c Cache
	c.curr.Store(curr)
	c.prev.Store(fastcache.New(1024))
	c.stopCh = make(chan struct{})
	atomic.StoreUint64(&c.mode, whole)
	return &c
}

// New creates new cache with the given maxBytes size and the given expireDuration
// for inactive entries.
//
// Stop must be called on the returned cache when it is no longer needed.
func New(maxBytes int, expireDuration time.Duration) *Cache {
	// Split maxBytes between curr and prev caches.
	maxBytes /= 2
	curr := fastcache.New(maxBytes)
	return newWorkingSetCache(curr, maxBytes, expireDuration)
}

func newWorkingSetCache(curr *fastcache.Cache, maxBytes int, expireDuration time.Duration) *Cache {
	prev := fastcache.New(1024)
	var c Cache
	c.curr.Store(curr)
	c.prev.Store(prev)
	c.stopCh = make(chan struct{})
	atomic.StoreUint64(&c.mode, split)

	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		c.expirationWorker(maxBytes, expireDuration)
	}()
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		c.cacheSizeWatcher(maxBytes)
	}()
	return &c
}

func (c *Cache) expirationWorker(maxBytes int, expireDuration time.Duration) {
	t := time.NewTicker(expireDuration / 2)
	for {
		select {
		case <-c.stopCh:
			t.Stop()
			return
		case <-t.C:
		}

		c.mu.Lock()
		if atomic.LoadUint64(&c.mode) == split {
			// Expire prev cache and create fresh curr cache.
			// Do not reuse prev cache, since it can have too big capacity.
			prev := c.prev.Load().(*fastcache.Cache)
			prev.Reset()
			curr := c.curr.Load().(*fastcache.Cache)
			curr.UpdateStats(&c.historicalStats)
			c.prev.Store(curr)
			curr = fastcache.New(maxBytes)
			c.curr.Store(curr)
		}
		c.mu.Unlock()
	}
}

func (c *Cache) cacheSizeWatcher(maxBytes int) {
	t := time.NewTicker(time.Minute)
	defer t.Stop()

	for {
		select {
		case <-c.stopCh:
			return
		case <-t.C:
		}
		var cs fastcache.Stats
		curr := c.curr.Load().(*fastcache.Cache)
		curr.UpdateStats(&cs)
		if cs.BytesSize >= uint64(maxBytes)/2 {
			break
		}
	}

	// curr cache size exceeds 50% of its capacity. It is better
	// to double the size of curr cache and stop using prev cache,
	// since this will result in higher summary cache capacity.
	//
	// Do this in the following steps:
	// 1) switch to mode=switching
	// 2) move curr cache to prev
	// 3) create curr with the double size
	// 4) wait until curr size exceeds maxBytes/2, i.e. it is populated with new data
	// 5) switch to mode=whole
	// 6) drop prev

	c.mu.Lock()
	atomic.StoreUint64(&c.mode, switching)
	prev := c.prev.Load().(*fastcache.Cache)
	prev.Reset()
	curr := c.curr.Load().(*fastcache.Cache)
	curr.UpdateStats(&c.historicalStats)
	c.prev.Store(curr)
	c.curr.Store(fastcache.New(maxBytes * 2))
	c.mu.Unlock()

	for {
		select {
		case <-c.stopCh:
			return
		case <-t.C:
		}
		var cs fastcache.Stats
		curr := c.curr.Load().(*fastcache.Cache)
		curr.UpdateStats(&cs)
		if cs.BytesSize >= uint64(maxBytes)/2 {
			break
		}
	}

	c.mu.Lock()
	atomic.StoreUint64(&c.mode, whole)
	prev = c.prev.Load().(*fastcache.Cache)
	prev.Reset()
	c.prev.Store(fastcache.New(1024))
	c.mu.Unlock()
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
}

// UpdateStats updates fcs with cache stats.
func (c *Cache) UpdateStats(fcs *fastcache.Stats) {
	curr := c.curr.Load().(*fastcache.Cache)
	curr.UpdateStats(fcs)

	// Add counters from historical stats
	hs := &c.historicalStats
	fcs.GetCalls += atomic.LoadUint64(&hs.GetCalls)
	fcs.SetCalls += atomic.LoadUint64(&hs.SetCalls)
	fcs.Misses += atomic.LoadUint64(&hs.Misses)
	fcs.Collisions += atomic.LoadUint64(&hs.Collisions)
	fcs.Corruptions += atomic.LoadUint64(&hs.Corruptions)

	if atomic.LoadUint64(&c.mode) == whole {
		return
	}

	// Add stats for entries from the previous cache
	// Do not add counters from the previous cache, since they are already
	// taken into account via c.historicalStats.
	prev := c.prev.Load().(*fastcache.Cache)
	var fcsTmp fastcache.Stats
	prev.UpdateStats(&fcsTmp)
	fcs.EntriesCount += fcsTmp.EntriesCount
	fcs.BytesSize += fcsTmp.BytesSize
}

// Get appends the found value for the given key to dst and returns the result.
func (c *Cache) Get(dst, key []byte) []byte {
	curr := c.curr.Load().(*fastcache.Cache)
	result := curr.Get(dst, key)
	if len(result) > len(dst) {
		// Fast path - the entry is found in the current cache.
		return result
	}
	if atomic.LoadUint64(&c.mode) == whole {
		return result
	}

	// Search for the entry in the previous cache.
	prev := c.prev.Load().(*fastcache.Cache)
	result = prev.Get(dst, key)
	if len(result) <= len(dst) {
		// Nothing found.
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
	if atomic.LoadUint64(&c.mode) == whole {
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
	if atomic.LoadUint64(&c.mode) == whole {
		return result
	}

	// Search for the entry in the previous cache.
	prev := c.prev.Load().(*fastcache.Cache)
	result = prev.GetBig(dst, key)
	if len(result) <= len(dst) {
		// Nothing found.
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
