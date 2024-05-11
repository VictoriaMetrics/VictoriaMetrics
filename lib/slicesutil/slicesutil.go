package slicesutil

// ExtendCapacity returns a with the capacity extended to len(a)+n if needed.
func ExtendCapacity[T any](a []T, n int) []T {
	aLen := len(a)
	if n := aLen + n - cap(a); n > 0 {
		a = append(a[:cap(a)], make([]T, n)...)
	}
	return a[:aLen]
}
