package roaring

import "iter"

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
