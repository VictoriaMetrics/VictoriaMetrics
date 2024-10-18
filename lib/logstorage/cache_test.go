package logstorage

import (
	"fmt"
	"testing"
)

func TestCache(t *testing.T) {
	m := make(map[string]int)
	for i := 0; i < 10; i++ {
		k := fmt.Sprintf("key_%d", i)
		m[k] = i
	}

	c := newCache()
	defer c.MustStop()

	for kStr := range m {
		k := []byte(kStr)

		if v, ok := c.Get(k); ok {
			t.Fatalf("unexpected value obtained from the cache for key %q: %v", k, v)
		}
		c.Set(k, m[kStr])
		v, ok := c.Get(k)
		if !ok {
			t.Fatalf("cannot obtain value for key %q", k)
		}
		if n := v.(int); n != m[kStr] {
			t.Fatalf("unexpected value obtained for key %q; got %d; want %d", k, n, m[kStr])
		}
	}

	// The cached entries should be still visible after a single clean() call.
	c.clean()
	for kStr := range m {
		k := []byte(kStr)

		v, ok := c.Get(k)
		if !ok {
			t.Fatalf("cannot obtain value for key %q", k)
		}
		if n := v.(int); n != m[kStr] {
			t.Fatalf("unexpected value obtained for key %q; got %d; want %d", k, n, m[kStr])
		}
	}

	// The cached entries must be dropped after two clean() calls.
	c.clean()
	c.clean()

	for kStr := range m {
		k := []byte(kStr)

		if v, ok := c.Get(k); ok {
			t.Fatalf("unexpected value obtained from the cache for key %q: %v", k, v)
		}
	}
}
