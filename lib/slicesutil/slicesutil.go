package slicesutil

// ExtendCapacity returns a with the capacity extended to len(a)+itemsToAdd.
//
// It may allocate new slice if cap(a) is smaller than len(a)+itemsToAdd.
func ExtendCapacity[T any](a []T, itemsToAdd int) []T {
	aLen := len(a)
	if n := aLen + itemsToAdd - cap(a); n > 0 {
		a = append(a[:cap(a)], make([]T, n)...)
		return a[:aLen]
	}
	return a
}

// SetLength sets len(a) to newLen and returns the result.
//
// It may allocate new slice if cap(a) is smaller than newLen.
func SetLength[T any](a []T, newLen int) []T {
	if n := newLen - cap(a); n > 0 {
		a = append(a[:cap(a)], make([]T, n)...)
	}
	return a[:newLen]
}
