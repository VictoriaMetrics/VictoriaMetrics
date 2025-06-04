//go:build !amd64
// +build !amd64

package encoding

// IsDeltaConst returns true if a contains counter with constant delta.
func IsDeltaConst(a []int64) bool {
	if len(a) < 2 {
		return false
	}
	d1 := a[1] - a[0]
	prev := a[1]
	for _, next := range a[2:] {
		if next-prev != d1 {
			return false
		}
		prev = next
	}
	return true
}
