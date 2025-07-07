package logstorage

import (
	"bytes"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
)

// createTestRLE creates a test RLE bitmap with specified pattern
func createTestRLE(pattern string) boolRLE {
	bm := getBitmap(len(pattern))
	defer putBitmap(bm)

	for i, c := range pattern {
		if c == '1' {
			bm.setBit(i)
		}
	}

	return boolRLE(bm.MarshalBoolRLE(nil))
}

func applyRLEToBitmap(bm *bitmap, rle boolRLE) {
	idx := 0
	pos := 0
	for idx < len(rle) {
		run, n := encoding.UnmarshalVarUint64(rle[idx:])
		idx += n
		if run > 0 {
			for i := 0; i < int(run); i++ {
				bm.setBit(pos + i)
			}
		}
	}
}

func equalRLE(a, b boolRLE) bool {
	bmA := getBitmap(1000)
	bmB := getBitmap(1000)
	defer putBitmap(bmA)
	defer putBitmap(bmB)

	applyRLEToBitmap(bmA, a) // SET 1-bits
	applyRLEToBitmap(bmB, b)

	if len(bmA.a) != len(bmB.a) {
		return false
	}
	for i := range bmA.a {
		if bmA.a[i] != bmB.a[i] {
			return false
		}
	}
	return true
}

func TestMarshalRLE(t *testing.T) {
	f := func(pattern string, expectedRuns []uint64) {
		t.Helper()

		bm := getBitmap(len(pattern))
		defer putBitmap(bm)
		for i, c := range pattern {
			if c == '1' {
				bm.setBit(i)
			}
		}

		// Build expected RLE from run lengths
		var expectedRLE []byte
		for _, run := range expectedRuns {
			expectedRLE = encoding.MarshalVarUint64(expectedRLE, run)
		}

		actualRLE := bm.MarshalBoolRLE(nil)
		if !bytes.Equal(actualRLE, expectedRLE) {
			t.Fatalf("unexpected result for pattern %s; got %v; want %v", pattern, actualRLE, expectedRLE)
		}
	}

	// Empty and single bits
	f("", []uint64{0})
	f("0", []uint64{1})
	f("1", []uint64{0, 1})

	// Small patterns
	f("00", []uint64{2})
	f("01", []uint64{1, 1})
	f("10", []uint64{0, 1, 1})
	f("11", []uint64{0, 2})

	// Basic runs
	f("000", []uint64{3})
	f("111", []uint64{0, 3})
	f("0001", []uint64{3, 1})
	f("1000", []uint64{0, 1, 3})
	f("0011", []uint64{2, 2})
	f("1100", []uint64{0, 2, 2})

	// Alternating patterns
	f("0101", []uint64{1, 1, 1, 1})
	f("1010", []uint64{0, 1, 1, 1, 1})
	f("010101", []uint64{1, 1, 1, 1, 1, 1})
	f("101010", []uint64{0, 1, 1, 1, 1, 1, 1})

	// Mixed patterns
	f("001100", []uint64{2, 2, 2})
	f("110011", []uint64{0, 2, 2, 2})
	f("00111000", []uint64{2, 3, 3})
	f("11000111", []uint64{0, 2, 3, 3})

	// Edge cases - all zeros/ones
	f("0000000000", []uint64{10})
	f("1111111111", []uint64{0, 10})

	// Single bit surrounded by zeros
	f("000100000", []uint64{3, 1, 5})
	f("000010000", []uint64{4, 1, 4})
	f("000001000", []uint64{5, 1, 3})
}

func TestBoolRLEUnion(t *testing.T) {
	f := func(a, b, expected []uint64) {
		t.Helper()

		rleA := encodeTestRLE(a)
		rleB := encodeTestRLE(b)
		expectedRLE := encodeTestRLE(expected)

		result := rleA.Union(rleB)
		if !equalRLE(result, expectedRLE) {
			t.Fatalf("unexpected result;\n got:      %v\n want:     %v", decodeRLE(result), decodeRLE(expectedRLE))
		}
	}

	// Basic
	f([]uint64{}, []uint64{}, []uint64{})
	f([]uint64{0, 1}, []uint64{}, []uint64{0, 1})
	f([]uint64{}, []uint64{0, 1}, []uint64{0, 1})
	f([]uint64{1}, []uint64{0, 1}, []uint64{0, 1})
	f([]uint64{0, 1}, []uint64{1}, []uint64{0, 1})
	f([]uint64{0, 1, 1, 1}, []uint64{1, 1, 1, 1}, []uint64{0, 4})
	f([]uint64{0, 2, 2}, []uint64{2, 2, 2}, []uint64{0, 4, 2})
	f([]uint64{0, 1, 1, 1}, []uint64{0, 1, 1, 1}, []uint64{0, 1, 1, 1})
	f([]uint64{2, 2, 2, 2}, []uint64{1, 1, 1, 1, 1, 1, 1, 1}, []uint64{1, 3, 1, 3})
	f([]uint64{0, 5}, []uint64{5, 5}, []uint64{0, 10})

	// Edge-cases
	f([]uint64{}, []uint64{4}, []uint64{4})
	f([]uint64{4}, []uint64{}, []uint64{4})
	f([]uint64{4}, []uint64{4}, []uint64{4})
	f([]uint64{0, 4}, []uint64{0, 4}, []uint64{0, 4})
	f([]uint64{4}, []uint64{0, 4}, []uint64{0, 4})
	f([]uint64{0, 4}, []uint64{4}, []uint64{0, 4})

	// Two halves overlap
	f([]uint64{0, 4, 4}, []uint64{4, 4, 4}, []uint64{0, 8, 4})

	// Middle gap
	f([]uint64{0, 4, 4, 4}, []uint64{8, 4}, []uint64{0, 4, 4, 4})

	// Complementary patterns
	f([]uint64{0, 1, 6, 1}, []uint64{1, 6, 1, 1}, []uint64{0, 9})
	f([]uint64{3, 1, 3}, []uint64{4, 1, 2}, []uint64{3, 2, 3})

	// Different lengths
	f([]uint64{0, 3, 3}, []uint64{2, 1}, []uint64{0, 3, 3})
	f([]uint64{0, 2}, []uint64{2, 1, 3}, []uint64{0, 2, 1, 3})

	// Alternation
	f([]uint64{0, 1, 1, 1, 1, 1, 1, 1, 1}, []uint64{1, 1, 1, 1, 1, 1, 1, 1, 1}, []uint64{0, 8, 1})

	// Extremes
	f([]uint64{0, 1, 8, 1}, []uint64{8, 1}, []uint64{0, 1, 7, 2})

	// All zeros remain zeros
	f([]uint64{6}, []uint64{1}, []uint64{6})

	// Second adds a one just beyond the end
	f([]uint64{0, 3}, []uint64{3, 1, 3}, []uint64{0, 4, 3})

	// Disjoint ones at opposite ends
	f([]uint64{3, 1}, []uint64{1, 3}, []uint64{1, 1, 3})

	// Leading zero-runs of different sizes
	f([]uint64{2, 4, 2}, []uint64{3, 2, 3}, []uint64{2, 4, 3})

	// Large runs
	f([]uint64{362}, []uint64{362, 246}, []uint64{362, 246})

	// Real-world testcases
	f(
		[]uint64{1, 4, 1, 16, 1, 52, 2, 24, 1, 32, 1, 4, 1, 7, 1, 10, 1, 6, 1, 53, 1, 28, 1, 41, 1, 14, 1, 1, 1, 87, 1, 29, 1, 15, 1, 1, 1, 36, 1, 13, 1, 37, 1, 18, 1, 8},
		[]uint64{0, 1, 4, 1, 16, 1, 116, 1, 25, 1, 275, 1, 1, 1, 116},
		[]uint64{0, 75, 2, 24, 1, 32, 1, 12, 1, 10, 1, 60, 1, 28, 1, 41, 1, 14, 1, 1, 1, 87, 1, 29, 1, 54, 1, 13, 1, 37, 1, 18, 1, 8})
}

func TestBoolRLEIsSubsetOf(t *testing.T) {
	f := func(a, b []uint64, want bool) {
		t.Helper()
		rleA := encodeTestRLE(a)
		rleB := encodeTestRLE(b)
		got := rleA.IsSubsetOf(rleB)
		if got != want {
			t.Fatalf("IsSubsetOf failed:\n a: %v\n b: %v\n got: %v\nwant: %v", a, b, got, want)
		}
	}

	// Both empty (length 0)
	f([]uint64{}, []uint64{}, true)

	// Subset: all zeros
	f([]uint64{6}, []uint64{6}, true)

	// Subset: a is all zeros, b is all ones
	f([]uint64{6}, []uint64{0, 6}, true)

	// Subset: both all ones
	f([]uint64{0, 6}, []uint64{0, 6}, true)

	// Subset: a is a proper subset of b
	f([]uint64{2, 2, 2}, []uint64{2, 4}, true)
	f([]uint64{0, 2, 2}, []uint64{0, 4}, true)

	// Not subset: a has ones where b has zeros
	f([]uint64{0, 2, 2}, []uint64{2, 2}, false)
	f([]uint64{0, 1, 1}, []uint64{2, 1}, false)

	// Both alternating, a subset of b
	f([]uint64{1, 1, 1, 1}, []uint64{1, 1, 1, 1}, true)
	f([]uint64{1, 1, 1, 1}, []uint64{1, 3}, true)
	f([]uint64{1, 2, 1}, []uint64{1, 1, 1, 1}, false)

	// Edge: a is empty, b is not (always true)
	f([]uint64{}, []uint64{5}, true)

	// Edge: b is empty, a is not (false unless a has no ones)
	f([]uint64{0, 3}, []uint64{}, false)
	f([]uint64{3}, []uint64{}, true)

	// a fully covers b in ones, not a subset
	f([]uint64{0, 6}, []uint64{3, 3}, false)

	//
	f(
		[]uint64{0, 1, 4, 1, 16, 1, 116, 1, 25, 1, 275, 1, 1, 1, 116},
		[]uint64{1, 4, 1, 16, 1, 52, 2, 24, 1, 32, 1, 4, 1, 7, 1, 10, 1, 6, 1, 53, 1, 28, 1, 41, 1, 14, 1, 1, 1, 87, 1, 29, 1, 15, 1, 1, 1, 36, 1, 13, 1, 37, 1, 18, 1, 8},
		true)
}

func encodeTestRLE(runs []uint64) boolRLE {
	var b boolRLE
	for _, x := range runs {
		b = encoding.MarshalVarUint64(b, x)
	}
	return b
}

// decodeRLE decodes a boolRLE slice back to []uint64 for pretty-printing test errors
func decodeRLE(rle boolRLE) []uint64 {
	var res []uint64
	idx := 0
	for idx < len(rle) {
		n, l := encoding.UnmarshalVarUint64(rle[idx:])
		res = append(res, n)
		idx += l
	}
	return res
}
