package slicesutil

import "sync"

// Buffer implements a simple buffer for T.
type Buffer[T any] struct {
	// B is the underlying T slice.
	B []T
}

// Reset resets b.
func (b *Buffer[T]) Reset() {
	b.B = b.B[:0]
}

// BufferPool is a pool of T Buffers.
type BufferPool[T any] struct {
	p sync.Pool
}

// Get obtains a Buffer from bp.
func (bp *BufferPool[T]) Get() *Buffer[T] {
	bbv := bp.p.Get()
	if bbv == nil {
		return &Buffer[T]{}
	}
	return bbv.(*Buffer[T])
}

// Put puts b into bp.
func (bp *BufferPool[T]) Put(b *Buffer[T]) {
	b.Reset()
	bp.p.Put(b)
}
