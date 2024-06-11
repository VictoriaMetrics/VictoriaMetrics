package logstorage

import (
	"testing"
)

func TestBitmap(t *testing.T) {
	for i := 0; i < 100; i++ {
		bitsLen := i

		bm := getBitmap(i)
		if bm.bitsLen != i {
			t.Fatalf("unexpected bits length: %d; want %d", bm.bitsLen, i)
		}

		if !bm.isZero() {
			t.Fatalf("all the bits must be zero for bitmap with %d bits", i)
		}
		if i == 0 && !bm.areAllBitsSet() {
			t.Fatalf("areAllBitsSet() must return true for bitmap with 0 bits")
		}
		if i > 0 && bm.areAllBitsSet() {
			t.Fatalf("areAllBitsSet() must return false on new bitmap with %d bits; %#v", i, bm)
		}
		if n := bm.onesCount(); n != 0 {
			t.Fatalf("unexpected number of set bits; got %d; want %d", n, 0)
		}

		bm.setBits()

		if n := bm.onesCount(); n != i {
			t.Fatalf("unexpected number of set bits; got %d; want %d", n, i)
		}

		// Make sure that all the bits are set.
		nextIdx := 0
		bm.forEachSetBitReadonly(func(idx int) {
			if idx >= i {
				t.Fatalf("index must be smaller than %d", i)
			}
			if idx != nextIdx {
				t.Fatalf("unexpected idx; got %d; want %d", idx, nextIdx)
			}
			nextIdx++
		})
		if nextIdx != bitsLen {
			t.Fatalf("unexpected number of bits set; got %d; want %d", nextIdx, bitsLen)
		}

		if !bm.areAllBitsSet() {
			t.Fatalf("all bits must be set for bitmap with %d bits", i)
		}

		// Clear a part of bits
		bm.forEachSetBit(func(idx int) bool {
			return idx%2 != 0
		})

		if i <= 1 && !bm.isZero() {
			t.Fatalf("bm.isZero() must return true for bitmap with %d bits", i)
		}
		if i > 1 && bm.isZero() {
			t.Fatalf("bm.isZero() must return false, since some bits are set for bitmap with %d bits", i)
		}
		if i == 0 && !bm.areAllBitsSet() {
			t.Fatalf("areAllBitsSet() must return true for bitmap with 0 bits")
		}
		if i > 0 && bm.areAllBitsSet() {
			t.Fatalf("some bits mustn't be set for bitmap with %d bits", i)
		}

		nextIdx = 1
		bm.forEachSetBitReadonly(func(idx int) {
			if idx != nextIdx {
				t.Fatalf("unexpected idx; got %d; want %d", idx, nextIdx)
			}
			nextIdx += 2
		})
		if nextIdx < bitsLen {
			t.Fatalf("unexpected number of bits visited; got %d; want %d", nextIdx, bitsLen)
		}

		// Clear all the bits
		bm.forEachSetBit(func(_ int) bool {
			return false
		})

		if !bm.isZero() {
			t.Fatalf("all the bits must be reset for bitmap with %d bits", i)
		}
		if i == 0 && !bm.areAllBitsSet() {
			t.Fatalf("allAllBitsSet() must return true for bitmap with 0 bits")
		}
		if i > 0 && bm.areAllBitsSet() {
			t.Fatalf("areAllBitsSet() must return false for bitmap with %d bits", i)
		}
		if n := bm.onesCount(); n != 0 {
			t.Fatalf("unexpected number of set bits; got %d; want %d", n, 0)
		}

		bitsCount := 0
		bm.forEachSetBitReadonly(func(_ int) {
			bitsCount++
		})
		if bitsCount != 0 {
			t.Fatalf("unexpected non-zero number of set bits remained: %d", bitsCount)
		}

		putBitmap(bm)
	}
}
