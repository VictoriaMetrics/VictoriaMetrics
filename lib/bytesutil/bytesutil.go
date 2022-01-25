package bytesutil

import (
	"reflect"
	"unsafe"
)

// ResizeWithCopy resizes b to n bytes and returns the resized buffer (which may be newly allocated).
//
// If newly allocated buffer is returned then b contents is copied to it.
func ResizeWithCopy(b []byte, n int) []byte {
	if n <= cap(b) {
		return b[:n]
	}
	// Allocate the exact number of bytes instead of using `b = append(b[:cap(b)], make([]byte, nn)...)`,
	// since `append()` may allocate more than the requested bytes for additional capacity.
	// Using make() instead of append() should save RAM when the resized slice is cached somewhere.
	bNew := make([]byte, n)
	copy(bNew, b)
	return bNew
}

// ResizeNoCopy resizes b to n bytes and returns the resized buffer (which may be newly allocated).
//
// If newly allocated buffer is returned then b contents isn't copied to it.
func ResizeNoCopy(b []byte, n int) []byte {
	if n <= cap(b) {
		return b[:n]
	}
	return make([]byte, n)
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
func ToUnsafeBytes(s string) (b []byte) {
	sh := (*reflect.StringHeader)(unsafe.Pointer(&s))
	slh := (*reflect.SliceHeader)(unsafe.Pointer(&b))
	slh.Data = sh.Data
	slh.Len = sh.Len
	slh.Cap = sh.Len
	return b
}
