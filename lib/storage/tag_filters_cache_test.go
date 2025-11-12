package storage

import (
	"reflect"
	"slices"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/uint64set"
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
