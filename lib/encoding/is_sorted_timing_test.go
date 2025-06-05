package encoding

import (
	"math/rand"
	"testing"
)

func isSortedSlow(a []int64) bool {
	for i := range a {
		if i > 0 && a[i] < a[i-1] {
			return false
		}
	}
	return true
}

func getDeltaData(cnt int) []int64 {
	arr := make([]int64, cnt)
	seed := rand.Int63n(0x7f7f7f7f7f7f7f7f)
	for i := 0; i < cnt; i++ {
		arr[i] = seed + int64(i)
	}
	return arr
}

/*
go test -benchmem -run=^$ -bench ^BenchmarkIsInt64ArraySorted$ github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding

goos: linux
goarch: amd64
pkg: github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding
cpu: Intel(R) Xeon(R) Platinum 8260 CPU @ 2.40GHz
=== RUN   BenchmarkIsInt64ArraySorted
BenchmarkIsInt64ArraySorted
BenchmarkIsInt64ArraySorted-8                100          11406021 ns/op        7354.54 MB/s           0 B/op          0 allocs/op

golang version: 5134.81 MB/s
30.18% faster
*/
func BenchmarkIsInt64ArraySorted(b *testing.B) {
	cnt := 1024 * 1024 * 10
	arr := getDeltaData(cnt)
	b.SetBytes(int64(cnt * 8))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ret := IsInt64ArraySorted(arr)
		if !ret {
			b.Fatalf("ret=%v", ret)
		}
	}
}

/*
go test -benchmem -run=^$ -bench ^Benchmark_is_sorted_slow$ github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding

goos: linux
goarch: amd64
pkg: github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding
cpu: Intel(R) Xeon(R) Platinum 8260 CPU @ 2.40GHz
=== RUN   Benchmark_is_sorted_slow
Benchmark_is_sorted_slow
Benchmark_is_sorted_slow-8            82          16336747 ns/op        5134.81 MB/s           0 B/op          0 allocs/op
*/
func Benchmark_is_sorted_slow(b *testing.B) {
	cnt := 1024 * 1024 * 10
	arr := getDeltaData(cnt)
	b.SetBytes(int64(cnt * 8))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ret := isSortedSlow(arr)
		if !ret {
			b.Fatalf("ret=%v", ret)
		}
	}
}
