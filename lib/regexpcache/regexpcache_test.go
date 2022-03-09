package regexpcache

import (
	"fmt"
	"testing"
)

func TestCache(t *testing.T) {
	maxSize := 100
	getMaxSize := func() int {
		return maxSize
	}
	c := NewCache(getMaxSize)
	defer c.MustStop()
	if n := c.Len(); n != 0 {
		t.Fatalf("unexpected SizeBytes(); got %d; want %d", n, 0)
	}
	if n := c.getMaxSize(); n != maxSize {
		t.Fatalf("unexpected SizeMaxBytes(); got %d; want %d", n, maxSize)
	}

	key := "test-entry"
	type testEntry struct {
		record string
	}
	v := testEntry{
		record: "test-value",
	}
	// Put a single entry into cache
	c.Put(key, &v)
	if n := c.Len(); n != 1 {
		t.Fatalf("unexpected number of items in the cache; got %d; want %d", n, 1)
	}
	if n := c.Requests(); n != 0 {
		t.Fatalf("unexpected number of requests; got %d; want %d", n, 0)
	}
	if n := c.Misses(); n != 0 {
		t.Fatalf("unexpected number of misses; got %d; want %d", n, 0)
	}
	// Obtain this entry from the cache
	if b1 := c.Get(key); b1 != &v {
		t.Fatalf("unexpected entry obtained; got %v; want %v", b1, &v)
	}
	if n := c.Requests(); n != 1 {
		t.Fatalf("unexpected number of requests; got %d; want %d", n, 1)
	}
	if n := c.Misses(); n != 0 {
		t.Fatalf("unexpected number of misses; got %d; want %d", n, 0)
	}
	// Obtain non-existing entry from the cache
	if b1 := c.Get("non-exist"); b1 != nil {
		t.Fatalf("unexpected non-nil value obtained for non-existing key: %v", b1)
	}
	if n := c.Requests(); n != 2 {
		t.Fatalf("unexpected number of requests; got %d; want %d", n, 2)
	}
	if n := c.Misses(); n != 1 {
		t.Fatalf("unexpected number of misses; got %d; want %d", n, 1)
	}
	// Check cache misses
	key = "entry-with-miss"
	// Verify that the entry wasn't stored to the cache and increment cache miss
	if b1 := c.Get(key); b1 != nil {
		t.Fatalf("unexpected non-nil value obtained; got %v", b1)
	}
	// Store the missed entry to the cache. It shouldn't be stored because of the previous cache miss
	c.Put(key, &v)
	// Verify that the entry wasn't stored to the cache.
	if b1 := c.Get(key); b1 != nil {
		t.Fatalf("unexpected non-nil value obtained after single cache miss; got %v", b1)
	}
	if n := c.Requests(); n != 4 {
		t.Fatalf("unexpected number of requests; got %d; want %d", n, 4)
	}
	if n := c.Misses(); n != 3 {
		t.Fatalf("unexpected number of misses; got %d; want %d", n, 3)
	}
	// Store the entry again. Now it must be stored because of the second cache miss.
	c.Put(key, &v)
	if b1 := c.Get(key); b1 != &v {
		t.Fatalf("unexpected value obtained; got %v; want %v", b1, &v)
	}
	if n := c.Requests(); n != 5 {
		t.Fatalf("unexpected number of requests; got %d; want %d", n, 5)
	}
	if n := c.Misses(); n != 3 {
		t.Fatalf("unexpected number of misses; got %d; want %d", n, 3)
	}

	// Manually clean the cache. The entry shouldn't be deleted because it was recently accessed.
	c.cleanPerKeyMisses()
	c.cleanByTimeout()
	if n := c.Len(); n != 2 {
		t.Fatalf("unexpected SizeBytes(); got %d; want %d", n, 2)
	}
	// overflow cache with maxSize
	for i := 0; i <= maxSize-2; i++ {
		key := fmt.Sprintf("key-number-%d", i)
		value := &testEntry{
			record: "test record",
		}
		c.Put(key, value)
	}
	// 10% of cache capacity must be left + 1 element
	if n := c.Len(); n != 91 {
		t.Fatalf("unexpected Len(); got %d; want %d", n, 93)
	}
	if len(c.lah) != len(c.m) {
		t.Fatalf("unexepected number of entries at lastAccessHeap, got: %d; and cache map, got: %d; must be equal", len(c.lah), len(c.m))
	}
}
