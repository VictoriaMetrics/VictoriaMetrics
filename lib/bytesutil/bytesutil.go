package bytesutil

import (
	"reflect"
	"runtime"
	"unsafe"
)

// Resize resizes b to n bytes and returns b (which may be newly allocated).
func Resize(b []byte, n int) []byte {
	if nn := n - cap(b); nn > 0 {
		b = append(b[:cap(b)], make([]byte, nn)...)
	}
	return b[:n]
}

// ToUnsafeString converts b to string without memory allocations.
//
// The returned string is valid only until b is reachable and unmodified.
func ToUnsafeString(b []byte) string {
	return *(*string)(unsafe.Pointer(&b))
}

// ToUnsafeBytes converts s to a byte slice without memory allocations.
//
// The returned byte slice is valid only until s is reachable and unmodified.
func ToUnsafeBytes(s string) []byte {
	sh := (*reflect.StringHeader)(unsafe.Pointer(&s))
	var slh reflect.SliceHeader
	slh.Data = sh.Data
	slh.Len = sh.Len
	slh.Cap = sh.Len
	b := *(*[]byte)(unsafe.Pointer(&slh))
	runtime.KeepAlive(s)
	return b
}
