package atomicutil

import (
	"sync/atomic"
	"unsafe"
)

// Uint64 is like atomic.Uint64, but is protected from false sharing.
type Uint64 struct {
	// The padding prevents false sharing with the previous memory location
	_ [CacheLineSize - unsafe.Sizeof(atomic.Uint64{})%CacheLineSize]byte

	atomic.Uint64

	// The padding prevents false sharing with the next memory location
	_ [CacheLineSize - unsafe.Sizeof(atomic.Uint64{})%CacheLineSize]byte
}
