//go:build goexperiment.synctest

package workingsetcache

import (
	"testing"
	"testing/synctest"
	"time"

	"github.com/VictoriaMetrics/fastcache"
)

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
