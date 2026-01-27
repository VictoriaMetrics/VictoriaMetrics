package workingsetcache

import (
	"flag"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/cgroup"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
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
	modeSplit     = 0
	modeSwitching = 1
	modeWhole     = 2
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

	// mode indicates whether to use only curr and skip prev.
	//
	// This flag is set to modeSwitching if curr is filled for more than 50% space.
	// In this case using prev would result in RAM waste,
	// it is better to use only curr cache with doubled size.
	// After the process of switching, this flag will be set to modeWhole.
	mode atomic.Uint32

	// The maxBytes value passed to New() or to Load().
	//
	// It is used for initialization of the curr cache with the proper size in modeSwitching.
	maxBytes int

	// mu serializes access to curr, prev and mode
	mu sync.Mutex

	// wg and stopCh are used for graceful shutdown of background watchers.
	wg     sync.WaitGroup
	stopCh chan struct{}
}

func newWithAutoCleanup(maxBytes int) *fastcache.Cache {
	c := fastcache.New(maxBytes)

	// Reset the cache after it is no longer reacheable since the cache
	// could remain in use at Set or Get methods after the rotation.
	runtime.SetFinalizer(c, func(c *fastcache.Cache) {
		c.Reset()
	})
	return c
}

// Load loads the cache from filePath and limits its size to maxBytes.
//
// Inactive entries are removed from the cache in *cacheExpireDuration.
//
// Stop must be called on the returned cache when it is no longer needed.
func Load(filePath string, maxBytes int) *Cache {
	return loadWithExpire(filePath, maxBytes, *cacheExpireDuration)
}

func loadWithExpire(filePath string, maxBytes int, expireDuration time.Duration) *Cache {
	if !fs.IsPathExist(filePath) {
		// There is no cache at the filePath. Create it
		logger.Infof("creating new cache at %s with max size %d bytes", filePath, maxBytes)
		return newWithExpire(maxBytes, expireDuration)
	}

	// Try loading the cache in modeWhole
	curr, err := fastcache.LoadFromFileMaxBytes(filePath, maxBytes)
	if err == nil {
		// Successfully loaded the cache in modeWhole
		logger.Infof("loaded cache at %s in modeWhole with maxSize %d bytes", filePath, maxBytes)
		prev := newWithAutoCleanup(1024)
		return newCacheInternal(curr, prev, modeWhole, maxBytes, expireDuration)
	}

	// Fall back loading the cache in modeSplit
	curr, err = fastcache.LoadFromFileMaxBytes(filePath, maxBytes/2)
	if err == nil {
		// Successfully loaded the cache in modeSplit
		// Put the loaded cache into `prev` instead of `curr`
		// in order to limit the growth of the cache for the current period of time.
		logger.Infof("loaded cache at %s in modeSplit with maxSize %d bytes", filePath, maxBytes)
		prev := curr
		curr = newWithAutoCleanup(maxBytes / 2)
		return newCacheInternal(curr, prev, modeSplit, maxBytes, expireDuration)
	}

	// Failed loading the cache in modeSplit. Verify and log the most likely errors
	if strings.Contains(err.Error(), "unexpected number of bucket chunks") {
		// covers the cache reset due to max memory size change at
		// https://github.com/VictoriaMetrics/fastcache/blob/9bc541587b1df2a9198cb2a0425b9ada4005a505/file.go#L147
		logger.Warnf("%s; the most likely reason: changed the cache size via command-line flags or changed the number of available CPU cores during the last restart", err)
	} else {
		logger.Errorf("invalid cache at %s: %s", filePath, err)
	}

	// Remove the invalid cache.
	fs.MustRemoveDir(filePath)

	logger.Infof("creating new cache at %s with max size %d bytes", filePath, maxBytes)
	return newWithExpire(maxBytes, expireDuration)
}

// New creates new cache with the given maxBytes capacity and *cacheExpireDuration expiration.
//
// Stop must be called on the returned cache when it is no longer needed.
func New(maxBytes int) *Cache {
	return newWithExpire(maxBytes, *cacheExpireDuration)
}

func newWithExpire(maxBytes int, expireDuration time.Duration) *Cache {
	curr := newWithAutoCleanup(maxBytes / 2)
	prev := newWithAutoCleanup(maxBytes / 2)
	c := newCacheInternal(curr, prev, modeSplit, maxBytes, expireDuration)
	return c
}

func newCacheInternal(curr, prev *fastcache.Cache, mode, maxBytes int, expireDuration time.Duration) *Cache {
	var c Cache
	c.maxBytes = maxBytes
	c.curr.Store(curr)
	c.prev.Store(prev)
	c.stopCh = make(chan struct{})
	c.mode.Store(uint32(mode))
	c.runWatchers(expireDuration)
	return &c
}

func (c *Cache) runWatchers(expireDuration time.Duration) {
	c.wg.Go(func() {
		c.expirationWatcher(expireDuration)
	})

	c.wg.Go(c.prevCacheWatcher)

	c.wg.Go(c.cacheSizeWatcher)
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

		if c.mode.Load() != modeSplit {
			// Do nothing in non-split mode.
			c.mu.Unlock()
			continue
		}

		// Reset prev cache and swap it with the curr cache.
		prev := c.prev.Load()
		curr := c.curr.Load()
		c.updateCacheStatsHistoryBeforeRotationLocked(prev, curr)

		c.prev.Store(curr)
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

		if c.mode.Load() != modeSplit {
			// Do nothing in non-split mode.
			c.mu.Unlock()
			continue
		}

		prev := c.prev.Load()
		curr := c.curr.Load()

		var csPrev, csCurr fastcache.Stats
		prev.UpdateStats(&csPrev)
		curr.UpdateStats(&csCurr)

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
				c.updateCacheStatsHistoryBeforeRotationLocked(prev, nil)
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

	for {
		select {
		case <-c.stopCh:
			return
		case <-t.C:
		}

		c.mu.Lock()

		if c.mode.Load() != modeSplit {
			// Do nothing in non-split mode.
			c.mu.Unlock()
			continue
		}

		var cs fastcache.Stats
		curr := c.curr.Load()
		curr.UpdateStats(&cs)
		if cs.BytesSize >= uint64(0.9*float64(cs.MaxBytesSize)) {
			c.transitIntoWholeModeLocked(cs.MaxBytesSize, t)
		}

		c.mu.Unlock()
	}
}

func (c *Cache) transitIntoWholeModeLocked(maxBytesSize uint64, t *time.Ticker) {
	// curr cache size exceeds 90% of its capacity. It is better
	// to double the size of curr cache and stop using prev cache,
	// since this will result in higher summary cache capacity.
	//
	// Do this in the following steps:
	// 1) switch to modeSwitching
	// 2) move curr cache to prev
	// 3) create curr cache with doubled size
	// 4) wait until curr cache size exceeds maxBytesSize, i.e. it is populated with new data
	// 5) switch to modeWhole
	// 6) drop prev cache

	c.mode.Store(modeSwitching)

	prev := c.prev.Load()
	curr := c.curr.Load()
	c.updateCacheStatsHistoryBeforeRotationLocked(prev, curr)

	c.prev.Store(curr)
	prev.Reset()

	// use c.maxBytes instead of maxBytesSize*2 for creating new cache, since otherwise the created cache
	// couldn't be loaded from file with c.maxBytes limit after saving with maxBytesSize*2 limit.
	c.curr.Store(newWithAutoCleanup(c.maxBytes))

	c.mu.Unlock()

	// Wait until curr cache size exceeds maxBytesSize.
	for {
		select {
		case <-c.stopCh:
			c.mu.Lock()
			return
		case <-t.C:
		}

		c.mu.Lock()

		if c.mode.Load() != modeSwitching {
			// mode was changed by the Reset call
			return
		}

		var cs fastcache.Stats
		curr := c.curr.Load()
		curr.UpdateStats(&cs)
		if cs.BytesSize >= maxBytesSize {
			// curr cache size became bigger than maxBytesSize.
			break
		}

		c.mu.Unlock()
	}

	if c.mode.Load() != modeSwitching {
		// mode was changed by the Reset call
		return
	}

	// Switch to modeWhole

	c.mode.Store(modeWhole)

	prev = c.prev.Load()
	curr = c.curr.Load()
	c.updateCacheStatsHistoryBeforeRotationLocked(prev, curr)

	c.prev.Store(newWithAutoCleanup(1024))
	prev.Reset()
}

// MustSave saves the cache to filePath.
func (c *Cache) MustSave(filePath string) {
	startTime := time.Now()

	var cs fastcache.Stats
	curr := c.curr.Load()
	curr.UpdateStats(&cs)

	concurrency := cgroup.AvailableCPUs()

	logger.Infof("saving cache to %s by using %d concurrent workers", filePath, concurrency)
	err := curr.SaveToFileConcurrent(filePath, concurrency)
	if err != nil {
		logger.Panicf("FATAL: cannot save cache to %s: %s", filePath, err)
	}

	logger.Infof("cache has been successfully saved to %s in %.3f seconds; entriesCount: %d, sizeBytes: %d", filePath, time.Since(startTime).Seconds(), cs.EntriesCount, cs.BytesSize)
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
	c.mu.Lock()
	defer c.mu.Unlock()

	// load caches first to properly release memory
	prev := c.prev.Load()
	curr := c.curr.Load()
	c.updateCacheStatsHistoryBeforeRotationLocked(prev, curr)
	c.updateCacheStatsHistoryBeforeRotationLocked(curr, nil)

	// Reset the mode to `split` in order to properly reset background workers.
	mode := c.mode.Load()
	if mode != modeSplit {
		// non-split mode changes size of the caches
		// so we have to restore it into original size for split mode
		c.prev.Store(newWithAutoCleanup(c.maxBytes / 2))
		c.curr.Store(newWithAutoCleanup(c.maxBytes / 2))

		c.mode.Store(modeSplit)
	}

	prev.Reset()
	curr.Reset()
}

// UpdateStats updates fcs with cache stats.
func (c *Cache) UpdateStats(fcs *fastcache.Stats) {
	c.mu.Lock()
	defer c.mu.Unlock()

	curr := c.curr.Load()
	prev := c.prev.Load()

	var csPrev, csCurr fastcache.Stats
	prev.UpdateStats(&csPrev)
	curr.UpdateStats(&csCurr)

	csHistory := &c.csHistory

	fcs.GetCalls += csHistory.GetCalls + csCurr.GetCalls
	fcs.SetCalls += csHistory.SetCalls + csCurr.SetCalls

	fcs.Collisions += csHistory.Collisions + csCurr.Collisions
	fcs.Corruptions += csHistory.Corruptions + csCurr.Corruptions

	misses := csHistory.Misses
	if c.mode.Load() != modeWhole {
		// Take into account only the misses from csPrev, since csCurr misses always incur get() calls at csPrev in non-whole mode.
		// This is needed for the proper tracking of cache misses at https://github.com/VictoriaMetrics/VictoriaMetrics/issues/9553
		misses += csPrev.Misses
	} else {
		// Take into account misses from csCurr in modeWhole, since csPrev isn't used in this mode.
		misses += csCurr.Misses
	}
	fcs.Misses += misses

	// Track the total number of entries across prev and curr, since they all occupy memory.
	fcs.EntriesCount += csPrev.EntriesCount + csCurr.EntriesCount
	fcs.BytesSize += csPrev.BytesSize + csCurr.BytesSize
	fcs.MaxBytesSize += csPrev.MaxBytesSize + csCurr.MaxBytesSize
}

// updateCacheStatsHistoryBeforeRotationLocked updates c.csHistory before the rotation of curr and prev.
//
// c.mu.Lock() must be taken while calling this function.
func (c *Cache) updateCacheStatsHistoryBeforeRotationLocked(prev, curr *fastcache.Cache) {
	var csPrev, csCurr fastcache.Stats
	prev.UpdateStats(&csPrev)
	if curr != nil {
		curr.UpdateStats(&csCurr)
	}

	csHistory := &c.csHistory

	if c.mode.Load() != modeWhole {
		atomic.AddUint64(&csHistory.GetCalls, csCurr.GetCalls)
		atomic.AddUint64(&csHistory.SetCalls, csCurr.SetCalls)

		atomic.AddUint64(&csHistory.Collisions, csCurr.Collisions)
		atomic.AddUint64(&csHistory.Corruptions, csCurr.Corruptions)
	}

	// Subtract csCurr misses from csPrev misses, since csCurr replaces csPrev after the rotation.
	// This guarantees that csCurr.Misses are taken into account only once after the rotation at Cache.UpdateStats().
	// This is needed for the proper tracking of cache misses at https://github.com/VictoriaMetrics/VictoriaMetrics/issues/9553
	atomic.AddUint64(&csHistory.Misses, csPrev.Misses-csCurr.Misses)

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
	if c.mode.Load() == modeWhole {
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
	if c.mode.Load() == modeWhole {
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
	if c.mode.Load() == modeWhole {
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
