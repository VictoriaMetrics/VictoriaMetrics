//go:build goexperiment.synctest

package workingsetcache

import (
	"fmt"
	"testing"
	"testing/synctest"
	"time"

	"github.com/VictoriaMetrics/fastcache"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
)

func TestCacheModeTransition(t *testing.T) {
	var cs fastcache.Stats
	assertCachesMaxSize := func(t *testing.T, c *Cache, maxPrevSize, maxCurrSize uint64) {
		t.Helper()

		c.mu.Lock()
		defer c.mu.Unlock()

		prev := c.prev.Load()
		cs.Reset()
		prev.UpdateStats(&cs)
		if cs.MaxBytesSize != maxPrevSize {
			t.Fatalf("unexpected prev cache maxSizeBytes: got %d, want %d", cs.MaxBytesSize, maxPrevSize)
		}

		curr := c.curr.Load()
		cs.Reset()
		curr.UpdateStats(&cs)
		if cs.MaxBytesSize != maxCurrSize {
			t.Fatalf("unexpected curr cache maxSizeBytes: got %d, want %d", cs.MaxBytesSize, maxCurrSize)
		}
	}
	const (
		cacheSize = 256 * 1024 * 1024
		// fastcache.chunkSize * fastcache.bucketsCount
		minCacheSize = 64 * 1024 * 512
		// check interval is ~1500ms with 10% jitter
		cacheSizeCheckInterval = time.Millisecond * 2000
	)

	baseKey := make([]byte, 8096)
	value := make([]byte, 12096)

	fillCacheToFull := func(t *testing.T, c *Cache) {
		t.Helper()
		for i := range 12096 {
			key := append(baseKey[:len(baseKey):len(baseKey)], fmt.Sprintf("idx_%d", i)...)
			c.Set(key, value)
		}
	}
	synctest.Test(t, func(t *testing.T) {
		fs.MustRemoveDir(t.Name())
		c := Load(t.Name(), cacheSize)
		defer c.Stop()

		assertMode(t, c, split)
		assertStats(t, c, fastcache.Stats{})

		synctest.Wait()
		assertCachesMaxSize(t, c, cacheSize/2, cacheSize/2)

		// fill curr cache up to 100% in order to transit it into switching state
		fillCacheToFull(t, c)
		time.Sleep(cacheSizeCheckInterval)
		synctest.Wait()

		assertMode(t, c, switching)

		// curr cache must use 100% of cache size during switching mode
		assertCachesMaxSize(t, c, cacheSize/2, cacheSize)

		// reset cache concurrently, switching into whole mode must not happen
		c.Reset()
		assertCachesMaxSize(t, c, cacheSize/2, cacheSize/2)
		assertMode(t, c, split)

		time.Sleep(cacheSizeCheckInterval)
		synctest.Wait()
		assertMode(t, c, split)

		// fill curr cache up to 100% in order to transit it into switching state
		// instead it should return back to switching mode
		fillCacheToFull(t, c)

		time.Sleep(cacheSizeCheckInterval)
		synctest.Wait()
		assertMode(t, c, switching)

		// curr cache must use 100% of cache size during switching mode
		assertCachesMaxSize(t, c, cacheSize/2, cacheSize)

		// fill cache up to 100% in order to transit into whole mode
		fillCacheToFull(t, c)

		time.Sleep(cacheSizeCheckInterval)
		synctest.Wait()
		assertMode(t, c, whole)

		// curr cache must use 100% of cache size whole mode
		// prev cache must use minimal amount of memory
		assertCachesMaxSize(t, c, minCacheSize, cacheSize)

		// reset cache, it must return into split mode
		c.Reset()
		assertCachesMaxSize(t, c, cacheSize/2, cacheSize/2)
		assertMode(t, c, split)

		// check if expiration worker operates correctly
		// it must rotate prev and curr

		// add item to curr and check it at prev after rotation
		key, value := []byte(`key1`), []byte(`value1`)
		c.Set(key, value)

		time.Sleep(35 * time.Minute)
		synctest.Wait()
		assertMode(t, c, split)
		assertCachesMaxSize(t, c, cacheSize/2, cacheSize/2)

		prev := c.prev.Load()
		result := prev.Get(nil, key)
		if string(result) != string(value) {
			t.Fatalf("key=%q must exist at prev cache", string(key))
		}
		curr := c.curr.Load()
		result = curr.Get(result[:0], key)
		if len(result) != 0 {
			t.Fatalf("key=%q must not exist at curr cache after rotation", string(key))
		}

		// check if size watcher operates correctly after Reset
		// fill it and check transition modes
		fillCacheToFull(t, c)
		time.Sleep(cacheSizeCheckInterval)
		assertMode(t, c, switching)
		fillCacheToFull(t, c)
		time.Sleep(cacheSizeCheckInterval)
		assertMode(t, c, whole)

		assertCachesMaxSize(t, c, minCacheSize, cacheSize)
	})
}

func TestSetGetStatsInSplitMode_newInmemoryCache(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c := New(1024)
		defer c.Stop()
		testSetGetStatsInSplitMode(t, c)
	})
}

func TestSetGetStatsInSplitMode_cacheLoadedFromEmptyFile(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c := Load(t.Name(), 1024)
		defer c.Stop()
		testSetGetStatsInSplitMode(t, c)
	})
}

func testSetGetStatsInSplitMode(t *testing.T, c *Cache) {
	var (
		k1, v1  = []byte("k1"), []byte("v1")
		k2, v2  = []byte("k2"), []byte("v2")
		kAbsent = []byte("absent")
		dst     []byte
	)

	assertMode(t, c, split)
	assertStats(t, c, fastcache.Stats{})

	c.Set(k1, v1)
	assertStats(t, c, fastcache.Stats{
		EntriesCount: 1,
		SetCalls:     1,
	})
	c.Set(k2, v2)
	assertStats(t, c, fastcache.Stats{
		EntriesCount: 2,
		SetCalls:     2,
	})

	c.Get(dst[:0], k1)
	assertStats(t, c, fastcache.Stats{
		EntriesCount: 2,
		SetCalls:     2,
		GetCalls:     1,
	})

	c.Get(dst[:0], kAbsent)
	assertStats(t, c, fastcache.Stats{
		EntriesCount: 2,
		SetCalls:     2,
		GetCalls:     2,
		Misses:       1,
	})

	// Wait until prev and curr cache are rotated.
	// k1 and k2 are now in prev, curr is empty.
	time.Sleep(*cacheExpireDuration + time.Minute)
	synctest.Wait()

	c.Get(dst[:0], k1)
	assertStats(t, c, fastcache.Stats{
		EntriesCount: 2,
		SetCalls:     2,
		GetCalls:     3,
		Misses:       1,
	})

	c.Get(dst[:0], k1)
	assertStats(t, c, fastcache.Stats{
		EntriesCount: 2,
		SetCalls:     2,
		GetCalls:     4,
		Misses:       1,
	})

	// Wait until prev and curr caches are rotated. k1 is now in prev, k2 is
	// gone, curr is empty
	time.Sleep(*cacheExpireDuration + time.Minute)
	synctest.Wait()

	c.Get(dst[:0], k2)
	assertStats(t, c, fastcache.Stats{
		EntriesCount: 1,
		SetCalls:     2,
		GetCalls:     5,
		Misses:       2,
	})

	// Wait until prev and curr caches are rotated. The both caches should
	// become empty.
	time.Sleep(*cacheExpireDuration + time.Minute)
	synctest.Wait()

	c.Get(dst[:0], k1)
	assertStats(t, c, fastcache.Stats{
		EntriesCount: 0,
		SetCalls:     2,
		GetCalls:     6,
		Misses:       3,
	})
}

func TestSetBigGetBigStatsInSplitMode_newInmemoryCache(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c := New(1024 * 1024)
		defer c.Stop()
		testSetBigGetBigStatsInSplitMode(t, c)
	})
}

func TestSetBigGetBigStatsInSplitMode_cacheLoadedFromEmptyFile(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c := Load(t.Name(), 1024*1024)
		defer c.Stop()
		testSetBigGetBigStatsInSplitMode(t, c)
	})
}

func testSetBigGetBigStatsInSplitMode(t *testing.T, c *Cache) {
	const maxSubvalueLen = 64*1024 - 16 - 4 - 1

	v := func(seed, size int) []byte {
		var buf []byte
		for i := 0; i < size; i++ {
			buf = append(buf, byte(i+seed))
		}
		return buf
	}

	var (
		k1, v1  = []byte("k1"), v(1, maxSubvalueLen-1)
		k2, v2  = []byte("k2"), v(2, maxSubvalueLen)
		k3, v3  = []byte("k3"), v(3, maxSubvalueLen+1)
		k4, v4  = []byte("k4"), v(4, 2*maxSubvalueLen)
		k5, v5  = []byte("k5"), v(5, 2*maxSubvalueLen+1)
		kAbsent = []byte("absent")
		dst     []byte
	)

	assertMode(t, c, split)
	assertStats(t, c, fastcache.Stats{})

	c.SetBig(k1, v1)
	assertStats(t, c, fastcache.Stats{
		EntriesCount: 2, // SetBig creates at least 2 entries per call.
		SetCalls:     1,
	})
	c.SetBig(k2, v2)
	assertStats(t, c, fastcache.Stats{
		EntriesCount: 4,
		SetCalls:     2,
	})
	c.SetBig(k3, v3)
	assertStats(t, c, fastcache.Stats{
		EntriesCount: 7,
		SetCalls:     3,
	})
	c.SetBig(k4, v4)
	assertStats(t, c, fastcache.Stats{
		EntriesCount: 10,
		SetCalls:     4,
	})
	c.SetBig(k5, v5)
	assertStats(t, c, fastcache.Stats{
		EntriesCount: 14,
		SetCalls:     5,
	})
	c.Get(dst[:0], k1)
	assertStats(t, c, fastcache.Stats{
		EntriesCount: 14,
		SetCalls:     5,
		GetCalls:     1,
	})
	c.Get(dst[:0], k3)
	assertStats(t, c, fastcache.Stats{
		EntriesCount: 14,
		SetCalls:     5,
		GetCalls:     2,
	})
	c.Get(dst[:0], kAbsent)
	assertStats(t, c, fastcache.Stats{
		EntriesCount: 14,
		SetCalls:     5,
		GetCalls:     3,
		Misses:       1,
	})

	// Wait until prev and curr cache are rotated.
	// k1-5 are now in prev, curr is empty.
	time.Sleep(*cacheExpireDuration + time.Minute)
	synctest.Wait()
	assertStats(t, c, fastcache.Stats{
		EntriesCount: 14,
		SetCalls:     5,
		GetCalls:     3,
		Misses:       1,
	})

	c.GetBig(dst[:0], k1)
	assertStats(t, c, fastcache.Stats{
		EntriesCount: 14,
		SetCalls:     5,
		GetCalls:     4,
		Misses:       1,
	})

	c.GetBig(dst[:0], k3)
	assertStats(t, c, fastcache.Stats{
		EntriesCount: 14,
		SetCalls:     5,
		GetCalls:     5,
		Misses:       1,
	})

	c.GetBig(dst[:0], k5)
	assertStats(t, c, fastcache.Stats{
		EntriesCount: 14,
		SetCalls:     5,
		GetCalls:     6,
		Misses:       1,
	})

	// Wait until prev and curr caches are rotated.
	// k1,3,5 are now in prev, k2,4 are gone, curr is empty.
	time.Sleep(*cacheExpireDuration + time.Minute)
	synctest.Wait()
	assertStats(t, c, fastcache.Stats{
		EntriesCount: 9, // 14-2-3
		SetCalls:     5,
		GetCalls:     6,
		Misses:       1,
	})

	c.GetBig(dst[:0], k2)
	assertStats(t, c, fastcache.Stats{
		EntriesCount: 9,
		SetCalls:     5,
		GetCalls:     7,
		Misses:       2,
	})

	c.GetBig(dst[:0], k4)
	assertStats(t, c, fastcache.Stats{
		EntriesCount: 9,
		SetCalls:     5,
		GetCalls:     8,
		Misses:       3,
	})

	// Wait until prev and curr caches are rotated.
	// The both caches should become empty.
	time.Sleep(*cacheExpireDuration + time.Minute)
	synctest.Wait()
	assertStats(t, c, fastcache.Stats{
		EntriesCount: 0,
		SetCalls:     5,
		GetCalls:     8,
		Misses:       3,
	})

	c.GetBig(dst[:0], k1)
	assertStats(t, c, fastcache.Stats{
		EntriesCount: 0,
		SetCalls:     5,
		GetCalls:     9,
		Misses:       4,
	})
}

func TestSetGetStatsInWholeMode_cacheLoadedFromNonEmptyFile(t *testing.T) {
	defer removeAll(t)
	synctest.Test(t, func(t *testing.T) {
		var (
			k1, v1 = []byte("k1"), []byte("v1")
			k2, v2 = []byte("k2"), []byte("v2")
			k3     = []byte("k3")
			dst    []byte
		)

		// Cache loaded from file operates in whole mode only if the file was
		// not empty.
		c := Load(t.Name(), 1024)
		c.Set(k1, v1)
		if err := c.Save(t.Name()); err != nil {
			t.Fatalf("could not save cache to file: %v", err)
		}
		c.Stop()
		c = Load(t.Name(), 1024)
		defer c.Stop()
		assertMode(t, c, whole)
		assertStats(t, c, fastcache.Stats{
			EntriesCount: 1,
		})

		c.Set(k2, v2)
		assertStats(t, c, fastcache.Stats{
			EntriesCount: 2,
			SetCalls:     1,
		})

		c.Get(dst[:0], k1)
		assertStats(t, c, fastcache.Stats{
			EntriesCount: 2,
			SetCalls:     1,
			GetCalls:     1,
		})
		c.Get(dst[:0], k2)
		assertStats(t, c, fastcache.Stats{
			EntriesCount: 2,
			SetCalls:     1,
			GetCalls:     2,
		})

		c.Get(dst[:0], k3)
		assertStats(t, c, fastcache.Stats{
			EntriesCount: 2,
			SetCalls:     1,
			GetCalls:     3,
			Misses:       1,
		})

		// In whole mode cache does not expire. Wait for expiration duration
		// anyway to confirm that data is still there.
		time.Sleep(*cacheExpireDuration + time.Minute)
		synctest.Wait()

		c.Get(dst[:0], k1)
		assertStats(t, c, fastcache.Stats{
			EntriesCount: 2,
			SetCalls:     1,
			GetCalls:     4,
			Misses:       1,
		})

		c.Get(dst[:0], k1)
		assertStats(t, c, fastcache.Stats{
			EntriesCount: 2,
			SetCalls:     1,
			GetCalls:     5,
			Misses:       1,
		})

		// In whole mode cache does not expire. Wait for expiration duration
		// anyway to confirm that data is still there.
		time.Sleep(*cacheExpireDuration + time.Minute)
		synctest.Wait()

		c.Get(dst[:0], k2)
		assertStats(t, c, fastcache.Stats{
			EntriesCount: 2,
			SetCalls:     1,
			GetCalls:     6,
			Misses:       1,
		})

		// In whole mode cache does not expire. Wait for expiration duration
		// anyway to confirm that data is still there.
		time.Sleep(*cacheExpireDuration + time.Minute)
		synctest.Wait()

		c.Get(dst[:0], k1)
		assertStats(t, c, fastcache.Stats{
			EntriesCount: 2,
			SetCalls:     1,
			GetCalls:     7,
			Misses:       1,
		})

	})
}

func TestSetBigGetBigStatsInWholeMode_cacheLoadedFromNonEmptyFile(t *testing.T) {
	defer removeAll(t)
	synctest.Test(t, func(t *testing.T) {
		v := func(seed, size int) []byte {
			var buf []byte
			for i := 0; i < size; i++ {
				buf = append(buf, byte(i+seed))
			}
			return buf
		}

		var (
			k1, v1  = []byte("k1"), v(1, maxSubvalueLen-1)
			k2, v2  = []byte("k2"), v(2, maxSubvalueLen)
			k3, v3  = []byte("k3"), v(3, maxSubvalueLen+1)
			k4, v4  = []byte("k4"), v(4, 2*maxSubvalueLen)
			k5, v5  = []byte("k5"), v(5, 2*maxSubvalueLen+1)
			kAbsent = []byte("absent")
			dst     []byte
		)

		// Cache loaded from file operates in whole mode only if the file was
		// not empty.
		c := Load(t.Name(), 1024*1024)
		assertMode(t, c, split)
		c.SetBig(k1, v1)
		assertStats(t, c, fastcache.Stats{
			EntriesCount: 2,
			SetCalls:     1,
		})
		if err := c.Save(t.Name()); err != nil {
			t.Fatalf("could not save cache to file: %v", err)
		}
		c.Stop()
		c = Load(t.Name(), 1024*1024)
		defer c.Stop()
		assertMode(t, c, whole)
		assertStats(t, c, fastcache.Stats{
			EntriesCount: 2,
		})

		c.SetBig(k2, v2)
		assertStats(t, c, fastcache.Stats{
			EntriesCount: 4,
			SetCalls:     1,
		})
		c.SetBig(k3, v3)
		assertStats(t, c, fastcache.Stats{
			EntriesCount: 7,
			SetCalls:     2,
		})
		c.SetBig(k4, v4)
		assertStats(t, c, fastcache.Stats{
			EntriesCount: 10,
			SetCalls:     3,
		})
		c.SetBig(k5, v5)
		assertStats(t, c, fastcache.Stats{
			EntriesCount: 14,
			SetCalls:     4,
		})
		c.GetBig(dst[:0], k1)
		assertStats(t, c, fastcache.Stats{
			EntriesCount: 14,
			SetCalls:     4,
			GetCalls:     1,
		})
		c.GetBig(dst[:0], k3)
		assertStats(t, c, fastcache.Stats{
			EntriesCount: 14,
			SetCalls:     4,
			GetCalls:     2,
		})
		c.GetBig(dst[:0], kAbsent)
		assertStats(t, c, fastcache.Stats{
			EntriesCount: 14,
			SetCalls:     4,
			GetCalls:     3,
			Misses:       1,
		})

		// In whole mode cache does not expire. Wait for expiration duration
		// anyway to confirm that data is still there.
		time.Sleep(*cacheExpireDuration + time.Minute)
		synctest.Wait()
		assertStats(t, c, fastcache.Stats{
			EntriesCount: 14,
			SetCalls:     4,
			GetCalls:     3,
			Misses:       1,
		})
		c.GetBig(dst[:0], k1)
		assertStats(t, c, fastcache.Stats{
			EntriesCount: 14,
			SetCalls:     4,
			GetCalls:     4,
			Misses:       1,
		})

		c.GetBig(dst[:0], k3)
		assertStats(t, c, fastcache.Stats{
			EntriesCount: 14,
			SetCalls:     4,
			GetCalls:     5,
			Misses:       1,
		})

		c.GetBig(dst[:0], k5)
		assertStats(t, c, fastcache.Stats{
			EntriesCount: 14,
			SetCalls:     4,
			GetCalls:     6,
			Misses:       1,
		})

		// In whole mode cache does not expire. Wait for expiration duration
		// anyway to confirm that data is still there.
		time.Sleep(*cacheExpireDuration + time.Minute)
		synctest.Wait()
		assertStats(t, c, fastcache.Stats{
			EntriesCount: 14,
			SetCalls:     4,
			GetCalls:     6,
			Misses:       1,
		})

		c.GetBig(dst[:0], k2)
		assertStats(t, c, fastcache.Stats{
			EntriesCount: 14,
			SetCalls:     4,
			GetCalls:     7,
			Misses:       1,
		})

		c.GetBig(dst[:0], k4)
		assertStats(t, c, fastcache.Stats{
			EntriesCount: 14,
			SetCalls:     4,
			GetCalls:     8,
			Misses:       1,
		})

		// In whole mode cache does not expire. Wait for expiration duration
		// anyway to confirm that data is still there.
		time.Sleep(*cacheExpireDuration + time.Minute)
		synctest.Wait()

		c.GetBig(dst[:0], k1)
		assertStats(t, c, fastcache.Stats{
			EntriesCount: 14,
			SetCalls:     4,
			GetCalls:     9,
			Misses:       1,
		})

	})
}
