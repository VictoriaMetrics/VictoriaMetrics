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
	part := (any)("foobar")
	k := Key{
		Offset: offset,
		Part:   part,
	}
	var b testBlock
	blockSize := b.SizeBytes()
	// Put a single entry into cache
	c.TryPutBlock(k, &b)
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
	for i := range *missesBeforeCaching {
		// Store the missed entry to the cache. It shouldn't be stored because of the previous cache miss
		c.TryPutBlock(k, &b)
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
	c.TryPutBlock(k, &b)
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
	for worker := range workers {
		wg.Go(func() {
			testCacheSetGet(c, worker)
		})
	}
	wg.Wait()
}

func testCacheSetGet(c *Cache, worker int) {
	for i := range 1000 {
		part := (any)(i)
		b := testBlock{}
		k := Key{
			Offset: uint64(worker*1000 + i),
			Part:   part,
		}
		c.TryPutBlock(k, &b)
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

func TestCacheCanStoreBlock(t *testing.T) {
	maxSizeBytes := 64 * 1024
	getMaxSize := func() int {
		return maxSizeBytes
	}

	f := func(shardsCount, sizeBytes int, want bool) {
		t.Helper()
		c := NewCacheWithShardsCount(getMaxSize, shardsCount)
		defer c.MustStop()
		if got := c.CanStoreBlock(sizeBytes); got != want {
			t.Fatalf("unexpected CanStoreBlock(%d) for the cache with %d shards and %d bytes budget; got %v; want %v",
				sizeBytes, shardsCount, maxSizeBytes, got, want)
		}
	}

	// A single shard must be able to hold the block.
	f(1, maxSizeBytes, true)
	f(1, maxSizeBytes+1, false)
	f(4, maxSizeBytes/4, true)
	f(4, maxSizeBytes/4+1, false)

	// The block fits into the total budget, but not into a single shard.
	f(2, maxSizeBytes/2, true)
	f(2, maxSizeBytes/2+1, false)
	f(4, maxSizeBytes, false)
	f(64, maxSizeBytes, false)

	// Blocks with a non-positive size are never reported as storable.
	f(4, 0, false)
	f(4, -1, false)

	// An empty budget cannot hold anything.
	maxSizeBytes = 0
	f(1, 1, false)
	f(4, 1, false)
	maxSizeBytes = 64 * 1024

	// The check must use the current budget instead of a cached value.
	c := NewCacheWithShardsCount(getMaxSize, 4)
	defer c.MustStop()
	if !c.CanStoreBlock(16 << 10) {
		t.Fatalf("the block must be storable with the %d bytes budget", maxSizeBytes)
	}
	maxSizeBytes = 4 << 10
	if c.CanStoreBlock(16 << 10) {
		t.Fatalf("the block must not be storable after the budget is reduced to %d bytes", maxSizeBytes)
	}
	if !c.CanStoreBlock(1 << 10) {
		t.Fatalf("the smaller block must be storable with the %d bytes budget", maxSizeBytes)
	}
	maxSizeBytes = 64 * 1024
	if !c.CanStoreBlock(16 << 10) {
		t.Fatalf("the block must be storable again after the budget is restored to %d bytes", maxSizeBytes)
	}

	// The number of shards must be at least 1.
	func() {
		defer func() {
			if r := recover(); r == nil {
				t.Fatalf("expected panic when creating the cache with zero shards")
			}
		}()
		NewCacheWithShardsCount(getMaxSize, 0)
	}()
}
