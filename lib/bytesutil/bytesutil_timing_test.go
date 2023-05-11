package bytesutil

import (
	"sync/atomic"
	"testing"
)

func BenchmarkToUnsafeString(b *testing.B) {
	buf := []byte("foobarbaz abcde fafdsfds")
	b.ReportAllocs()
	b.SetBytes(int64(len(buf)))
	b.RunParallel(func(pb *testing.PB) {
		n := 0
		for pb.Next() {
			for i := range buf {
				s := ToUnsafeString(buf[:i])
				n += len(s)
			}
		}
		atomic.AddUint64(&Sink, uint64(n))
	})
}

func BenchmarkToUnsafeBytes(b *testing.B) {
	s := "foobarbaz abcde fafdsfds"
	b.ReportAllocs()
	b.SetBytes(int64(len(s)))
	b.RunParallel(func(pb *testing.PB) {
		n := 0
		for pb.Next() {
			for i := range s {
				s := ToUnsafeBytes(s[:i])
				n += len(s)
			}
		}
		atomic.AddUint64(&Sink, uint64(n))
	})
}

var Sink uint64
