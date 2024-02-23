package logstorage

import (
	"fmt"
	"sync/atomic"
	"testing"
)

func BenchmarkHash128(b *testing.B) {
	a := make([][]byte, 100)
	for i := range a {
		a[i] = []byte(fmt.Sprintf("some string %d", i))
	}
	b.ReportAllocs()
	b.SetBytes(int64(len(a)))
	b.RunParallel(func(pb *testing.PB) {
		var n uint64
		for pb.Next() {
			for _, b := range a {
				h := hash128(b)
				n += h.hi
				n += h.lo
			}
		}
		GlobalSinkU64.Add(n)
	})
}

var GlobalSinkU64 atomic.Uint64
