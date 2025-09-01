package roaring

// Values returns an iterator that yields the elements of the bitmap in
// increasing order. Starting with Go 1.23, users can use a for loop to iterate
// over it.
func Values(b *Bitmap) func(func(uint32) bool) {
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
func Backward(b *Bitmap) func(func(uint32) bool) {
	return func(yield func(uint32) bool) {
		it := b.ReverseIterator()
		for it.HasNext() {
			if !yield(it.Next()) {
				return
			}
		}
	}
}
