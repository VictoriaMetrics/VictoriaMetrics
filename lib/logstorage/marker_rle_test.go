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

	return boolRLE(bm.MarshalBoolRLE(nil))
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

	// Basic
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

	// Edge-cases around empties and all-zero blocks
	f("", "0000", "0000")
	f("0000", "", "0000")
	f("0000", "0000", "0000")
	f("1111", "1111", "1111")
	f("0000", "1111", "1111")
	f("1111", "0000", "1111")

	// Classical “two halves” overlap
	f("11110000", "00001111", "11111111")

	// Middle gap preserved
	f("111100001111", "000000001111", "111100001111")

	// Complementary patterns that fill every bit
	f("10000001", "01111110", "11111111")
	f("0001000", "0000100", "0001100")
	f("101010", "001100", "101110")
	f("11001100", "00110011", "11111111")

	// Different lengths (second is shorter)
	f("111000", "001", "111000")

	// Different lengths (second is longer, adds trailing zeros)
	f("11", "001000", "111000")

	// Perfect alternation (maximal run-boundary stress)
	f("10101010", "01010101", "11111111")

	// Ones at both extremes only
	f("1000000001", "0000000001", "1000000001")

	// All zeros remain zeros (makes sure trailing zeros are preserved)
	f("000000", "0", "000000")

	// Second stream adds a 1 just beyond the first stream’s end
	f("111", "0001000", "1111000")

	// Disjoint ones at opposite ends
	f("0001", "1000", "1001")

	// Streams with leading zero-runs of different size
	f("00111100", "00011000", "00111100")
	t.Run("contiguous-large-runs", func(t *testing.T) {
		a := strings.Repeat("1", 362)
		b := strings.Repeat("0", 362) + strings.Repeat("1", 246)
		expected := strings.Repeat("1", 608)
		f(a, b, expected)
	})
}
