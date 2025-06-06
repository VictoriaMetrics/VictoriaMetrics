package encoding

import (
	"math/rand"
	"testing"
)

func getDeltaData(cnt int) []int64 {
	arr := make([]int64, cnt)
	seed := rand.Int63n(0x7f7f7f7f7f7f7f7f)
	for i := 0; i < cnt; i++ {
		arr[i] = seed + int64(i)
	}
	return arr
}

// go test -benchmem -run=^$ -bench ^Benchmark_is_delta_const$ github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding
// 6948.91 MB/s  // improve 31.8%
//
//4738.99 MB/s	// golang version,
/*
goos: linux
goarch: amd64
pkg: is_delta_const
cpu: Intel(R) Xeon(R) Platinum 8260 CPU @ 2.40GHz
Benchmark_is_delta_const
Benchmark_is_delta_const-8           100          12071831 ns/op        6948.91 MB/s           0 B/op          0 allocs/op
*/
func Benchmark_is_delta_const(b *testing.B) {
	cnt := 1024 * 1024 * 10
	arr := getDeltaData(cnt)
	b.SetBytes(int64(cnt * 8))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ret := IsDeltaConst(arr)
		if !ret {
			b.Fatalf("ret=%v", ret)
		}
	}
}

// isDeltaConst returns true if a contains counter with constant delta.
func isDeltaConst(a []int64) bool {
	if len(a) < 2 {
		return false
	}
	d1 := a[1] - a[0]
	prev := a[1]
	for _, next := range a[2:] {
		if next-prev != d1 {
			return false
		}
		prev = next
	}
	return true
}

func Benchmark_is_delta_const_slow(b *testing.B) {
	cnt := 1024 * 1024 * 10
	arr := getDeltaData(cnt)
	b.SetBytes(int64(cnt * 8))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ret := isDeltaConst(arr)
		if !ret {
			b.Fatalf("ret=%v", ret)
		}
	}
}
