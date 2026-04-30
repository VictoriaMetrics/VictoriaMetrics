package netstorage

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
)

// ---------------------------------------------------------------------------
// deduplicateStrings
// ---------------------------------------------------------------------------

func TestDeduplicateStrings(t *testing.T) {
	f := func(input []string, expected []string) {
		t.Helper()
		got := deduplicateStrings(input)
		sort.Strings(got)
		sort.Strings(expected)
		if len(got) != len(expected) {
			t.Fatalf("len mismatch: got %d, want %d\ngot=%v\nwant=%v", len(got), len(expected), got, expected)
		}
		for i := range got {
			if got[i] != expected[i] {
				t.Fatalf("element[%d] mismatch: got %q, want %q", i, got[i], expected[i])
			}
		}
	}

	// Empty input must return empty slice.
	f(nil, []string{})
	f([]string{}, []string{})

	// Single element — no duplicates possible.
	f([]string{"a"}, []string{"a"})

	// All elements distinct — nothing deduped.
	f([]string{"a", "b", "c"}, []string{"a", "b", "c"})

	// All elements identical — collapse to one.
	f([]string{"x", "x", "x"}, []string{"x"})

	// Mixed duplicates.
	f([]string{"foo", "bar", "foo", "baz", "bar"}, []string{"foo", "bar", "baz"})

	// Large number of duplicates across two strings.
	big := make([]string, 0, 1000)
	for range 500 {
		big = append(big, "alpha", "beta")
	}
	f(big, []string{"alpha", "beta"})

	// Unicode strings are handled correctly.
	f([]string{"中文", "日本語", "中文", "한국어", "日本語"}, []string{"中文", "日本語", "한국어"})
}

func BenchmarkDeduplicateStrings(b *testing.B) {
	for _, n := range []int{10, 100, 1000, 10000} {
		input := make([]string, n*2)
		for i := range n {
			s := fmt.Sprintf("label_name_%d", i)
			input[i] = s
			input[i+n] = s // every string appears twice
		}
		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				cp := make([]string, len(input))
				copy(cp, input)
				deduplicateStrings(cp)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// binarySearchTimestamps
// ---------------------------------------------------------------------------

func TestBinarySearchTimestamps(t *testing.T) {
	f := func(timestamps []int64, ts int64, expected int) {
		t.Helper()
		got := binarySearchTimestamps(timestamps, ts)
		if got != expected {
			t.Fatalf("binarySearchTimestamps(%v, %d)=%d, want %d", timestamps, ts, got, expected)
		}
	}

	// Empty slice: always returns 0.
	f(nil, 0, 0)
	f([]int64{}, 100, 0)

	// Single element: ts < element.
	f([]int64{5}, 4, 0)

	// Single element: ts == element.
	f([]int64{5}, 5, 1)

	// Single element: ts > element (fast-path in implementation).
	f([]int64{5}, 6, 1)

	// Multiple elements, ts before all.
	f([]int64{10, 20, 30, 40, 50}, 5, 0)

	// Multiple elements, ts equals first.
	f([]int64{10, 20, 30, 40, 50}, 10, 1)

	// Multiple elements, ts between two elements.
	f([]int64{10, 20, 30, 40, 50}, 25, 2)

	// Multiple elements, ts equals last.
	f([]int64{10, 20, 30, 40, 50}, 50, 5)

	// Multiple elements, ts after all (fast path: last element <= ts).
	f([]int64{10, 20, 30, 40, 50}, 100, 5)

	// Negative timestamps.
	f([]int64{-50, -30, -10, 0, 10}, -30, 2)
	f([]int64{-50, -30, -10, 0, 10}, -20, 2)
	f([]int64{-50, -30, -10, 0, 10}, -100, 0)

	// Large slice: verifies binary search correctness, not just boundary scan.
	large := make([]int64, 10000)
	for i := range large {
		large[i] = int64(i * 2) // even numbers: 0, 2, 4, ..., 19998
	}
	f(large, -1, 0)
	f(large, 0, 1)
	f(large, 1, 1)       // between 0 and 2 → idx 1
	f(large, 9998, 5000) // == large[4999], so count=5000
	f(large, 9999, 5000) // between 9998 and 10000
	f(large, 19998, 10000)
	f(large, 19999, 10000) // fast path (> last)
	f(large, 20000, 10000) // fast path (> last)
}

func BenchmarkBinarySearchTimestamps(b *testing.B) {
	for _, n := range []int{10, 100, 1000, 10000} {
		ts := make([]int64, n)
		for i := range n {
			ts[i] = int64(i * 2)
		}
		// Falls between two elements — exercises full binary search, not fast-path.
		target := ts[n/2] + 1

		b.Run(fmt.Sprintf("n=%d", n), func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				binarySearchTimestamps(ts, target)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// toTopHeapEntries
// ---------------------------------------------------------------------------

func TestToTopHeapEntries(t *testing.T) {
	f := func(m map[string]uint64, topN int, expected []storage.TopHeapEntry) {
		t.Helper()
		got := toTopHeapEntries(m, topN)
		if len(got) != len(expected) {
			t.Fatalf("len mismatch: got %d, want %d\ngot=%v\nwant=%v", len(got), len(expected), got, expected)
		}
		for i := range got {
			if got[i] != expected[i] {
				t.Fatalf("entry[%d]: got %+v, want %+v", i, got[i], expected[i])
			}
		}
	}

	// Empty map.
	f(map[string]uint64{}, 10, []storage.TopHeapEntry{})

	// Single entry, topN larger than map.
	f(map[string]uint64{"a": 5}, 10, []storage.TopHeapEntry{{Name: "a", Count: 5}})

	// topN larger than map size; all entries returned sorted descending by count.
	f(
		map[string]uint64{"c": 1, "a": 3, "b": 2},
		10,
		[]storage.TopHeapEntry{{Name: "a", Count: 3}, {Name: "b", Count: 2}, {Name: "c", Count: 1}},
	)

	// topN limits the result to the N largest.
	f(
		map[string]uint64{"c": 1, "a": 3, "b": 2},
		2,
		[]storage.TopHeapEntry{{Name: "a", Count: 3}, {Name: "b", Count: 2}},
	)

	// Ties in count are broken alphabetically by name (ascending).
	f(
		map[string]uint64{"beta": 10, "alpha": 10, "gamma": 10},
		3,
		[]storage.TopHeapEntry{
			{Name: "alpha", Count: 10},
			{Name: "beta", Count: 10},
			{Name: "gamma", Count: 10},
		},
	)

	// topN larger than number of entries — return all entries.
	f(
		map[string]uint64{"x": 7, "y": 3},
		100,
		[]storage.TopHeapEntry{{Name: "x", Count: 7}, {Name: "y", Count: 3}},
	)

	// topN=1: only the largest is returned.
	f(
		map[string]uint64{"a": 5, "b": 9, "c": 1},
		1,
		[]storage.TopHeapEntry{{Name: "b", Count: 9}},
	)
}

// ---------------------------------------------------------------------------
// requestHandler path normalisation
// ---------------------------------------------------------------------------

// normalizePath mirrors the optimised hot-path logic proposed for requestHandler:
// skip ReplaceAll (and its allocation) when "//" is absent.
func normalizePath(raw string) string {
	for strings.Contains(raw, "//") {
		raw = strings.ReplaceAll(raw, "//", "/")
	}
	return raw
}

func TestNormalizePath(t *testing.T) {
	f := func(input, expected string) {
		t.Helper()
		got := normalizePath(input)
		if got != expected {
			t.Fatalf("normalizePath(%q)=%q, want %q", input, got, expected)
		}
	}

	// No double-slash: returned as-is (no allocation on hot path).
	f("", "")
	f("/", "/")
	f("/select/0/prometheus/api/v1/query", "/select/0/prometheus/api/v1/query")

	// Single double-slash: collapsed.
	f("//", "/")
	f("//select", "/select")
	f("/select//0/prometheus", "/select/0/prometheus")

	// Multiple consecutive or non-consecutive double-slashes.
	f("///", "/")
	f("/select///0", "/select/0")
	f("/select//0//prometheus", "/select/0/prometheus")
}

func BenchmarkNormalizePath(b *testing.B) {
	cases := []struct {
		name  string
		input string
	}{
		{"no-double-slash", "/select/0/prometheus/api/v1/query_range"},
		{"one-double-slash", "/select//0/prometheus/api/v1/query_range"},
		{"many-double-slash", "/select//0//prometheus//api//v1//query_range"},
	}
	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for range b.N {
				normalizePath(tc.input)
			}
		})
	}
}
