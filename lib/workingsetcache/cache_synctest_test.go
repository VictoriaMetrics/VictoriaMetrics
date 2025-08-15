//go:build goexperiment.synctest

package workingsetcache

import (
	"os"
	"testing"
	"testing/synctest"
	"time"

	"github.com/VictoriaMetrics/fastcache"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestStats_split(t *testing.T) {
	synctest.Run(func() {
		var (
			k1, v1 = []byte("k1"), []byte("v1")
			k2, v2 = []byte("k2"), []byte("v2")
			k3     = []byte("k3")
			dst    []byte
		)

		c := New(1024)
		defer c.Stop()
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

		c.Get(dst[:0], k3)
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
	})
}

func TestStats_whole(t *testing.T) {
	defer removeAll(t)
	synctest.Run(func() {
		var (
			k1, v1 = []byte("k1"), []byte("v1")
			k2, v2 = []byte("k2"), []byte("v2")
			k3     = []byte("k3")
			dst    []byte
		)

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

func assertMode(t *testing.T, c *Cache, want uint32) {
	if got := c.mode.Load(); got != want {
		t.Fatalf("unexpected cache mode: got %d, want %d", got, want)
	}
}

func assertStats(t *testing.T, c *Cache, want fastcache.Stats) {
	t.Helper()
	var got fastcache.Stats
	c.UpdateStats(&got)
	ignoreFields := cmpopts.IgnoreFields(fastcache.Stats{}, "BytesSize", "MaxBytesSize", "EvictedBytes", "BigStats")
	if diff := cmp.Diff(want, got, ignoreFields); diff != "" {
		t.Fatalf("unexpected stats (-want, +got):\n%s", diff)
	}
}

func removeAll(t *testing.T) {
	if !t.Failed() {
		_ = os.RemoveAll(t.Name())
	}
}
