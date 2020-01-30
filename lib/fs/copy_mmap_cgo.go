// +build cgo

package fs

// #cgo CFLAGS: -O3
//
// #include <stdint.h>  // for uintptr_t
// #include <string.h>  // for memcpy
//
// // The memcpy_wrapper allows avoiding memory allocations during calls from Go.
// // See https://github.com/golang/go/issues/24450 .
// static void memcpy_wrapper(uintptr_t dst, uintptr_t src, size_t n) {
//     memcpy((void*)dst, (void*)src, n);
// }
import "C"

import (
	"runtime"
	"unsafe"
)

// copyMmap copies len(dst) bytes from src to dst.
func copyMmap(dst, src []byte) {
	// Copy data from mmap'ed src via cgo call in order to protect from goroutine stalls
	// when the copied data isn't available in RAM, so the OS triggers reading the data from file.
	// See https://medium.com/@valyala/mmap-in-go-considered-harmful-d92a25cb161d for details.
	dstPtr := C.uintptr_t(uintptr(unsafe.Pointer(&dst[0])))
	srcPtr := C.uintptr_t(uintptr(unsafe.Pointer(&src[0])))
	C.memcpy_wrapper(dstPtr, srcPtr, C.size_t(len(dst)))

	// Prevent from GC'ing src or dst during C.memcpy_wrapper call.
	runtime.KeepAlive(src)
	runtime.KeepAlive(dst)
}
