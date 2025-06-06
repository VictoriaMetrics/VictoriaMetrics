package encoding

import "unsafe"

func IsConst(a []int64) bool

func AreConstUint64s(a []uint64) bool {
	if len(a) == 0 {
		return false
	}
	a1 := unsafe.Slice((*int64)(unsafe.Pointer(&a[0])), len(a))
	return IsConst(a1)
}
