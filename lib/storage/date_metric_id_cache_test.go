package storage

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestDateMetricIDCacheSerial(t *testing.T) {
	c := newDateMetricIDCache()
	if err := testDateMetricIDCache(c, false); err != nil {
		t.Fatalf("unexpected error: %s", err)
	}
}

func TestDateMetricIDCacheConcurrent(t *testing.T) {
	c := newDateMetricIDCache()
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
			c.mu.Lock()
			c.resetLocked()
			c.mu.Unlock()
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

	// Verify c.Reset
	if n := c.EntriesCount(); !concurrent && n < 123 {
		return fmt.Errorf("c.EntriesCount must return at least 123; returned %d", n)
	}
	c.mu.Lock()
	c.resetLocked()
	c.mu.Unlock()
	if n := c.EntriesCount(); !concurrent && n > 0 {
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
	var wg sync.WaitGroup
	for i := range concurrency {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for id := uint64(i * numMetrics); id < uint64((i+1)*numMetrics); id++ {
				dmc.Set(date, id)
				if !dmc.Has(date, id) {
					panic(fmt.Errorf("dmc.Has(metricID=%d): unexpected cache miss after adding the entry to cache", id))
				}
			}
		}()
	}
	wg.Wait()
}
