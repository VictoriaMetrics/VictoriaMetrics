//go:build !(arm || vm_ensure_xxx64_alignment)

package promql

import (
	"reflect"
	"unsafe"
)

// For some architecture the alignment of float64/int64 is not important.
// It may result in a performance penalty (apparently minor on recent
// processors)
// https://lemire.me/blog/2012/05/31/data-alignment-for-speed-myth-or-reality/#comment-55271

func byteSliceToInt64(b []byte) (a []int64) {
	sh := (*reflect.SliceHeader)(unsafe.Pointer(&a))
	sh.Data = uintptr(unsafe.Pointer(&b[0]))
	sh.Len = len(b) / int(unsafe.Sizeof(a[0]))
	sh.Cap = sh.Len
	return a
}

func byteSliceToFloat64(b []byte) (a []float64) {
	sh := (*reflect.SliceHeader)(unsafe.Pointer(&a))
	sh.Data = uintptr(unsafe.Pointer(&b[0]))
	sh.Len = len(b) / int(unsafe.Sizeof(a[0]))
	sh.Cap = sh.Len
	return a
}
