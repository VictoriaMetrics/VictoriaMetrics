package logstorage

import (
	"testing"
)

func BenchmarkBitmapForEachSetBitReadonly(b *testing.B) {
	const bitsLen = 64 * 1024

	b.Run("no-zero-bits", func(b *testing.B) {
		bm := getBitmap(bitsLen)
		bm.setBits()
		benchmarkBitmapForEachSetBitReadonly(b, bm)
		putBitmap(bm)
	})
	b.Run("half-zero-bits", func(b *testing.B) {
		bm := getBitmap(bitsLen)
		bm.setBits()
		bm.forEachSetBit(func(idx int) bool {
			return idx%2 == 0
		})
		benchmarkBitmapForEachSetBitReadonly(b, bm)
		putBitmap(bm)
	})
	b.Run("one-set-bit", func(b *testing.B) {
		bm := getBitmap(bitsLen)
		bm.setBits()
		bm.forEachSetBit(func(idx int) bool {
			return idx == bitsLen/2
		})
		benchmarkBitmapForEachSetBitReadonly(b, bm)
		putBitmap(bm)
	})
}

func BenchmarkBitmapForEachSetBit(b *testing.B) {
	const bitsLen = 64 * 1024

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

func benchmarkBitmapForEachSetBitReadonly(b *testing.B, bm *bitmap) {
	b.SetBytes(int64(bm.bitsLen))
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		bmLocal := getBitmap(bm.bitsLen)
		n := 0
		for pb.Next() {
			bmLocal.copyFrom(bm)
			bmLocal.forEachSetBitReadonly(func(_ int) {
				n++
			})
		}
		putBitmap(bmLocal)
		GlobalSink.Add(uint64(n))
	})
}

func benchmarkBitmapForEachSetBit(b *testing.B, bm *bitmap, isClearBits bool) {
	b.SetBytes(int64(bm.bitsLen))
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		bmLocal := getBitmap(bm.bitsLen)
		n := 0
		for pb.Next() {
			bmLocal.copyFrom(bm)
			bmLocal.forEachSetBit(func(_ int) bool {
				n++
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
		GlobalSink.Add(uint64(n))
	})
}
