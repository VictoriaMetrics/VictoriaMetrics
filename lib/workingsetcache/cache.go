package workingsetcache

import (
	"errors"
	"flag"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/cgroup"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/timeutil"
	"github.com/VictoriaMetrics/fastcache"
)

var (
	prevCacheRemovalPercent = flag.Float64("prevCacheRemovalPercent", 0.1, "Items in the previous caches are removed when the percent of requests it serves "+
		"becomes lower than this value. Higher values reduce memory usage at the cost of higher CPU usage. See also -cacheExpireDuration")
	cacheExpireDuration = flag.Duration("cacheExpireDuration", 30*time.Minute, "Items are removed from in-memory caches after they aren't accessed for this duration. "+
		"Lower values may reduce memory usage at the cost of higher CPU usage. See also -prevCacheRemovalPercent")
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
	curr atomic.Pointer[fastcache.Cache]
	prev atomic.Pointer[fastcache.Cache]

	// csHistory holds cache stats history
	csHistory fastcache.Stats

	ExpireEvictionBytes atomic.Uint64
	MissEvictionBytes   atomic.Uint64
	SizeEvictionBytes   atomic.Uint64

	// mode indicates whether to use only curr and skip prev.
	//
	// This flag is set to switching if curr is filled for more than 50% space.
	// In this case using prev would result in RAM waste,
	// it is better to use only curr cache with doubled size.
	// After the process of switching, this flag will be set to whole.
	mode atomic.Uint32

	// The maxBytes value passed to New() or to Load().
	maxBytes int

	// mu serializes access to curr, prev and mode
	// in expirationWatcher, prevCacheWatcher and cacheSizeWatcher.
	mu sync.Mutex

	wg     sync.WaitGroup
	stopCh chan struct{}
}

// Load loads the cache from filePath and limits its size to maxBytes
// and evicts inactive entries in *cacheExpireDuration minutes.
//
// Stop must be called on the returned cache when it is no longer needed.
func Load(filePath string, maxBytes int) *Cache {
	return loadWithExpire(filePath, maxBytes, *cacheExpireDuration)
}

// loadFromFileOrNew attempts to load a fastcache.Cache from the given file path
// If loading fails due to an error (e.g. corrupted or unreadable file), the error is logged
// and a new cache is created with the specified maxBytes size.
func loadFromFileOrNew(filePath string, maxBytes int) *fastcache.Cache {
	cache, err := fastcache.LoadFromFileMaxBytes(filePath, maxBytes)
	if err == nil {
		return cache
	}

	if errors.Is(err, os.ErrNotExist) {
		logger.Infof("cache at path %s missing files; init new cache", filePath)
	} else if strings.Contains(err.Error(), "contains maxBytes") {
		// covers the cache reset due to max memory size change at
		// https://github.com/VictoriaMetrics/fastcache/blob/198c85ee90a1f65127126b5904c191e70f083cbf/file.go#L133
		logger.Warnf("%s; init new cache", err)
	} else {
		logger.Errorf("cache at path %s is invalid: %s; init new cache", filePath, err)
	}
	return fastcache.New(maxBytes)
}

func loadWithExpire(filePath string, maxBytes int, expireDuration time.Duration) *Cache {
	curr := loadFromFileOrNew(filePath, maxBytes)
	var cs fastcache.Stats
	curr.UpdateStats(&cs)
	if cs.EntriesCount == 0 {
		curr.Reset()
		// The cache couldn't be loaded with maxBytes size.
		// This may mean that the cache is split into curr and prev caches.
		// Try loading it again with maxBytes / 2 size.
		// Put the loaded cache into `prev` instead of `curr`
		// in order to limit the growth of the cache for the current period of time.
		prev := loadFromFileOrNew(filePath, maxBytes/2)
		curr := fastcache.New(maxBytes / 2)
		c := newCacheInternal(curr, prev, split, maxBytes)
		c.runWatchers(expireDuration)
		return c
	}

	// The cache has been successfully loaded in full.
	// Set its' mode to `whole`.
	// There is no need in runWatchers call.
	prev := fastcache.New(1024)
	return newCacheInternal(curr, prev, whole, maxBytes)
}

// New creates new cache with the given maxBytes capacity and *cacheExpireDuration expiration.
//
// Stop must be called on the returned cache when it is no longer needed.
func New(maxBytes int) *Cache {
	return newWithExpire(maxBytes, *cacheExpireDuration)
}

func newWithExpire(maxBytes int, expireDuration time.Duration) *Cache {
	curr := fastcache.New(maxBytes / 2)
	prev := fastcache.New(1024)
	c := newCacheInternal(curr, prev, split, maxBytes)
	c.runWatchers(expireDuration)
	return c
}

func newCacheInternal(curr, prev *fastcache.Cache, mode, maxBytes int) *Cache {
	var c Cache
	c.maxBytes = maxBytes
	c.curr.Store(curr)
	c.prev.Store(prev)
	c.stopCh = make(chan struct{})
	c.mode.Store(uint32(mode))
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
		c.prevCacheWatcher()
	}()
	c.wg.Add(1)
	go func() {
		defer c.wg.Done()
		c.cacheSizeWatcher()
	}()
}

func (c *Cache) expirationWatcher(expireDuration time.Duration) {
	expireDuration = timeutil.AddJitterToDuration(expireDuration)
	t := time.NewTicker(expireDuration)
	defer t.Stop()
	for {
		select {
		case <-c.stopCh:
			return
		case <-t.C:
		}
		c.mu.Lock()
		if c.mode.Load() != split {
			// Stop the expirationWatcher on non-split mode.
			c.mu.Unlock()
			return
		}
		// Reset prev cache and swap it with the curr cache.
		prev := c.prev.Load()
		curr := c.curr.Load()
		c.prev.Store(curr)
		var cs fastcache.Stats
		prev.UpdateStats(&cs)
		updateCacheStatsHistory(&c.csHistory, &cs)
		c.ExpireEvictionBytes.Add(cs.BytesSize)
		prev.Reset()
		c.curr.Store(prev)
		c.mu.Unlock()
	}
}

func (c *Cache) prevCacheWatcher() {
	p := *prevCacheRemovalPercent / 100
	if p <= 0 {
		// There is no need in removing the previous cache.
		return
	}
	minCurrRequests := uint64(1 / p)

	// Watch for the usage of the prev cache and drop it whenever it receives
	// less than prevCacheRemovalPercent requests comparing to the curr cache during the last 60 seconds.
	checkInterval := timeutil.AddJitterToDuration(time.Second * 60)
	t := time.NewTicker(checkInterval)
	defer t.Stop()
	prevGetCalls := uint64(0)
	currGetCalls := uint64(0)
	for {
		select {
		case <-c.stopCh:
			return
		case <-t.C:
		}
		c.mu.Lock()
		if c.mode.Load() != split {
			// Do nothing in non-split mode.
			c.mu.Unlock()
			return
		}
		prev := c.prev.Load()
		curr := c.curr.Load()
		var csCurr, csPrev fastcache.Stats
		curr.UpdateStats(&csCurr)
		prev.UpdateStats(&csPrev)
		currRequests := csCurr.GetCalls
		if currRequests >= currGetCalls {
			currRequests -= currGetCalls
		}
		prevRequests := csPrev.GetCalls
		if prevRequests >= prevGetCalls {
			prevRequests -= prevGetCalls
		}
		currGetCalls = csCurr.GetCalls
		prevGetCalls = csPrev.GetCalls
		if currRequests >= minCurrRequests && float64(prevRequests)/float64(currRequests) < p {
			// The majority of requests are served from the curr cache,
			// so the prev cache can be deleted in order to free up memory.
			if csPrev.EntriesCount > 0 {
				updateCacheStatsHistory(&c.csHistory, &csPrev)
				c.MissEvictionBytes.Add(csPrev.BytesSize)
				prev.Reset()
			}
		}
		c.mu.Unlock()
	}
}

func (c *Cache) cacheSizeWatcher() {
	checkInterval := timeutil.AddJitterToDuration(time.Millisecond * 1500)
	t := time.NewTicker(checkInterval)
	defer t.Stop()

	var maxBytesSize uint64
	for {
		select {
		case <-c.stopCh:
			return
		case <-t.C:
		}
		if c.mode.Load() != split {
			continue
		}
		var cs fastcache.Stats
		curr := c.curr.Load()
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
	// 3) create curr cache with doubled size
	// 4) wait until curr cache size exceeds maxBytesSize, i.e. it is populated with new data
	// 5) switch to mode=whole
	// 6) drop prev cache

	c.mu.Lock()
	c.mode.Store(switching)
	prev := c.prev.Load()
	curr := c.curr.Load()
	c.prev.Store(curr)
	var cs fastcache.Stats
	prev.UpdateStats(&cs)
	updateCacheStatsHistory(&c.csHistory, &cs)
	c.SizeEvictionBytes.Add(cs.BytesSize)
	prev.Reset()
	// use c.maxBytes instead of maxBytesSize*2 for creating new cache, since otherwise the created cache
	// couldn't be loaded from file with c.maxBytes limit after saving with maxBytesSize*2 limit.
	c.curr.Store(fastcache.New(c.maxBytes))
	c.mu.Unlock()

	for {
		select {
		case <-c.stopCh:
			return
		case <-t.C:
		}
		var cs fastcache.Stats
		curr := c.curr.Load()
		curr.UpdateStats(&cs)
		if cs.BytesSize >= maxBytesSize {
			break
		}
	}

	c.mu.Lock()
	c.mode.Store(whole)
	prev = c.prev.Load()
	c.prev.Store(fastcache.New(1024))
	cs.Reset()
	prev.UpdateStats(&cs)
	updateCacheStatsHistory(&c.csHistory, &cs)
	prev.Reset()
	c.mu.Unlock()
}

// Save saves the cache to filePath.
func (c *Cache) Save(filePath string) error {
	curr := c.curr.Load()
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
	var cs fastcache.Stats
	prev := c.prev.Load()
	prev.UpdateStats(&cs)
	prev.Reset()
	curr := c.curr.Load()
	curr.UpdateStats(&cs)
	updateCacheStatsHistory(&c.csHistory, &cs)
	curr.Reset()
	// Reset the mode to `split` in the hope the working set size becomes smaller after the reset.
	c.mode.Store(split)
}

// UpdateStats updates fcs with cache stats.
func (c *Cache) UpdateStats(fcs *fastcache.Stats) {
	updateCacheStatsHistory(fcs, &c.csHistory)

	var cs fastcache.Stats
	curr := c.curr.Load()
	curr.UpdateStats(&cs)
	updateCacheStats(fcs, &cs)

	prev := c.prev.Load()
	cs.Reset()
	prev.UpdateStats(&cs)
	updateCacheStats(fcs, &cs)
}

func updateCacheStats(dst, src *fastcache.Stats) {
	dst.GetCalls += src.GetCalls
	dst.SetCalls += src.SetCalls
	dst.Misses += src.Misses
	dst.Collisions += src.Collisions
	dst.Corruptions += src.Corruptions
	dst.EntriesCount += src.EntriesCount
	dst.BytesSize += src.BytesSize
	dst.MaxBytesSize += src.MaxBytesSize
}

func updateCacheStatsHistory(dst, src *fastcache.Stats) {
	atomic.AddUint64(&dst.GetCalls, atomic.LoadUint64(&src.GetCalls))
	atomic.AddUint64(&dst.SetCalls, atomic.LoadUint64(&src.SetCalls))
	atomic.AddUint64(&dst.Misses, atomic.LoadUint64(&src.Misses))
	atomic.AddUint64(&dst.Collisions, atomic.LoadUint64(&src.Collisions))
	atomic.AddUint64(&dst.Corruptions, atomic.LoadUint64(&src.Corruptions))

	// Do not add EntriesCount, BytesSize and MaxBytesSize, since these metrics
	// are calculated from c.curr and c.prev caches.
}

// Get appends the found value for the given key to dst and returns the result.
func (c *Cache) Get(dst, key []byte) []byte {
	curr := c.curr.Load()
	result := curr.Get(dst, key)
	if len(result) > len(dst) {
		// Fast path - the entry is found in the current cache.
		return result
	}
	if c.mode.Load() == whole {
		// Nothing found.
		return result
	}

	// Search for the entry in the previous cache.
	prev := c.prev.Load()
	result = prev.Get(dst, key)
	if len(result) <= len(dst) {
		// Nothing found.
		return result
	}
	// Cache the found entry in the current cache.
	curr.Set(key, result[len(dst):])
	return result
}

// Has verifies whether the cache contains the given key.
func (c *Cache) Has(key []byte) bool {
	curr := c.curr.Load()
	if curr.Has(key) {
		return true
	}
	if c.mode.Load() == whole {
		return false
	}
	prev := c.prev.Load()
	if !prev.Has(key) {
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
	curr := c.curr.Load()
	curr.Set(key, value)
}

// GetBig appends the found value for the given key to dst and returns the result.
func (c *Cache) GetBig(dst, key []byte) []byte {
	curr := c.curr.Load()
	result := curr.GetBig(dst, key)
	if len(result) > len(dst) {
		// Fast path - the entry is found in the current cache.
		return result
	}
	if c.mode.Load() == whole {
		// Nothing found.
		return result
	}

	// Search for the entry in the previous cache.
	prev := c.prev.Load()
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
	curr := c.curr.Load()
	curr.SetBig(key, value)
}
