package storage

import (
	"testing"
	"testing/synctest"
	"time"
)

func TestMetricIDCache_ClearedWhenUnused(t *testing.T) {
	// Entries that are added to the cache but then never retrieved will be
	// eventually removed from it.
	synctest.Test(t, func(t *testing.T) {
		c := newMetricIDCache()
		defer c.Stop()
		c.Set(123)
		time.Sleep(time.Hour + 15*time.Minute)
		time.Sleep(time.Hour + 15*time.Minute)
		time.Sleep(time.Hour + 15*time.Minute)
		if c.Has(123) {
			t.Fatalf("entry is still in cache")
		}
	})

	// Entries that are added to the cache and retrieved but then never
	// retrieved again will be eventually removed from it.
	synctest.Test(t, func(t *testing.T) {
		c := newMetricIDCache()
		defer c.Stop()
		c.Set(123)
		time.Sleep(15 * time.Minute)
		if !c.Has(123) {
			t.Fatalf("entry not in cache")
		}
		time.Sleep(time.Hour + 15*time.Minute)
		time.Sleep(time.Hour + 15*time.Minute)
		if c.Has(123) {
			t.Fatalf("entry is still in cache")
		}
	})

	// Entries that are added to the cache and then periodically retrieved,
	// will remain in cache indefinitely.
	synctest.Test(t, func(t *testing.T) {
		c := newMetricIDCache()
		defer c.Stop()
		c.Set(123)
		for range 10_000 {
			time.Sleep(15 * time.Minute)
			if !c.Has(123) {
				t.Fatalf("entry not in cache")
			}
		}
	})
}
