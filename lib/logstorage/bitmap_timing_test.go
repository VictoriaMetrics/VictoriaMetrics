package logstorage

import (
	"testing"
)

func BenchmarkBitmapForEachSetBit(b *testing.B) {
	const bitsLen = 64*1024

	b.Run("no-zero-bits-noclear", func(b *testing.B) {
		bm := getBitmap(bitsLen)
		bm.setBits()
		benchmarkBitmapForEachSetBit(b, bm, false)
		putBitmap(bm)
	})
	b.Run("no-zero-bits-clear", func(b *testing.B) {
		bm := getBitmap(bitsLen)
		bm.setBits()
		benchmarkBitmapForEachSetBit(b, bm, true)
		putBitmap(bm)
	})
	b.Run("half-zero-bits-noclear", func(b *testing.B) {
		bm := getBitmap(bitsLen)
		bm.setBits()
		bm.forEachSetBit(func(idx int) bool {
			return idx%2 == 0
		})
		benchmarkBitmapForEachSetBit(b, bm, false)
		putBitmap(bm)
	})
	b.Run("half-zero-bits-clear", func(b *testing.B) {
		bm := getBitmap(bitsLen)
		bm.setBits()
		bm.forEachSetBit(func(idx int) bool {
			return idx%2 == 0
		})
		benchmarkBitmapForEachSetBit(b, bm, true)
		putBitmap(bm)
	})
	b.Run("one-set-bit-noclear", func(b *testing.B) {
		bm := getBitmap(bitsLen)
		bm.setBits()
		bm.forEachSetBit(func(idx int) bool {
			return idx == bitsLen/2
		})
		benchmarkBitmapForEachSetBit(b, bm, false)
		putBitmap(bm)
	})
	b.Run("one-set-bit-clear", func(b *testing.B) {
		bm := getBitmap(bitsLen)
		bm.setBits()
		bm.forEachSetBit(func(idx int) bool {
			return idx == bitsLen/2
		})
		benchmarkBitmapForEachSetBit(b, bm, true)
		putBitmap(bm)
	})
}

func benchmarkBitmapForEachSetBit(b *testing.B, bm *bitmap, isClearBits bool) {
	b.SetBytes(int64(bm.bitsLen))
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		bmLocal := getBitmap(bm.bitsLen)
		for pb.Next() {
			bmLocal.copyFrom(bm)
			bmLocal.forEachSetBit(func(idx int) bool {
				return !isClearBits
			})
			if isClearBits {
				if !bmLocal.isZero() {
					panic("BUG: bitmap must have no set bits")
				}
			} else {
				if bmLocal.isZero() {
					panic("BUG: bitmap must have some set bits")
				}
			}
		}
		putBitmap(bmLocal)
	})
}
