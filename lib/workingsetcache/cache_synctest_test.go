//go:build goexperiment.synctest

package workingsetcache

import (
	"testing"
	"testing/synctest"
	"time"

	"github.com/VictoriaMetrics/fastcache"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestStats(t *testing.T) {
	assertStats := func(c *Cache, want fastcache.Stats) {
		t.Helper()
		var got fastcache.Stats
		c.UpdateStats(&got)
		ignoreFields := cmpopts.IgnoreFields(fastcache.Stats{}, "BytesSize", "MaxBytesSize", "EvictedBytes", "BigStats")
		if diff := cmp.Diff(want, got, ignoreFields); diff != "" {
			t.Errorf("unexpected stats (-want, +got):\n%s", diff)
		}
	}

	synctest.Run(func() {
		var (
			k1, v1 = []byte("k1"), []byte("v1")
			k2, v2 = []byte("k2"), []byte("v2")
			k3     = []byte("k3")
			dst    []byte
		)

		c := New(10 ^ 6)
		defer c.Stop()
		assertStats(c, fastcache.Stats{})

		c.Set(k1, v1)
		assertStats(c, fastcache.Stats{
			EntriesCount: 1,
			SetCalls:     1,
		})
		c.Set(k2, v2)
		assertStats(c, fastcache.Stats{
			EntriesCount: 2,
			SetCalls:     2,
		})

		c.Get(dst[:0], k1)
		assertStats(c, fastcache.Stats{
			EntriesCount: 2,
			SetCalls:     2,
			GetCalls:     1,
		})

		c.Get(dst[:0], k3)
		assertStats(c, fastcache.Stats{
			EntriesCount: 2,
			SetCalls:     2,
			GetCalls:     3, // should be 2
			Misses:       2, // should be 1
		})

		// Wait until prev and curr cache are rotated.
		// k1 and k2 are now in prev, curr is empty.
		time.Sleep(*cacheExpireDuration + time.Minute)
		synctest.Wait()

		c.Get(dst[:0], k1)
		assertStats(c, fastcache.Stats{
			EntriesCount: 3, // should be 2
			SetCalls:     3, // should be 2
			GetCalls:     5, // should be 3
			Misses:       3, // should be 1
		})

		c.Get(dst[:0], k1)
		assertStats(c, fastcache.Stats{
			EntriesCount: 3, // should be 2
			SetCalls:     3, // should be 2
			GetCalls:     6, // should be 4
			Misses:       3, // should be 1
		})

		// Wait until prev and curr cache are rotated.
		// k1 is now in prev, k2 is gone, curr is empty
		time.Sleep(*cacheExpireDuration + time.Minute)
		synctest.Wait()

		c.Get(dst[:0], k2)
		assertStats(c, fastcache.Stats{
			EntriesCount: 1,
			SetCalls:     3, // should be 2
			GetCalls:     8, // should be 5
			Misses:       5, // should be 2
		})

		// Wait until prev and curr cache are rotated twice, i.e. the cache
		// should become empty.
		time.Sleep(2**cacheExpireDuration + time.Minute)
		synctest.Wait()

		c.Get(dst[:0], k1)
		assertStats(c, fastcache.Stats{
			EntriesCount: 0,
			SetCalls:     3,  // should be 1
			GetCalls:     10, // should be 6
			Misses:       7,  // should be 3
		})
	})
}
