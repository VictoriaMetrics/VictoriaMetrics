package logstorage

import (
	"math/bits"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/slicesutil"
)

func getBitmap(bitsLen int) *bitmap {
	v := bitmapPool.Get()
	if v == nil {
		v = &bitmap{}
	}
	bm := v.(*bitmap)
	bm.resizeNoInit(bitsLen)
	return bm
}

func putBitmap(bm *bitmap) {
	bm.reset()
	bitmapPool.Put(bm)
}

var bitmapPool sync.Pool

type bitmap struct {
	a       []uint64
	bitsLen int
}

func (bm *bitmap) reset() {
	bm.resetBits()
	bm.a = bm.a[:0]

	bm.bitsLen = 0
}

func (bm *bitmap) copyFrom(src *bitmap) {
	bm.reset()

	bm.a = append(bm.a[:0], src.a...)
	bm.bitsLen = src.bitsLen
}

func (bm *bitmap) init(bitsLen int) {
	bm.reset()
	bm.resizeNoInit(bitsLen)
}

func (bm *bitmap) resizeNoInit(bitsLen int) {
	wordsLen := (bitsLen + 63) / 64
	bm.a = slicesutil.SetLength(bm.a, wordsLen)
	bm.bitsLen = bitsLen
}

func (bm *bitmap) resetBits() {
	clear(bm.a)
}

func (bm *bitmap) setBits() {
	a := bm.a
	for i := range a {
		a[i] = ^uint64(0)
	}
	tailBits := bm.bitsLen % 64
	if tailBits > 0 && len(a) > 0 {
		// Zero bits outside bitsLen at the last word
		a[len(a)-1] &= (uint64(1) << tailBits) - 1
	}
}

func (bm *bitmap) isZero() bool {
	for _, word := range bm.a {
		if word != 0 {
			return false
		}
	}
	return true
}

func (bm *bitmap) areAllBitsSet() bool {
	a := bm.a
	for i, word := range a {
		if word != (1<<64)-1 {
			if i+1 < len(a) {
				return false
			}
			tailBits := bm.bitsLen % 64
			if tailBits == 0 || word != (uint64(1)<<tailBits)-1 {
				return false
			}
		}
	}
	return true
}

func (bm *bitmap) andNot(x *bitmap) {
	if bm.bitsLen != x.bitsLen {
		logger.Panicf("BUG: cannot merge bitmaps with distinct lengths; %d vs %d", bm.bitsLen, x.bitsLen)
	}
	if x.isZero() {
		return
	}
	a := bm.a
	b := x.a
	for i := range a {
		a[i] &= ^b[i]
	}
}

func (bm *bitmap) setBit(i int) {
	wordIdx := uint(i) / 64
	wordOffset := uint(i) % 64
	wordPtr := &bm.a[wordIdx]
	*wordPtr |= (1 << wordOffset)
}

func (bm *bitmap) isSetBit(i int) bool {
	wordIdx := uint(i) / 64
	wordOffset := uint(i) % 64
	word := bm.a[wordIdx]
	return (word & (1 << wordOffset)) != 0
}

// forEachSetBit calls f for each set bit and clears that bit if f returns false
func (bm *bitmap) forEachSetBit(f func(idx int) bool) {
	a := bm.a
	bitsLen := bm.bitsLen
	for i, word := range a {
		if word == 0 {
			continue
		}
		wordNew := word
		for j := 0; j < 64; j++ {
			mask := uint64(1) << j
			if (word & mask) == 0 {
				continue
			}
			idx := i*64 + j
			if idx >= bitsLen {
				return
			}
			if !f(idx) {
				wordNew &= ^mask
			}
		}
		if word != wordNew {
			a[i] = wordNew
		}
	}
}

// forEachSetBitReadonly calls f for each set bit
func (bm *bitmap) forEachSetBitReadonly(f func(idx int)) {
	if bm.areAllBitsSet() {
		n := bm.bitsLen
		for i := 0; i < n; i++ {
			f(i)
		}
		return
	}

	a := bm.a
	bitsLen := bm.bitsLen
	for i, word := range a {
		if word == 0 {
			continue
		}
		for j := 0; j < 64; j++ {
			mask := uint64(1) << j
			if (word & mask) == 0 {
				continue
			}
			idx := i*64 + j
			if idx >= bitsLen {
				return
			}
			f(idx)
		}
	}
}

func (bm *bitmap) onesCount() int {
	n := 0
	for _, word := range bm.a {
		n += bits.OnesCount64(word)
	}
	return n
}
