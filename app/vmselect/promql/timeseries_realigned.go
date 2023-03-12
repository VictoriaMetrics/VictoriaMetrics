//go:build arm || vm_ensure_xxx64_alignment

package promql

import (
	"reflect"
	"unsafe"
)

// For some architecture the alignment of float64/int64 is important.

func byteSliceToInt64(b []byte) (a []int64) {
	// check if the bytes must be moved, for proper int64 alignment
	addr := uintptr(unsafe.Pointer(&b[0]))
	if mod := int(addr % unsafe.Alignof(&a[0])); mod != 0 {
		a = make([]int64, len(b)/int(unsafe.Sizeof(a[0])))
		ab := int64ToByteSlice(a)
		copy(ab, b)
		return a
	}

	sh := (*reflect.SliceHeader)(unsafe.Pointer(&a))
	sh.Data = uintptr(unsafe.Pointer(&b[0]))
	sh.Len = len(b) / int(unsafe.Sizeof(a[0]))
	sh.Cap = sh.Len
	return a
}

func byteSliceToFloat64(b []byte) (a []float64) {
	// check if the bytes must be moved, for proper float64 alignment
	addr := uintptr(unsafe.Pointer(&b[0]))
	if mod := int(addr % unsafe.Alignof(&a[0])); mod != 0 {
		a = make([]float64, len(b)/int(unsafe.Sizeof(a[0])))
		ab := float64ToByteSlice(a)
		copy(ab, b)
		return a
	}

	sh := (*reflect.SliceHeader)(unsafe.Pointer(&a))
	sh.Data = uintptr(unsafe.Pointer(&b[0]))
	sh.Len = len(b) / int(unsafe.Sizeof(a[0]))
	sh.Cap = sh.Len
	return a
}
