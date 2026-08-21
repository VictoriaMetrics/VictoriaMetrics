package roaring

import (
	"iter"
	"math/bits"
)

// Values returns an iterator that yields the elements of the bitmap in
// increasing order. Starting with Go 1.23, users can use a for loop to iterate
// over it.
func Values(b *Bitmap) iter.Seq[uint32] {
	return func(yield func(uint32) bool) {
		it := b.Iterator()
		for it.HasNext() {
			if !yield(it.Next()) {
				return
			}
		}
	}
}

// Backward returns an iterator that yields the elements of the bitmap in
// decreasing order. Starting with Go 1.23, users can use a for loop to iterate
// over it.
func Backward(b *Bitmap) iter.Seq[uint32] {
	return func(yield func(uint32) bool) {
		it := b.ReverseIterator()
		for it.HasNext() {
			if !yield(it.Next()) {
				return
			}
		}
	}
}

// Unset creates an iterator that yields values in the range [min, max] that are NOT contained in the bitmap.
// The iterator becomes invalid if the bitmap is modified (e.g., with Add or Remove).
func Unset(b *Bitmap, min, max uint32) iter.Seq[uint32] {
	return func(yield func(uint32) bool) {
		it := b.UnsetIterator(uint64(min), uint64(max)+1)
		for it.HasNext() {
			if !yield(it.Next()) {
				return
			}
		}
	}
}

// Ranges iterates contiguous ranges of values present in the bitmap as
// half-open [start, endExclusive) pairs. endExclusive is uint64 to represent
// ranges that include MaxUint32. Ranges spanning container boundaries are merged.
func (b *Bitmap) Ranges() iter.Seq2[uint32, uint64] {
	return func(yield func(uint32, uint64) bool) {
		ra := &b.highlowcontainer
		keys := ra.keys
		containers := ra.containers
		n := len(keys)

		var pendingStart, pendingEnd uint64
		hasPending := false

		emit := func(rStart, rEnd uint64) bool {
			if hasPending && rStart <= pendingEnd {
				if rEnd > pendingEnd {
					pendingEnd = rEnd
				}
				return true
			}
			if hasPending {
				if !yield(uint32(pendingStart), pendingEnd) {
					return false
				}
			}
			pendingStart = rStart
			pendingEnd = rEnd
			hasPending = true
			return true
		}

		for idx := 0; idx < n; idx++ {
			hs := uint64(keys[idx]) << 16
			c := containers[idx]

			switch t := c.(type) {
			case *runContainer16:
				for _, iv := range t.iv {
					if !emit(hs+uint64(iv.start), hs+uint64(iv.start)+uint64(iv.length)+1) {
						return
					}
				}

			case *bitmapContainer:
				bm := t.bitmap
				length := uint(len(bm))
				pos := uint(0)

				for pos < length {
					w := bm[pos]
					if w == 0 {
						pos++
						continue
					}

					for w != 0 {
						lo := uint(bits.TrailingZeros64(w))
						bitStart := pos*64 + lo

						ones := uint(bits.TrailingZeros64(^(w >> lo)))
						if lo+ones < 64 {
							if !emit(hs+uint64(bitStart), hs+uint64(bitStart+ones)) {
								return
							}
							w &= ^((uint64(1) << (lo + ones)) - 1)
						} else {
							pos++
							for pos < length && bm[pos] == 0xFFFFFFFFFFFFFFFF {
								pos++
							}
							var bitEnd uint
							if pos < length {
								trailing := uint(bits.TrailingZeros64(^bm[pos]))
								bitEnd = pos*64 + trailing
								w = bm[pos] & ^((uint64(1) << trailing) - 1)
							} else {
								bitEnd = length * 64
								w = 0
							}
							if !emit(hs+uint64(bitStart), hs+uint64(bitEnd)) {
								return
							}
							continue
						}
					}
					pos++
				}

			case *arrayContainer:
				content := t.content
				i := 0
				for i < len(content) {
					start := uint64(content[i])
					end := start + 1
					i++
					for i < len(content) && uint64(content[i]) == end {
						end++
						i++
					}
					if !emit(hs+start, hs+end) {
						return
					}
				}
			}
		}

		if hasPending {
			yield(uint32(pendingStart), pendingEnd)
		}
	}
}
