package atomicutil

import (
	"sync/atomic"
)

// Slice allows goroutine-safe access to []*T items with automatic growth of the slice.
//
// This is a replacement for [workersCount]*T where workersCount isn't known beforehand.
//
// It also prevents from false sharing of the created T items on multi-CPU systems.
type Slice[T any] struct {
	// Init is an optional callback for initializing the created item x.
	Init func(x *T)

	p atomic.Pointer[[]*itemPadded[T]]
}

type itemPadded[T any] struct {
	x T

	// The padding prevents false sharing
	_ [CacheLineSize]byte
}

// Get returns *T item for the given workerID in a goroutine-safe manner.
//
// The returned item is automatically created via s.New on the first access.
//
// It is expected that only a single goroutine can access *T at workerID index at any given time.
func (s *Slice[T]) Get(workerID uint) *T {
	ap := s.p.Load()
	if ap != nil {
		a := *ap
		if workerID < uint(len(a)) {
			// Fast path - return already created item.
			return &a[workerID].x
		}
	}

	// Slow path - create the item, since it is missing.
	return s.getSlow(workerID)
}

func (s *Slice[T]) getSlow(workerID uint) *T {
	for {
		ap := s.p.Load()
		var a []*itemPadded[T]
		if ap != nil {
			a = *ap
		}
		if workerID < uint(len(a)) {
			return &a[workerID].x
		}

		aNew := make([]*itemPadded[T], workerID+1)
		copy(aNew, a)
		for i := len(a); i < len(aNew); i++ {
			x := new(itemPadded[T])
			if s.Init != nil {
				s.Init(&x.x)
			}
			aNew[i] = x
		}
		if s.p.CompareAndSwap(ap, &aNew) {
			return &aNew[workerID].x
		}
	}
}

// All returns the underlying []*T.
//
// The length of the returned slice equals to the max(workerID)+1 passed to s.Get().
// It is guaranteed that all the items in the returned slice are non-nil.
//
// It is unsafe calling this function when concurrent goroutines access s.
//
// All() is relatively slow, so it shouldn't be called in hot paths.
func (s *Slice[T]) All() []*T {
	ap := s.p.Load()
	if ap == nil {
		return nil
	}

	a := *ap
	result := make([]*T, len(a))
	for i, x := range a {
		result[i] = &x.x
	}
	return result
}
