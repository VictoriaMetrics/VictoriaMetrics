package atomicutil

import (
	"runtime"
	"sync/atomic"
	"testing"
)

func BenchmarkSlice(b *testing.B) {
	const loops = 1000
	var s Slice[int]

	b.ReportAllocs()
	b.SetBytes(1)

	var workerIDSource atomic.Uint64
	b.RunParallel(func(pb *testing.PB) {
		runtime.Gosched()
		workerID := uint(workerIDSource.Add(1) - 1)
		for pb.Next() {
			p := s.Get(workerID)
			for i := 0; i < loops; i++ {
				*p += i
			}
		}
	})

	result := s.GetSlice()
	sum := 0
	for _, p := range result {
		sum += *p
	}
	Sink.Add(uint64(sum))
}

func BenchmarkStandardSlice_Prealloc(b *testing.B) {
	const loops = 1000
	gomaxprocs := runtime.GOMAXPROCS(-1)
	a := make([]int, gomaxprocs)
	s := make([]*int, gomaxprocs)
	for i := range s {
		s[i] = &a[i]
	}

	b.ReportAllocs()
	b.SetBytes(1)

	var workerIDSource atomic.Uint64
	b.RunParallel(func(pb *testing.PB) {
		runtime.Gosched()
		workerID := uint(workerIDSource.Add(1) - 1)
		for pb.Next() {
			p := s[workerID]
			for i := 0; i < loops; i++ {
				*p += i
			}
		}
	})

	sum := 0
	for _, p := range s {
		sum += *p
	}
	Sink.Add(uint64(sum))
}

func BenchmarkStandardSlice_PerCPUAlloc(b *testing.B) {
	const loops = 1000
	gomaxprocs := runtime.GOMAXPROCS(-1)
	s := make([]*int, gomaxprocs)

	b.ReportAllocs()
	b.SetBytes(1)

	var workerIDSource atomic.Uint64
	b.RunParallel(func(pb *testing.PB) {
		runtime.Gosched()
		workerID := uint(workerIDSource.Add(1) - 1)
		s[workerID] = new(int)
		for pb.Next() {
			p := s[workerID]
			for i := 0; i < loops; i++ {
				*p += i
			}
		}
	})

	sum := 0
	for _, p := range s {
		sum += *p
	}
	Sink.Add(uint64(sum))
}

var Sink atomic.Uint64
