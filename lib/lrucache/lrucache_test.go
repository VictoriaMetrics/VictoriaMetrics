package lrucache

import (
	"fmt"
	"sync"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/cgroup"
)

func TestCache(t *testing.T) {
	sizeMaxBytes := 64 * 1024
	// Multiply sizeMaxBytes by the square of available CPU cores
	// in order to get proper distribution of sizes between cache shards.
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/2204
	cpus := cgroup.AvailableCPUs()
	sizeMaxBytes *= cpus * cpus
	getMaxSize := func() int {
		return sizeMaxBytes
	}
	c := NewCache(getMaxSize)
	defer c.MustStop()
	if n := c.SizeBytes(); n != 0 {
		t.Fatalf("unexpected SizeBytes(); got %d; want %d", n, 0)
	}
	if n := c.SizeMaxBytes(); n != sizeMaxBytes {
		t.Fatalf("unexpected SizeMaxBytes(); got %d; want %d", n, sizeMaxBytes)
	}
	k := "foobar"
	var e testEntry
	entrySize := e.SizeBytes()
	// Put a single entry into cache
	c.PutEntry(k, &e)
	if n := c.Len(); n != 1 {
		t.Fatalf("unexpected number of items in the cache; got %d; want %d", n, 1)
	}
	if n := c.SizeBytes(); n != entrySize {
		t.Fatalf("unexpected SizeBytes(); got %d; want %d", n, entrySize)
	}
	if n := c.Requests(); n != 0 {
		t.Fatalf("unexpected number of requests; got %d; want %d", n, 0)
	}
	if n := c.Misses(); n != 0 {
		t.Fatalf("unexpected number of misses; got %d; want %d", n, 0)
	}
	// Obtain this entry from the cache
	if e1 := c.GetEntry(k); e1 != &e {
		t.Fatalf("unexpected entry obtained; got %v; want %v", e1, &e)
	}
	if n := c.Requests(); n != 1 {
		t.Fatalf("unexpected number of requests; got %d; want %d", n, 1)
	}
	if n := c.Misses(); n != 0 {
		t.Fatalf("unexpected number of misses; got %d; want %d", n, 0)
	}
	// Obtain non-existing entry from the cache
	if e1 := c.GetEntry("non-existing-key"); e1 != nil {
		t.Fatalf("unexpected non-nil block obtained for non-existing key: %v", e1)
	}
	if n := c.Requests(); n != 2 {
		t.Fatalf("unexpected number of requests; got %d; want %d", n, 2)
	}
	if n := c.Misses(); n != 1 {
		t.Fatalf("unexpected number of misses; got %d; want %d", n, 1)
	}
	// Store the entry again.
	c.PutEntry(k, &e)
	if n := c.SizeBytes(); n != entrySize {
		t.Fatalf("unexpected SizeBytes(); got %d; want %d", n, entrySize)
	}
	if e1 := c.GetEntry(k); e1 != &e {
		t.Fatalf("unexpected entry obtained; got %v; want %v", e1, &e)
	}
	if n := c.Requests(); n != 3 {
		t.Fatalf("unexpected number of requests; got %d; want %d", n, 3)
	}
	if n := c.Misses(); n != 1 {
		t.Fatalf("unexpected number of misses; got %d; want %d", n, 1)
	}

	// Manually clean the cache. The entry shouldn't be deleted because it was recently accessed.
	c.cleanByTimeout()
	if n := c.SizeBytes(); n != entrySize {
		t.Fatalf("unexpected SizeBytes(); got %d; want %d", n, entrySize)
	}
}

func TestCacheConcurrentAccess(_ *testing.T) {
	const sizeMaxBytes = 16 * 1024 * 1024
	getMaxSize := func() int {
		return sizeMaxBytes
	}
	c := NewCache(getMaxSize)
	defer c.MustStop()

	workers := 5
	var wg sync.WaitGroup
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func(worker int) {
			defer wg.Done()
			testCacheSetGet(c, worker)
		}(i)
	}
	wg.Wait()
}

func testCacheSetGet(c *Cache, worker int) {
	for i := 0; i < 1000; i++ {
		e := testEntry{}
		k := fmt.Sprintf("key_%d_%d", worker, i)
		c.PutEntry(k, &e)
		if e1 := c.GetEntry(k); e1 != &e {
			panic(fmt.Errorf("unexpected entry obtained; got %v; want %v", e1, &e))
		}
		if e1 := c.GetEntry("non-existing-key"); e1 != nil {
			panic(fmt.Errorf("unexpected non-nil entry obtained: %v", e1))
		}
	}
}

type testEntry struct{}

func (tb *testEntry) SizeBytes() int {
	return 42
}
