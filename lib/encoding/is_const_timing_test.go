package encoding

import (
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fastnum"
)

/*
goos: linux
goarch: amd64
pkg: github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding
cpu: Intel(R) Xeon(R) Platinum 8260 CPU @ 2.40GHz
Benchmark_is_const-8        3028            420935 ns/op        19928.63 MB/s          0 B/op          0 allocs/op

47.9% fast then old version
*/
func Benchmark_is_const(b *testing.B) {
	cnt := 1024*1024 + 7
	arr := getData(cnt)
	b.SetBytes(int64(cnt * 8))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ret := IsConst(arr)
		if !ret {
			b.Fatalf("ret=%v", ret)
		}
	}
}

func isConstSlow(a []int64) bool {
	if len(a) == 0 {
		return false
	}
	if fastnum.IsInt64Zeros(a) {
		// Fast path for array containing only zeros.
		return true
	}
	if fastnum.IsInt64Ones(a) {
		// Fast path for array containing only ones.
		return true
	}
	v1 := a[0]
	for _, v := range a {
		if v != v1 {
			return false
		}
	}
	return true
}

/*
goos: linux
goarch: amd64
pkg: github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding
cpu: Intel(R) Xeon(R) Platinum 8260 CPU @ 2.40GHz
=== RUN   Benchmark_is_const_slow
Benchmark_is_const_slow
Benchmark_is_const_slow-8           1348            808103 ns/op        10380.69 MB/s          0 B/op          0 allocs/op
*/
func Benchmark_is_const_slow(b *testing.B) {
	cnt := 1024*1024 + 7
	arr := getData(cnt)
	b.SetBytes(int64(cnt * 8))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ret := isConstSlow(arr)
		if !ret {
			b.Fatalf("ret=%v", ret)
		}
	}
}
