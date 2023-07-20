package slicesutil

import "math/bits"

// ResizeNoCopyMayOverallocate resizes dst to minimum n bytes and returns the resized buffer (which may be newly allocated).
//
// If newly allocated buffer is returned then b contents isn't copied to it.
func ResizeNoCopyMayOverallocate[T any](dst []T, n int) []T {
	if n <= cap(dst) {
		return dst[:n]
	}
	nNew := roundToNearestPow2(n)
	dstNew := make([]T, nNew)
	return dstNew[:n]
}

func roundToNearestPow2(n int) int {
	pow2 := uint8(bits.Len(uint(n - 1)))
	return 1 << pow2
}
