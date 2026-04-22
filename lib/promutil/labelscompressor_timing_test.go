package promutil

import (
	"sync/atomic"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
)

func BenchmarkLabelsCompressorCompress(b *testing.B) {
	series := newTestSeries(100, 10)
	run := func(b *testing.B, withCleanup bool) {
		var lc LabelsCompressor
		var buf []byte
		liveKeys := make([]string, len(series))
		for i, labels := range series {
			bufLen := len(buf)
			buf = lc.Compress(buf, labels)
			liveKeys[i] = string(buf[bufLen:])
		}
		var cleanups atomic.Int64
		if withCleanup {
			done := make(chan struct{})
			go func() {
				for {
					select {
					case <-done:
						return
					default:
						lc.Cleanup(liveKeys)
						cleanups.Add(1)
					}
				}
			}()
			defer close(done)
		}
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
		if withCleanup {
			b.ReportMetric(float64(cleanups.Load())/float64(b.N), "cleanups/op")
		}
	}
	b.Run("no_cleanup", func(b *testing.B) { run(b, false) })
	b.Run("with_cleanup", func(b *testing.B) { run(b, true) })
}

func BenchmarkLabelsCompressorDecompress(b *testing.B) {
	series := newTestSeries(100, 10)
	run := func(b *testing.B, withCleanup bool) {
		var lc LabelsCompressor
		datas := make([][]byte, len(series))
		liveKeys := make([]string, len(series))
		var buf []byte
		for i, labels := range series {
			bufLen := len(buf)
			buf = lc.Compress(buf, labels)
			datas[i] = buf[bufLen:]
			liveKeys[i] = string(buf[bufLen:])
		}
		var cleanups atomic.Int64
		if withCleanup {
			done := make(chan struct{})
			go func() {
				for {
					select {
					case <-done:
						return
					default:
						lc.Cleanup(liveKeys)
						cleanups.Add(1)
					}
				}
			}()
			defer close(done)
		}
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
		if withCleanup {
			b.ReportMetric(float64(cleanups.Load())/float64(b.N), "cleanups/op")
		}
	}
	b.Run("no_cleanup", func(b *testing.B) { run(b, false) })
	b.Run("with_cleanup", func(b *testing.B) { run(b, true) })
}

var Sink atomic.Uint64
