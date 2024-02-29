package bytesutil

import (
	"sync/atomic"
	"testing"
)

func BenchmarkToUnsafeString(b *testing.B) {
	b.ReportAllocs()
	b.SetBytes(int64(len(benchBytes)))
	b.RunParallel(func(pb *testing.PB) {
		n := 0
		for pb.Next() {
			for _, b := range benchBytes {
				s := ToUnsafeString(b)
				n += len(s)
			}
		}
		Sink.Add(uint64(n))
	})
}

func BenchmarkToUnsafeBytes(b *testing.B) {
	b.ReportAllocs()
	b.SetBytes(int64(len(benchStrings)))
	b.RunParallel(func(pb *testing.PB) {
		n := 0
		for pb.Next() {
			for _, s := range benchStrings {
				b := ToUnsafeBytes(s)
				n += len(b)
			}
		}
		Sink.Add(uint64(n))
	})
}

var benchBytes = func() [][]byte {
	a := make([][]byte, 1000)
	for i := range a {
		a[i] = make([]byte, i)
	}
	return a
}()

var benchStrings = func() []string {
	a := make([]string, 1000)
	for i := range a {
		a[i] = string(make([]byte, i))
	}
	return a
}()

var Sink atomic.Uint64
