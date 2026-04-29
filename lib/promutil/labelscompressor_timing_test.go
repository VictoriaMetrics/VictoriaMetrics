package promutil

import (
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
)

func BenchmarkLabelsCompressorCompressFastPath(b *testing.B) {
	lc := NewLabelsCompressor()
	series := newTestSeries(100, 10)

	var dst []byte
	for _, labels := range series {
		dst = dst[:0]
		dst = lc.Compress(dst, labels)
	}

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
}

func BenchmarkLabelsCompressorCompressSlowPath(b *testing.B) {
	series := newTestSeries(100, 10)

	b.ReportAllocs()
	b.SetBytes(int64(len(series)))

	for b.Loop() {
		var dst []byte
		lc := NewLabelsCompressor()
		dst = dst[:0]
		for _, labels := range series {
			dst = lc.Compress(dst, labels)
		}
		Sink.Add(uint64(len(dst)))
	}
}

func BenchmarkLabelsCompressorDecompress(b *testing.B) {
	f := func(b *testing.B, preload, postload int) {
		lc := NewLabelsCompressor()

		var labels []prompb.Label

		var preloadDst []byte
		for i := 0; i < preload; i++ {
			preloadDst = preloadDst[:0]
			labels = labels[:0]

			labels := []prompb.Label{
				{
					Name:  "instance00",
					Value: fmt.Sprintf("preload00%d", i),
				},
				{
					Name:  "job1111111",
					Value: fmt.Sprintf("preload11%d", i),
				},
			}
			lc.Decompress(labels, lc.Compress(preloadDst, labels))
		}

		series := newTestSeries(100, 10)
		datas := make([][]byte, len(series))
		var dst []byte
		for i, labels := range series {
			dstLen := len(dst)
			dst = lc.Compress(dst, labels)
			datas[i] = dst[dstLen:]
		}

		var postloadDst []byte
		for i := 0; i < postload; i++ {
			postloadDst = postloadDst[:0]
			labels = labels[:0]

			labels := []prompb.Label{
				{
					Name:  "instance22",
					Value: fmt.Sprintf("postload2%d", i),
				},
				{
					Name:  "job3333333",
					Value: fmt.Sprintf("postload3%d", i),
				},
			}
			lc.Decompress(labels, lc.Compress(postloadDst, labels))
		}

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
	}

	b.Run("Preload0", func(b *testing.B) {
		f(b, 0, 0)
	})

	b.Run("Preload50k", func(b *testing.B) {
		f(b, 25000, 25000)
	})

	b.Run("Preload100k", func(b *testing.B) {
		f(b, 50000, 50000)
	})

	b.Run("Preload200k", func(b *testing.B) {
		f(b, 100000, 100000)
	})

	b.Run("Preload300k", func(b *testing.B) {
		f(b, 150000, 150000)
	})
}

var Sink atomic.Uint64
