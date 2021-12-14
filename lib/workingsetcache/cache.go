package workingsetcache

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/cgroup"
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
type Cache struct {
	curr atomic.Value
	prev atomic.Value

	// cs holds cache stats
	cs fastcache.Stats

	// mode indicates whether to use only curr and skip prev.
	//
	// This flag is set to switching if curr is filled for more than 50% space.
	// In this case using prev would result in RAM waste,
	// it is better to use only curr cache with doubled size.
	// After the process of switching, this flag will be set to whole.
	mode uint32

	// mu serializes access to curr, prev and mode
	// in expirationWatcher and cacheSizeWatcher.
	mu sync.Mutex

	wg     sync.WaitGroup
	stopCh chan struct{}
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
		curr := fastcache.New(maxBytes / 2)
		prev := fastcache.LoadFromFileOrNew(filePath, maxBytes/2)
		c := newCacheInternal(curr, prev, split)
		c.runWatchers(expireDuration)
		return c
	}

	// The cache has been successfully loaded in full.
	// Set its' mode to `whole`.
	// There is no need in runWatchers call.
	prev := fastcache.New(1024)
	return newCacheInternal(curr, prev, whole)
}

// New creates new cache with the given maxBytes capacity and the given expireDuration
// for inactive entries.
//
// Stop must be called on the returned cache when it is no longer needed.
func New(maxBytes int, expireDuration time.Duration) *Cache {
	curr := fastcache.New(maxBytes / 2)
	prev := fastcache.New(1024)
	c := newCacheInternal(curr, prev, split)
	c.runWatchers(expireDuration)
	return c
}

func newCacheInternal(curr, prev *fastcache.Cache, mode int) *Cache {
	var c Cache
	c.curr.Store(curr)
	c.prev.Store(prev)
	c.stopCh = make(chan struct{})
	c.setMode(mode)
	return &c
}

func (c *Cache) runWatchers(expireDuration time.Duration) {
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		c.expirationWatcher(expireDuration)
	}()
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		c.cacheSizeWatcher()
	}()
}

func (c *Cache) expirationWatcher(expireDuration time.Duration) {
	t := time.NewTicker(expireDuration / 2)
	for {
		select {
		case <-c.stopCh:
			t.Stop()
			return
		case <-t.C:
		}

		c.mu.Lock()
		if atomic.LoadUint32(&c.mode) != split {
			// Stop the expirationWatcher on non-split mode.
			c.mu.Unlock()
			return
		}
		// Expire prev cache and create fresh curr cache with the same capacity.
		// Do not reuse prev cache, since it can occupy too big amounts of memory.
		prev := c.prev.Load().(*fastcache.Cache)
		prev.Reset()
		curr := c.curr.Load().(*fastcache.Cache)
		var cs fastcache.Stats
		curr.UpdateStats(&cs)
		c.prev.Store(curr)
		curr = fastcache.New(int(cs.MaxBytesSize))
		c.curr.Store(curr)
		c.mu.Unlock()
	}
}

func (c *Cache) cacheSizeWatcher() {
	t := time.NewTicker(time.Minute)
	defer t.Stop()

	var maxBytesSize uint64
	for {
		select {
		case <-c.stopCh:
			return
		case <-t.C:
		}
		var cs fastcache.Stats
		curr := c.curr.Load().(*fastcache.Cache)
		curr.UpdateStats(&cs)
		if cs.BytesSize >= uint64(0.9*float64(cs.MaxBytesSize)) {
			maxBytesSize = cs.MaxBytesSize
			break
		}
	}

	// curr cache size exceeds 90% of its capacity. It is better
	// to double the size of curr cache and stop using prev cache,
	// since this will result in higher summary cache capacity.
	//
	// Do this in the following steps:
	// 1) switch to mode=switching
	// 2) move curr cache to prev
	// 3) create curr with the double size
	// 4) wait until curr size exceeds maxBytesSize, i.e. it is populated with new data
	// 5) switch to mode=whole
	// 6) drop prev

	c.mu.Lock()
	c.setMode(switching)
	prev := c.prev.Load().(*fastcache.Cache)
	prev.Reset()
	curr := c.curr.Load().(*fastcache.Cache)
	c.prev.Store(curr)
	c.curr.Store(fastcache.New(int(maxBytesSize * 2)))
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
		if cs.BytesSize >= maxBytesSize {
			break
		}
	}

	c.mu.Lock()
	c.setMode(whole)
	prev = c.prev.Load().(*fastcache.Cache)
	prev.Reset()
	c.prev.Store(fastcache.New(1024))
	c.mu.Unlock()
}

// Save saves the cache to filePath.
func (c *Cache) Save(filePath string) error {
	curr := c.curr.Load().(*fastcache.Cache)
	concurrency := cgroup.AvailableCPUs()
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
	// Reset the mode to `split` in the hope the working set size becomes smaller after the reset.
	c.setMode(split)
}

func (c *Cache) setMode(mode int) {
	atomic.StoreUint32(&c.mode, uint32(mode))
}

func (c *Cache) loadMode() int {
	return int(atomic.LoadUint32(&c.mode))
}

// UpdateStats updates fcs with cache stats.
func (c *Cache) UpdateStats(fcs *fastcache.Stats) {
	var cs fastcache.Stats
	curr := c.curr.Load().(*fastcache.Cache)
	curr.UpdateStats(&cs)
	fcs.Collisions += cs.Collisions
	fcs.Corruptions += cs.Corruptions
	fcs.EntriesCount += cs.EntriesCount
	fcs.BytesSize += cs.BytesSize
	fcs.MaxBytesSize += cs.MaxBytesSize

	fcs.GetCalls += atomic.LoadUint64(&c.cs.GetCalls)
	fcs.SetCalls += atomic.LoadUint64(&c.cs.SetCalls)
	fcs.Misses += atomic.LoadUint64(&c.cs.Misses)

	prev := c.prev.Load().(*fastcache.Cache)
	cs.Reset()
	prev.UpdateStats(&cs)
	fcs.EntriesCount += cs.EntriesCount
	fcs.BytesSize += cs.BytesSize
	fcs.MaxBytesSize += cs.MaxBytesSize
}

// Get appends the found value for the given key to dst and returns the result.
func (c *Cache) Get(dst, key []byte) []byte {
	atomic.AddUint64(&c.cs.GetCalls, 1)
	curr := c.curr.Load().(*fastcache.Cache)
	result := curr.Get(dst, key)
	if len(result) > len(dst) {
		// Fast path - the entry is found in the current cache.
		return result
	}
	if c.loadMode() == whole {
		// Nothing found.
		atomic.AddUint64(&c.cs.Misses, 1)
		return result
	}

	// Search for the entry in the previous cache.
	prev := c.prev.Load().(*fastcache.Cache)
	result = prev.Get(dst, key)
	if len(result) <= len(dst) {
		// Nothing found.
		atomic.AddUint64(&c.cs.Misses, 1)
		return result
	}
	// Cache the found entry in the current cache.
	curr.Set(key, result[len(dst):])
	return result
}

// Has verifies whether the cache contains the given key.
func (c *Cache) Has(key []byte) bool {
	atomic.AddUint64(&c.cs.GetCalls, 1)
	curr := c.curr.Load().(*fastcache.Cache)
	if curr.Has(key) {
		return true
	}
	if c.loadMode() == whole {
		atomic.AddUint64(&c.cs.Misses, 1)
		return false
	}
	prev := c.prev.Load().(*fastcache.Cache)
	if !prev.Has(key) {
		atomic.AddUint64(&c.cs.Misses, 1)
		return false
	}
	// Cache the found entry in the current cache.
	tmpBuf := tmpBufPool.Get()
	tmpBuf.B = prev.Get(tmpBuf.B, key)
	curr.Set(key, tmpBuf.B)
	tmpBufPool.Put(tmpBuf)
	return true
}

var tmpBufPool bytesutil.ByteBufferPool

// Set sets the given value for the given key.
func (c *Cache) Set(key, value []byte) {
	atomic.AddUint64(&c.cs.SetCalls, 1)
	curr := c.curr.Load().(*fastcache.Cache)
	curr.Set(key, value)
}

// GetBig appends the found value for the given key to dst and returns the result.
func (c *Cache) GetBig(dst, key []byte) []byte {
	atomic.AddUint64(&c.cs.GetCalls, 1)
	curr := c.curr.Load().(*fastcache.Cache)
	result := curr.GetBig(dst, key)
	if len(result) > len(dst) {
		// Fast path - the entry is found in the current cache.
		return result
	}
	if c.loadMode() == whole {
		// Nothing found.
		atomic.AddUint64(&c.cs.Misses, 1)
		return result
	}

	// Search for the entry in the previous cache.
	prev := c.prev.Load().(*fastcache.Cache)
	result = prev.GetBig(dst, key)
	if len(result) <= len(dst) {
		// Nothing found.
		atomic.AddUint64(&c.cs.Misses, 1)
		return result
	}
	// Cache the found entry in the current cache.
	curr.SetBig(key, result[len(dst):])
	return result
}

// SetBig sets the given value for the given key.
func (c *Cache) SetBig(key, value []byte) {
	atomic.AddUint64(&c.cs.SetCalls, 1)
	curr := c.curr.Load().(*fastcache.Cache)
	curr.SetBig(key, value)
}
