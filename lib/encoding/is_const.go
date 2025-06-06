//go:build !amd64
// +build !amd64

package encoding

import (
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fastnum"
)

// isConst returns true if a contains only equal values.
func IsConst(a []int64) bool {
	if len(a) == 0 {
		return false
	}
	if fastnum.IsInt64Zeros(a) {
		// Fast path for array containing only zeros.
		return true
	}
	if fastnum.IsInt64Ones(a) {
		// Fast path for array containing only ones.
		return true
	}
	v1 := a[0]
	for _, v := range a {
		if v != v1 {
			return false
		}
	}
	return true
}

func AreConstUint64s(a []uint64) bool {
	if len(a) == 0 {
		return false
	}
	v := a[0]
	for i := 1; i < len(a); i++ {
		if v != a[i] {
			return false
		}
	}
	return true
}
