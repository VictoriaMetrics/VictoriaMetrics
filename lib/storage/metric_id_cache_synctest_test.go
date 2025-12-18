package storage

import (
	"testing"
	"testing/synctest"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/uint64set"
	"github.com/google/go-cmp/cmp"
)

func TestMetricIDCache_ClearedWhenUnused(t *testing.T) {
	// Entries that are added to the cache but then never retrieved will be
	// eventually removed from it.
	synctest.Test(t, func(t *testing.T) {
		c := newMetricIDCache()
		defer c.MustStop()
		c.Set(123)
		time.Sleep(15 * time.Minute)
		time.Sleep(15 * time.Minute)
		time.Sleep(15 * time.Minute)
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
		time.Sleep(5 * time.Minute)
		if !c.Has(123) {
			t.Fatalf("entry not in cache")
		}
		time.Sleep(15 * time.Minute)
		time.Sleep(15 * time.Minute)
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
			time.Sleep(5 * time.Minute)
			if !c.Has(123) {
				t.Fatalf("entry not in cache")
			}
		}
	})
}

func TestMetricIDCache_Stats(t *testing.T) {
	assertStats := func(t *testing.T, c *metricIDCache, want metricIDCacheStats) {
		if diff := cmp.Diff(want, c.Stats()); diff != "" {
			t.Fatalf("unexpected stats (-want, +got):\n%s", diff)
		}
	}

	synctest.Test(t, func(t *testing.T) {
		c := newMetricIDCache()
		defer c.MustStop()

		// Check stats right after the creation.
		assertStats(t, c, metricIDCacheStats{})

		// Add metricIDs and check stats.
		// At this point, all metricIDs are in next.
		metricIDs := uint64set.Set{}
		for metricID := range uint64(100_000) {
			c.Set(metricID)
			metricIDs.Add(metricID)
		}
		assertStats(t, c, metricIDCacheStats{
			Size:      100_000,
			SizeBytes: metricIDs.SizeBytes(),
		})

		// Get all metricIDs and check stats.
		// All metricIDs will be sync'ed from next to curr.
		for metricID := range uint64(100_000) {
			if !c.Has(metricID) {
				t.Fatalf("metricID not in cache: %d", metricID)
			}
		}
		assertStats(t, c, metricIDCacheStats{
			Size:       100_000,
			SizeBytes:  metricIDs.SizeBytes(),
			SyncsCount: 1,
		})

		// Wait until next rotation.
		// curr metricIDs will be moved to prev.
		time.Sleep(15 * time.Minute)
		assertStats(t, c, metricIDCacheStats{
			Size:           100_000,
			SizeBytes:      metricIDs.SizeBytes(),
			SyncsCount:     1,
			RotationsCount: 1,
		})

		// Wait until another rotation.
		// The cache now should be empty.
		time.Sleep(15 * time.Minute)
		assertStats(t, c, metricIDCacheStats{
			SyncsCount:     1,
			RotationsCount: 2,
		})
	})
}
