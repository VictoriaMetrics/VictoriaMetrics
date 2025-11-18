package storage

import (
	"fmt"
	"reflect"
	"slices"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/uint64set"
	"github.com/google/go-cmp/cmp"
)

func TestTagFiltersCache(t *testing.T) {
	f := func(want []uint64) {
		t.Helper()

		key := []byte("key")
		c := newTagFiltersCache()
		wantSet := &uint64set.Set{}
		wantSet.AddMulti(want)
		c.set(nil, wantSet, key)
		gotSet, ok := c.get(nil, key)
		if !ok {
			t.Fatalf("expected metricIDs to be found in cache but they weren't: %v", want)
		}
		got := gotSet.AppendTo(nil)
		slices.Sort(want)
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("unexpected metricIDs in cache: got %v, want %v", got, want)
		}
	}

	f([]uint64{0})
	f([]uint64{1})
	f([]uint64{1234, 678932943, 843289893843})
	f([]uint64{1, 2, 3, 4, 5, 6, 8989898, 823849234, 1<<64 - 1, 1<<32 - 1, 0})
}

func TestTagFiltersCache_EmptyMetricIDs(t *testing.T) {
	f := func(metricIDs *uint64set.Set) {
		t.Helper()

		c := newTagFiltersCache()
		key := []byte("key")
		c.set(nil, metricIDs, key)
		got, ok := c.get(nil, key)
		if !ok {
			t.Fatalf("expected metricIDs to be found in cache but they weren't")
		}
		if got.Len() != 0 {
			t.Fatalf("unexpected metricIDs len: got %d, want 0", got.Len())
		}
	}

	f(&uint64set.Set{})
	f(nil)
}

func TestTagFiltersCache_GetSetStats(t *testing.T) {
	maxBytesSize := uint64(getTagFiltersCacheSize())
	metricIDs := func(items ...uint64) *uint64set.Set {
		v := &uint64set.Set{}
		v.AddMulti(items)
		return v
	}

	c := newTagFiltersCache()
	assertTagFiltersCacheStats(t, c, tagFiltersCacheStats{
		maxBytesSize: maxBytesSize,
	})

	k1 := []byte("k1")
	v1 := metricIDs(1, 100, 1000, 10000)
	c.set(nil, v1, k1)
	assertTagFiltersCacheStats(t, c, tagFiltersCacheStats{
		maxBytesSize: maxBytesSize,
		entriesCount: 1,
		bytesSize:    uint64(len(k1)) + v1.SizeBytes(),
	})

	if _, ok := c.get(nil, k1); !ok {
		t.Fatalf("missing entry for key %v", k1)
	}
	assertTagFiltersCacheStats(t, c, tagFiltersCacheStats{
		maxBytesSize: maxBytesSize,
		entriesCount: 1,
		bytesSize:    uint64(len(k1)) + v1.SizeBytes(),
		getCalls:     1,
	})

	k2 := []byte("non-existent")
	if _, ok := c.get(nil, k2); ok {
		t.Fatalf("unexpected entry for key %v", k2)
	}
	assertTagFiltersCacheStats(t, c, tagFiltersCacheStats{
		maxBytesSize: maxBytesSize,
		entriesCount: 1,
		bytesSize:    uint64(len(k1)) + v1.SizeBytes(),
		getCalls:     2,
		misses:       1,
	})

	// Duplicates are accepted, the old value is overwritten with the new one.
	// Duplicate value of the same size.
	v2 := metricIDs(2, 200, 2000, 20000)
	c.set(nil, v2, k1)
	assertTagFiltersCacheStats(t, c, tagFiltersCacheStats{
		maxBytesSize: maxBytesSize,
		entriesCount: 1,
		bytesSize:    uint64(len(k1)) + v2.SizeBytes(),
		getCalls:     2,
		misses:       1,
	})
	got, _ := c.get(nil, k1)
	assertTagFiltersCacheValue(t, got, v2)
	assertTagFiltersCacheStats(t, c, tagFiltersCacheStats{
		maxBytesSize: maxBytesSize,
		entriesCount: 1,
		bytesSize:    uint64(len(k1)) + v2.SizeBytes(),
		getCalls:     3,
		misses:       1,
	})

	// Duplicate value of bigger size.
	v3 := metricIDs(3, 3e2, 3e3, 3e4, 3e5)
	c.set(nil, v3, k1)
	assertTagFiltersCacheStats(t, c, tagFiltersCacheStats{
		maxBytesSize: maxBytesSize,
		entriesCount: 1,
		bytesSize:    uint64(len(k1)) + v3.SizeBytes(),
		getCalls:     3,
		misses:       1,
	})
	got, _ = c.get(nil, k1)
	assertTagFiltersCacheValue(t, got, v3)
	assertTagFiltersCacheStats(t, c, tagFiltersCacheStats{
		maxBytesSize: maxBytesSize,
		entriesCount: 1,
		bytesSize:    uint64(len(k1)) + v3.SizeBytes(),
		getCalls:     4,
		misses:       1,
	})

	// Duplicate value of smaller size.
	v4 := metricIDs(4, 4e2)
	c.set(nil, v4, k1)
	assertTagFiltersCacheStats(t, c, tagFiltersCacheStats{
		maxBytesSize: maxBytesSize,
		entriesCount: 1,
		bytesSize:    uint64(len(k1)) + v4.SizeBytes(),
		getCalls:     4,
		misses:       1,
	})
	got, _ = c.get(nil, k1)
	assertTagFiltersCacheValue(t, got, v4)
	assertTagFiltersCacheStats(t, c, tagFiltersCacheStats{
		maxBytesSize: maxBytesSize,
		entriesCount: 1,
		bytesSize:    uint64(len(k1)) + v4.SizeBytes(),
		getCalls:     5,
		misses:       1,
	})

	c.reset()
	assertTagFiltersCacheStats(t, c, tagFiltersCacheStats{
		maxBytesSize: maxBytesSize,
		resets:       1,
	})
}

func TestTagFiltersCache_Utilization(t *testing.T) {
	// Reset the cache size to default after the test is finished.
	defer SetTagFiltersCacheSize(0)

	key := []byte("key-xxx")
	value := &uint64set.Set{}
	value.AddMulti([]uint64{1, 100, 1e3, 1e4, 1e5, 1e6, 1e7, 1e8, 1e9})
	bytesSize := uint64(len(key)) + value.SizeBytes()
	const maxEntries = 100
	maxBytesSize := maxEntries * bytesSize

	SetTagFiltersCacheSize(int(maxBytesSize))
	c := newTagFiltersCache()
	assertTagFiltersCacheStats(t, c, tagFiltersCacheStats{
		maxBytesSize: maxBytesSize,
	})

	// Fill up the cache so there is no room for new entries.
	for i := uint64(1); i <= maxEntries; i++ {
		key := []byte(fmt.Sprintf("key-%03d", i))
		c.set(nil, value, key)

		want := tagFiltersCacheStats{
			maxBytesSize: maxBytesSize,
			entriesCount: i,
			bytesSize:    i * bytesSize,
		}
		assertTagFiltersCacheStats(t, c, want)
	}

	// Add one more entry. 10% of the cache must be freed.
	// One entry is 1%, so the resulting utilization after adding the entry must
	//be 91%.
	key = []byte(fmt.Sprintf("key-%03d", maxEntries+1))
	c.set(nil, value, key)

	want := tagFiltersCacheStats{
		maxBytesSize: maxBytesSize,
		entriesCount: 91,
		bytesSize:    91 * bytesSize,
	}
	assertTagFiltersCacheStats(t, c, want)
}

func assertTagFiltersCacheStats(t *testing.T, c *tagFiltersCache, want tagFiltersCacheStats) {
	t.Helper()
	got := c.stats()
	if diff := cmp.Diff(want, got, cmp.AllowUnexported(tagFiltersCacheStats{})); diff != "" {
		t.Fatalf("unexpected stats (-want, +got):\n%s", diff)
	}
}

func assertTagFiltersCacheValue(t *testing.T, gotSet, wantSet *uint64set.Set) {
	t.Helper()
	got := gotSet.AppendTo(nil)
	want := wantSet.AppendTo(nil)
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("unexpected value (-want, +got):\n%s", diff)
	}
}
