package remotewrite

import (
	"fmt"
	"testing"

	"github.com/golang/snappy"
	"github.com/klauspost/compress/s2"
)

func BenchmarkCompressWriteRequestSnappy(b *testing.B) {
	b.Run("snappy", func(b *testing.B) {
		benchmarkCompressWriteRequest(b, snappy.Encode)
	})
	b.Run("s2", func(b *testing.B) {
		benchmarkCompressWriteRequest(b, s2.EncodeSnappy)
	})
}

func benchmarkCompressWriteRequest(b *testing.B, compressFunc func(dst, src []byte) []byte) {
	for _, rowsCount := range []int{1, 10, 100, 1e3, 1e4} {
		b.Run(fmt.Sprintf("rows_%d", rowsCount), func(b *testing.B) {
			wr := newTestWriteRequest(rowsCount, 10)
			data := wr.MarshalProtobuf(nil)
			b.ReportAllocs()
			b.SetBytes(int64(rowsCount))
			b.RunParallel(func(pb *testing.PB) {
				var zb []byte
				for pb.Next() {
					zb = compressFunc(zb[:cap(zb)], data)
				}
			})
		})
	}
}
