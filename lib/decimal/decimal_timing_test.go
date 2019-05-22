package decimal

import (
	"fmt"
	"math/rand"
	"sync/atomic"
	"testing"
)

func BenchmarkAppendDecimalToFloat(b *testing.B) {
	b.ReportAllocs()
	b.SetBytes(int64(len(testVA)))
	b.RunParallel(func(pb *testing.PB) {
		var fa []float64
		for pb.Next() {
			fa = AppendDecimalToFloat(fa[:0], testVA, 0)
			atomic.AddUint64(&Sink, uint64(len(fa)))
		}
	})
}

func BenchmarkAppendFloatToDecimal(b *testing.B) {
	b.Run("RealFloat", func(b *testing.B) {
		benchmarkAppendFloatToDecimal(b, testFAReal)
	})
	b.Run("Integers", func(b *testing.B) {
		benchmarkAppendFloatToDecimal(b, testFAInteger)
	})
}

func benchmarkAppendFloatToDecimal(b *testing.B, fa []float64) {
	b.ReportAllocs()
	b.SetBytes(int64(len(fa)))
	b.RunParallel(func(pb *testing.PB) {
		var da []int64
		var e int16
		var sink uint64
		for pb.Next() {
			da, e = AppendFloatToDecimal(da[:0], fa)
			sink += uint64(len(da))
			sink += uint64(e)
		}
		atomic.AddUint64(&Sink, sink)
	})
}

var testFAReal = func() []float64 {
	fa := make([]float64, 8*1024)
	for i := 0; i < len(fa); i++ {
		fa[i] = rand.NormFloat64() * 1e6
	}
	return fa
}()

var testFAInteger = func() []float64 {
	fa := make([]float64, 8*1024)
	for i := 0; i < len(fa); i++ {
		fa[i] = float64(int(rand.NormFloat64() * 1e6))
	}
	return fa
}()

var testVA = func() []int64 {
	va, _ := AppendFloatToDecimal(nil, testFAReal)
	return va
}()

func BenchmarkFromFloat(b *testing.B) {
	for _, f := range []float64{0, 1234, 12334345, 12343.4344, 123.45678901e12, 12.3454435e30} {
		b.Run(fmt.Sprintf("%g", f), func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(1)
			b.RunParallel(func(pb *testing.PB) {
				var sink uint64
				for pb.Next() {
					v, e := FromFloat(f)
					sink += uint64(v)
					sink += uint64(e)
				}
				atomic.AddUint64(&Sink, sink)
			})
		})
	}
}

var Sink uint64
