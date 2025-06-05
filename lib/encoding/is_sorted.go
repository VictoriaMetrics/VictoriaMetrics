//go:build !amd64
// +build !amd64

package encoding

func IsInt64ArraySorted(a []int64) bool {
	for i := range a {
		if i > 0 && a[i] < a[i-1] {
			return false
		}
	}
	return true
}
