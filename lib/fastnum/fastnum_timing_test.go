package fastnum

import (
	"fmt"
	"sync/atomic"
	"testing"
)

var Sink atomic.Uint64

func BenchmarkAppendInt64Zeros(b *testing.B) {
	for _, n := range []int{10, 100, 1000, 8 * 1024} {
		b.Run(fmt.Sprintf("%d_items", n), func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(n * 8))
			var dst []int64
			for b.Loop() {
				dst = AppendInt64Zeros(dst[:0], n)
				Sink.Add(uint64(len(dst)))
			}
		})
	}
}

func BenchmarkAppendInt64Ones(b *testing.B) {
	for _, n := range []int{10, 100, 1000, 8 * 1024} {
		b.Run(fmt.Sprintf("%d_items", n), func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(n * 8))
			var dst []int64
			for b.Loop() {
				dst = AppendInt64Ones(dst[:0], n)
				Sink.Add(uint64(len(dst)))
			}
		})
	}
}

func BenchmarkAppendFloat64Zeros(b *testing.B) {
	for _, n := range []int{10, 100, 1000, 8 * 1024} {
		b.Run(fmt.Sprintf("%d_items", n), func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(n * 8))
			var dst []float64
			for b.Loop() {
				dst = AppendFloat64Zeros(dst[:0], n)
				Sink.Add(uint64(len(dst)))
			}
		})
	}
}

func BenchmarkAppendFloat64Ones(b *testing.B) {
	for _, n := range []int{10, 100, 1000, 8 * 1024} {
		b.Run(fmt.Sprintf("%d_items", n), func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(int64(n * 8))
			var dst []float64
			for b.Loop() {
				dst = AppendFloat64Ones(dst[:0], n)
				Sink.Add(uint64(len(dst)))
			}
		})
	}
}

func BenchmarkIsInt64Zeros(b *testing.B) {
	for _, n := range []int{10, 100, 1000, 8 * 1024} {
		b.Run(fmt.Sprintf("%d_items", n), func(b *testing.B) {
			a := make([]int64, n)
			b.ReportAllocs()
			b.SetBytes(int64(n * 8))
			for b.Loop() {
				if !IsInt64Zeros(a) {
					b.Fatal("unexpected false")
				}
			}
		})
	}
}
