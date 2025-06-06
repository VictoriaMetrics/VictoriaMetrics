package atomicutil

import (
	"sync/atomic"
)

// Uint64 is like atomic.Uint64, but is protected from false sharing.
type Uint64 struct {
	// The padding prevents false sharing with the previous memory location on widespread platforms with cache line size >= 128.
	_ [128]byte

	atomic.Uint64

	// The padding prevents false sharing with the next memory location on widespread platforms with cache line size >= 128.
	_ [128]byte
}
