package blockcache

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
	offset := uint64(1234)
	part := (interface{})("foobar")
	k := Key{
		Offset: offset,
		Part:   part,
	}
	var b testBlock
	blockSize := b.SizeBytes()
	// Put a single entry into cache
	c.PutBlock(k, &b)
	if n := c.Len(); n != 1 {
		t.Fatalf("unexpected number of items in the cache; got %d; want %d", n, 1)
	}
	if n := c.SizeBytes(); n != blockSize {
		t.Fatalf("unexpected SizeBytes(); got %d; want %d", n, blockSize)
	}
	if n := c.Requests(); n != 0 {
		t.Fatalf("unexpected number of requests; got %d; want %d", n, 0)
	}
	if n := c.Misses(); n != 0 {
		t.Fatalf("unexpected number of misses; got %d; want %d", n, 0)
	}
	// Obtain this entry from the cache
	if b1 := c.GetBlock(k); b1 != &b {
		t.Fatalf("unexpected block obtained; got %v; want %v", b1, &b)
	}
	if n := c.Requests(); n != 1 {
		t.Fatalf("unexpected number of requests; got %d; want %d", n, 1)
	}
	if n := c.Misses(); n != 0 {
		t.Fatalf("unexpected number of misses; got %d; want %d", n, 0)
	}
	// Obtain non-existing entry from the cache
	if b1 := c.GetBlock(Key{Offset: offset + 1}); b1 != nil {
		t.Fatalf("unexpected non-nil block obtained for non-existing key: %v", b1)
	}
	if n := c.Requests(); n != 2 {
		t.Fatalf("unexpected number of requests; got %d; want %d", n, 2)
	}
	if n := c.Misses(); n != 1 {
		t.Fatalf("unexpected number of misses; got %d; want %d", n, 1)
	}
	// Remove entries for the given part from the cache
	c.RemoveBlocksForPart(part)
	if n := c.SizeBytes(); n != 0 {
		t.Fatalf("unexpected SizeBytes(); got %d; want %d", n, 0)
	}
	// Verify that the entry has been removed from the cache
	if b1 := c.GetBlock(k); b1 != nil {
		t.Fatalf("unexpected non-nil block obtained after removing all the blocks for the part; got %v", b1)
	}
	if n := c.Requests(); n != 3 {
		t.Fatalf("unexpected number of requests; got %d; want %d", n, 3)
	}
	if n := c.Misses(); n != 2 {
		t.Fatalf("unexpected number of misses; got %d; want %d", n, 2)
	}
	for i := 0; i < *missesBeforeCaching; i++ {
		// Store the missed entry to the cache. It shouldn't be stored because of the previous cache miss
		c.PutBlock(k, &b)
		if n := c.SizeBytes(); n != 0 {
			t.Fatalf("unexpected SizeBytes(); got %d; want %d", n, 0)
		}
		// Verify that the entry wasn't stored to the cache.
		if b1 := c.GetBlock(k); b1 != nil {
			t.Fatalf("unexpected non-nil block obtained after removing all the blocks for the part; got %v", b1)
		}
		if n := c.Requests(); n != uint64(4+i) {
			t.Fatalf("unexpected number of requests; got %d; want %d", n, 4+i)
		}
		if n := c.Misses(); n != uint64(3+i) {
			t.Fatalf("unexpected number of misses; got %d; want %d", n, 3+i)
		}
	}
	// Store the entry again. Now it must be stored because of the second cache miss.
	c.PutBlock(k, &b)
	if n := c.SizeBytes(); n != blockSize {
		t.Fatalf("unexpected SizeBytes(); got %d; want %d", n, blockSize)
	}
	if b1 := c.GetBlock(k); b1 != &b {
		t.Fatalf("unexpected block obtained; got %v; want %v", b1, &b)
	}
	if n := c.Requests(); n != uint64(4+*missesBeforeCaching) {
		t.Fatalf("unexpected number of requests; got %d; want %d", n, 4+*missesBeforeCaching)
	}
	if n := c.Misses(); n != uint64(2+*missesBeforeCaching) {
		t.Fatalf("unexpected number of misses; got %d; want %d", n, 2+*missesBeforeCaching)
	}

	// Manually clean the cache. The entry shouldn't be deleted because it was recently accessed.
	c.cleanPerKeyMisses()
	c.cleanByTimeout()
	if n := c.SizeBytes(); n != blockSize {
		t.Fatalf("unexpected SizeBytes(); got %d; want %d", n, blockSize)
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
		part := (interface{})(i)
		b := testBlock{}
		k := Key{
			Offset: uint64(worker*1000 + i),
			Part:   part,
		}
		c.PutBlock(k, &b)
		if b1 := c.GetBlock(k); b1 != &b {
			panic(fmt.Errorf("unexpected block obtained; got %v; want %v", b1, &b))
		}
		if b1 := c.GetBlock(Key{}); b1 != nil {
			panic(fmt.Errorf("unexpected non-nil block obtained: %v", b1))
		}
	}
}

type testBlock struct{}

func (tb *testBlock) SizeBytes() int {
	return 42
}
