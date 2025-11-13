package storage

import (
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

func TestTagFiltersCache_Stats(t *testing.T) {
	maxBytesSize := uint64(getTagFiltersCacheSize())
	metricIDs := func(items ...uint64) *uint64set.Set {
		v := &uint64set.Set{}
		v.AddMulti(items)
		return v
	}
	c := newTagFiltersCache()
	assertStats := func(want tagFiltersCacheStats) {
		t.Helper()
		got := c.stats()
		if diff := cmp.Diff(want, got, cmp.AllowUnexported(tagFiltersCacheStats{})); diff != "" {
			t.Fatalf("unexpected stats (-want, +got):\n%s", diff)
		}
	}

	assertStats(tagFiltersCacheStats{
		maxBytesSize: maxBytesSize,
	})

	k1 := []byte("k1")
	v1 := metricIDs(1, 100, 1000, 10000)
	c.set(nil, v1, k1)
	assertStats(tagFiltersCacheStats{
		maxBytesSize: maxBytesSize,
		entriesCount: 1,
		bytesSize:    uint64(len(k1)) + v1.SizeBytes(),
	})

	if _, ok := c.get(nil, k1); !ok {
		t.Fatalf("missing entry for key %v", k1)
	}
	assertStats(tagFiltersCacheStats{
		maxBytesSize: maxBytesSize,
		entriesCount: 1,
		bytesSize:    uint64(len(k1)) + v1.SizeBytes(),
		getCalls:     1,
	})

	k2 := []byte("non-existent")
	if _, ok := c.get(nil, k2); ok {
		t.Fatalf("unexpected entry for key %v", k2)
	}
	assertStats(tagFiltersCacheStats{
		maxBytesSize: maxBytesSize,
		entriesCount: 1,
		bytesSize:    uint64(len(k1)) + v1.SizeBytes(),
		getCalls:     2,
		misses:       1,
	})

	// Duplicates are ignored.
	v2 := metricIDs(2, 200, 2000, 20000)
	c.set(nil, v2, k1)
	assertStats(tagFiltersCacheStats{
		maxBytesSize: maxBytesSize,
		entriesCount: 1,
		bytesSize:    uint64(len(k1)) + v1.SizeBytes(),
		getCalls:     2,
		misses:       1,
	})

	c.reset()
	assertStats(tagFiltersCacheStats{
		maxBytesSize: maxBytesSize,
	})
}
