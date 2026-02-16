//go:build synctest

package storage

import (
	"testing"
	"testing/synctest"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func (c *metricIDCache) numShards() uint64 {
	return uint64(len(c.shards))
}

func (c *metricIDCache) fullRotationPeriod() time.Duration {
	return metricIDCacheShardCount * c.rotationPeriod
}

func TestMetricIDCache_ClearedWhenUnused(t *testing.T) {
	// Entries that are added to the cache but then never retrieved will be
	// eventually removed from it.
	synctest.Test(t, func(t *testing.T) {
		c := newMetricIDCache()
		defer c.MustStop()
		c.Set(123)
		// It takes 3 full rotation cycles for an entry to be evicted from the
		// cache:
		// [in next] -rotation1-> [in curr] -rotation2-> [in prev] -rotation3-> evicted.
		time.Sleep(3 * c.fullRotationPeriod())
		if c.Has(123) {
			t.Fatalf("entry is still in cache")
		}
	})

	// Entries that are added to the cache and retrieved but then never
	// retrieved again will be eventually removed from it.
	synctest.Test(t, func(t *testing.T) {
		c := newMetricIDCache()
		defer c.MustStop()
		c.Set(123)
		time.Sleep(c.fullRotationPeriod())
		if !c.Has(123) {
			t.Fatalf("entry not in cache")
		}
		time.Sleep(3 * c.fullRotationPeriod())
		if c.Has(123) {
			t.Fatalf("entry is still in cache")
		}
	})

	// Entries that are added to the cache and then periodically retrieved,
	// will remain in cache indefinitely.
	synctest.Test(t, func(t *testing.T) {
		c := newMetricIDCache()
		defer c.MustStop()
		c.Set(123)
		for range 10_000 {
			time.Sleep(c.fullRotationPeriod())
			if !c.Has(123) {
				t.Fatalf("entry not in cache")
			}
		}
	})
}

func TestMetricIDCache_Stats(t *testing.T) {
	assertStats := func(t *testing.T, c *metricIDCache, want metricIDCacheStats) {
		t.Helper()
		if diff := cmp.Diff(want, c.Stats(), cmpopts.IgnoreFields(metricIDCacheStats{}, "SizeBytes")); diff != "" {
			t.Fatalf("unexpected stats (-want, +got):\n%s", diff)
		}
	}

	const numMetricIDs = 16 * 65536

	synctest.Test(t, func(t *testing.T) {
		c := newMetricIDCache()
		defer c.MustStop()

		// Check stats right after the creation.
		assertStats(t, c, metricIDCacheStats{})

		// Add metricIDs and check stats.
		// At this point, all metricIDs are in next.
		for metricID := range uint64(numMetricIDs) {
			c.Set(metricID)
		}
		assertStats(t, c, metricIDCacheStats{
			Size: numMetricIDs,
		})

		// Get all metricIDs and check stats.
		// All metricIDs will be sync'ed from next to curr.
		for metricID := range uint64(numMetricIDs) {
			if !c.Has(metricID) {
				t.Fatalf("metricID not in cache: %d", metricID)
			}
		}
		assertStats(t, c, metricIDCacheStats{
			Size:       numMetricIDs,
			SyncsCount: c.numShards(),
		})

		// Wait until all groups are rotated.
		// curr metricIDs will be moved to prev.
		time.Sleep(c.fullRotationPeriod() + time.Second)
		assertStats(t, c, metricIDCacheStats{
			Size:           numMetricIDs,
			SyncsCount:     c.numShards(),
			RotationsCount: c.numShards(),
		})

		// Wait until all groups are rotated.
		// The cache now should be empty.
		time.Sleep(c.fullRotationPeriod())
		assertStats(t, c, metricIDCacheStats{
			SyncsCount:     c.numShards(),
			RotationsCount: 2 * c.numShards(),
		})
	})
}
