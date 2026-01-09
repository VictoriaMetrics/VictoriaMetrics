package storage

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/uint64set"
)

func TestDateMetricIDCacheSerial(t *testing.T) {
	c := newDateMetricIDCache()
	defer c.MustStop()
	if err := testDateMetricIDCache(c, false); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
}

func TestDateMetricIDCacheConcurrent(t *testing.T) {
	c := newDateMetricIDCache()
	defer c.MustStop()
	ch := make(chan error, 5)
	for i := 0; i < 5; i++ {
		go func() {
			ch <- testDateMetricIDCache(c, true)
		}()
	}
	for i := 0; i < 5; i++ {
		select {
		case err := <-ch:
			if err != nil {
				t.Fatalf("unexpected error: %s", err)
			}
		case <-time.After(time.Second * 5):
			t.Fatalf("timeout")
		}
	}
}

func testDateMetricIDCache(c *dateMetricIDCache, concurrent bool) error {
	type dmk struct {
		date     uint64
		metricID uint64
	}
	m := make(map[dmk]bool)
	for i := 0; i < 1e5; i++ {
		date := uint64(i) % 2
		metricID := uint64(i) % 1237
		if !concurrent && c.Has(date, metricID) {
			if !m[dmk{date, metricID}] {
				return fmt.Errorf("c.Has(%d, %d) must return false, but returned true", date, metricID)
			}
			continue
		}
		c.Set(date, metricID)
		m[dmk{date, metricID}] = true
		if !concurrent && !c.Has(date, metricID) {
			return fmt.Errorf("c.Has(%d, %d) must return true, but returned false", date, metricID)
		}
		if i%11234 == 0 {
			c.mu.Lock()
			c.syncLocked()
			c.mu.Unlock()
		}
		if i%34323 == 0 {
			// Two rotations are needed to clear the cache.
			c.rotate()
			c.rotate()
			m = make(map[dmk]bool)
		}
	}

	// Verify fast path after sync.
	for i := 0; i < 1e5; i++ {
		date := uint64(i) % 2
		metricID := uint64(i) % 123
		c.Set(date, metricID)
	}
	c.mu.Lock()
	c.syncLocked()
	c.mu.Unlock()
	for i := 0; i < 1e5; i++ {
		date := uint64(i) % 2
		metricID := uint64(i) % 123
		if !concurrent && !c.Has(date, metricID) {
			return fmt.Errorf("c.Has(%d, %d) must return true after sync", date, metricID)
		}
	}

	// Verify that cache becomes empty after two rotations.
	if n := c.Stats().Size; !concurrent && n < 123 {
		return fmt.Errorf("c.EntriesCount must return at least 123; returned %d", n)
	}
	c.rotate()
	if n := c.Stats().Size; !concurrent && n < 123 {
		return fmt.Errorf("c.EntriesCount must return at least 123; returned %d", n)
	}
	c.rotate()
	if n := c.Stats().Size; !concurrent && n > 0 {
		return fmt.Errorf("c.EntriesCount must return 0 after reset; returned %d", n)
	}
	return nil
}

func TestDateMetricIDCacheIsConsistent(_ *testing.T) {
	const (
		generation  = 1
		date        = 1
		concurrency = 2
		numMetrics  = 100000
	)
	dmc := newDateMetricIDCache()
	defer dmc.MustStop()
	var wg sync.WaitGroup
	for i := range concurrency {
		wg.Go(func() {
			for id := uint64(i * numMetrics); id < uint64((i+1)*numMetrics); id++ {
				dmc.Set(date, id)
				if !dmc.Has(date, id) {
					panic(fmt.Errorf("dmc.Has(metricID=%d): unexpected cache miss after adding the entry to cache", id))
				}
			}
		})
	}
	wg.Wait()
}

func TestDateMetricIDCache_Size(t *testing.T) {
	dmc := newDateMetricIDCache()
	defer dmc.MustStop()
	for i := range 100_000 {
		date := 12345 + uint64(i%30)
		metricID := uint64(i)
		dmc.Set(date, metricID)

		if got, want := dmc.Stats().Size, uint64(i+1); got != want {
			t.Fatalf("unexpected size: got %d, want %d", got, want)
		}
	}

	// Retrieve all entries and check the cache size again.
	for i := range 100_000 {
		date := 12345 + uint64(i%30)
		metricID := uint64(i)
		if !dmc.Has(date, metricID) {
			t.Fatalf("entry not in cache: (date=%d, metricID=%d)", date, metricID)
		}
	}
	if got, want := dmc.Stats().Size, uint64(100_000); got != want {
		t.Fatalf("unexpected size: got %d, want %d", got, want)
	}
}

func TestDateMetricIDCache_SizeBytes(t *testing.T) {
	dmc := newDateMetricIDCache()
	defer dmc.MustStop()
	metricIDs := &uint64set.Set{}
	for i := range 100_000 {
		date := uint64(123)
		metricID := uint64(i)
		metricIDs.Add(metricID)
		dmc.Set(date, metricID)
	}
	if got, want := dmc.Stats().SizeBytes, metricIDs.SizeBytes(); got != want {
		t.Fatalf("unexpected sizeBytes: got %d, want %d", got, want)
	}

	// Retrieve all entries and check the cache sizeBytes again.
	for i := range 100_000 {
		date := uint64(123)
		metricID := uint64(i)
		if !dmc.Has(date, metricID) {
			t.Fatalf("entry not in cache: (date=%d, metricID=%d)", date, metricID)
		}
	}
	if got, want := dmc.Stats().SizeBytes, metricIDs.SizeBytes(); got != want {
		t.Fatalf("unexpected sizeBytes: got %d, want %d", got, want)
	}
}
