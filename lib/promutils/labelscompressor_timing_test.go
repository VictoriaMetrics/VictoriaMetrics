package promutils

import (
	"sync/atomic"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
)

func BenchmarkLabelsCompressorCompress(b *testing.B) {
	var lc LabelsCompressor
	series := newTestSeries(100, 10)

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

func BenchmarkLabelsCompressorDecompress(b *testing.B) {
	var lc LabelsCompressor
	series := newTestSeries(100, 10)
	datas := make([][]byte, len(series))
	var dst []byte
	for i, labels := range series {
		dstLen := len(dst)
		dst = lc.Compress(dst, labels)
		datas[i] = dst[dstLen:]
	}

	b.ReportAllocs()
	b.SetBytes(int64(len(series)))

	b.RunParallel(func(pb *testing.PB) {
		var labels []prompbmarshal.Label
		for pb.Next() {
			for _, data := range datas {
				labels = lc.Decompress(labels[:0], data)
			}
			Sink.Add(uint64(len(labels)))
		}
	})
}

var Sink atomic.Uint64
