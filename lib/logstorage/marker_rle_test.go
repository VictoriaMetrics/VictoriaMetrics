package logstorage

import (
	"bytes"
	"strings"
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

	return boolRLE(bm.MarshalRLE(nil))
}

// equalRLE checks if two RLE bitmaps are equivalent
func equalRLE(a, b boolRLE) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	if len(a) == 0 || len(b) == 0 {
		return false
	}

	// Compare by converting both to bitmap and checking bit-by-bit
	bmA := getBitmap(1000) // sufficient for test cases
	defer putBitmap(bmA)
	bmB := getBitmap(1000)
	defer putBitmap(bmB)

	// Apply RLE to bitmaps (AndNotRLE sets bits where RLE is 1)
	a.AndNotRLE(bmA)
	b.AndNotRLE(bmB)

	// Compare word-by-word
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

		actualRLE := bm.MarshalRLE(nil)
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
	f := func(a, b, expected string) {
		t.Helper()

		rleA := createTestRLE(a)
		rleB := createTestRLE(b)
		expectedRLE := createTestRLE(expected)

		result := rleA.Union(rleB)
		if !equalRLE(result, expectedRLE) {
			t.Fatalf("unexpected result; got %v; want %v", result, expectedRLE)
		}
	}

	f("", "", "")
	f("1", "", "1")
	f("", "1", "1")
	f("0", "1", "1")
	f("1", "0", "1")
	f("101", "010", "111")
	f("1100", "0011", "1111")
	f("1010", "0101", "1111")
	f("1010", "1010", "1010")
	f("11001100", "10101010", "11101110")
	f("11111", "0000011111", "1111111111")
	t.Run("contiguous-large-runs", func(t *testing.T) {
		a := strings.Repeat("1", 362)
		b := strings.Repeat("0", 362) + strings.Repeat("1", 246)
		expected := strings.Repeat("1", 608)
		f(a, b, expected)
	})
}
