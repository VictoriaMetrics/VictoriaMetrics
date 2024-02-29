package decimal

import (
	"fmt"
	"math/rand"
	"sync/atomic"
	"testing"
)

func BenchmarkCalbirateScale(b *testing.B) {
	aSrc := []int64{1, 2, 3, 4, 5, 67, 8, 9, 12, 324, 29, -34, -94, -84, -34, 0, 2, 3}
	bSrc := []int64{2, 3, 4, 5, 67, 8, 9, 12, 324, 29, -34, -94, -84, -34, 0, 2, 3, 1}
	const aScale = 1
	const bScale = 2
	const scaleExpected = 1

	b.ReportAllocs()
	b.SetBytes(int64(len(aSrc)))
	b.RunParallel(func(pb *testing.PB) {
		var a, b []int64
		for pb.Next() {
			a = append(a[:0], aSrc...)
			b = append(b[:0], bSrc...)
			scale := CalibrateScale(a, aScale, b, bScale)
			if scale != scaleExpected {
				panic(fmt.Errorf("unexpected scale; got %d; want %d", scale, scaleExpected))
			}
		}
	})
}

func BenchmarkAppendDecimalToFloat(b *testing.B) {
	b.Run("RealFloat", func(b *testing.B) {
		benchmarkAppendDecimalToFloat(b, testVA, vaScale)
	})
	b.Run("Integers", func(b *testing.B) {
		benchmarkAppendDecimalToFloat(b, testIntegers, integersScale)
	})
	b.Run("Zeros", func(b *testing.B) {
		benchmarkAppendDecimalToFloat(b, testZeros, 0)
	})
	b.Run("Ones", func(b *testing.B) {
		benchmarkAppendDecimalToFloat(b, testOnes, 0)
	})
}

func benchmarkAppendDecimalToFloat(b *testing.B, a []int64, scale int16) {
	b.ReportAllocs()
	b.SetBytes(int64(len(a)))
	b.RunParallel(func(pb *testing.PB) {
		var fa []float64
		for pb.Next() {
			fa = AppendDecimalToFloat(fa[:0], a, scale)
			Sink.Add(uint64(len(fa)))
		}
	})
}

var testZeros = make([]int64, 8*1024)
var testOnes = func() []int64 {
	a := make([]int64, 8*1024)
	for i := 0; i < len(a); i++ {
		a[i] = 1
	}
	return a
}()

func BenchmarkAppendFloatToDecimal(b *testing.B) {
	b.Run("RealFloat", func(b *testing.B) {
		benchmarkAppendFloatToDecimal(b, testFAReal)
	})
	b.Run("Integers", func(b *testing.B) {
		benchmarkAppendFloatToDecimal(b, testFAInteger)
	})
	b.Run("Zeros", func(b *testing.B) {
		benchmarkAppendFloatToDecimal(b, testFZeros)
	})
	b.Run("Ones", func(b *testing.B) {
		benchmarkAppendFloatToDecimal(b, testFOnes)
	})
}

var testFZeros = make([]float64, 8*1024)
var testFOnes = func() []float64 {
	a := make([]float64, 8*1024)
	for i := 0; i < len(a); i++ {
		a[i] = 1
	}
	return a
}()

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
		Sink.Add(sink)
	})
}

var testFAReal = func() []float64 {
	r := rand.New(rand.NewSource(1))
	fa := make([]float64, 8*1024)
	for i := 0; i < len(fa); i++ {
		fa[i] = r.NormFloat64() * 1e-6
	}
	return fa
}()

var testFAInteger = func() []float64 {
	r := rand.New(rand.NewSource(2))
	fa := make([]float64, 8*1024)
	for i := 0; i < len(fa); i++ {
		fa[i] = float64(int(r.NormFloat64() * 1e6))
	}
	return fa
}()

var testVA, vaScale = AppendFloatToDecimal(nil, testFAReal)
var testIntegers, integersScale = AppendFloatToDecimal(nil, testFAInteger)

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
				Sink.Add(sink)
			})
		})
	}
}

var Sink atomic.Uint64
