package promutil

import (
	"sync/atomic"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
)

func BenchmarkLabelsCompressorCompress(b *testing.B) {
	series := newTestSeries(100, 10)
	run := func(b *testing.B, withRotate bool) {
		var lc LabelsCompressor
		for _, labels := range series {
			lc.Compress(nil, labels)
		}
		var rotations atomic.Int64
		if withRotate {
			done := make(chan struct{})
			go func() {
				for {
					select {
					case <-done:
						return
					default:
						lc.rotate()
						rotations.Add(1)
					}
				}
			}()
			defer close(done)
		}
		rotations.Store(0)
		b.ResetTimer()
		b.ReportAllocs()
		b.SetBytes(int64(len(series)))
		b.RunParallel(func(pb *testing.PB) {
			var dst []byte
			for pb.Next() {
				dst = dst[:0]
				for _, labels := range series {
					dst = lc.Compress(dst, labels)
				}
				Sink.Add(uint64(len(dst)))
			}
		})
		if withRotate {
			b.ReportMetric(float64(rotations.Load())/float64(b.N), "rotations/op")
		}
	}
	b.Run("no_rotate", func(b *testing.B) { run(b, false) })
	b.Run("with_rotate", func(b *testing.B) { run(b, true) })
}

func BenchmarkLabelsCompressorDecompress(b *testing.B) {
	series := newTestSeries(100, 10)
	run := func(b *testing.B, withRotate bool) {
		var lc LabelsCompressor
		datas := make([][]byte, len(series))
		var buf []byte
		for i, labels := range series {
			bufLen := len(buf)
			buf = lc.Compress(buf, labels)
			datas[i] = buf[bufLen:]
		}
		var rotations atomic.Int64
		if withRotate {
			done := make(chan struct{})
			go func() {
				for {
					select {
					case <-done:
						return
					default:
						lc.rotate()
						rotations.Add(1)
					}
				}
			}()
			defer close(done)
		}
		rotations.Store(0)
		b.ResetTimer()
		b.ReportAllocs()
		b.SetBytes(int64(len(series)))
		b.RunParallel(func(pb *testing.PB) {
			var labels []prompb.Label
			for pb.Next() {
				for _, data := range datas {
					labels = lc.Decompress(labels[:0], data)
				}
				Sink.Add(uint64(len(labels)))
			}
		})
		if withRotate {
			b.ReportMetric(float64(rotations.Load())/float64(b.N), "rotations/op")
		}
	}
	b.Run("no_rotate", func(b *testing.B) { run(b, false) })
	b.Run("with_rotate", func(b *testing.B) { run(b, true) })
}

var Sink atomic.Uint64
