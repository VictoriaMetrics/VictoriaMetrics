package encoding

import (
	"fmt"
	"sync/atomic"
	"testing"
)

func BenchmarkMarshalInt64NearestDelta(b *testing.B) {
	for _, precisionBits := range []uint8{4, 64} {
		b.Run(fmt.Sprintf("precisionBits_%d", precisionBits), func(b *testing.B) {
			benchmarkMarshalInt64NearestDelta(b, precisionBits)
		})
	}
}

func benchmarkMarshalInt64NearestDelta(b *testing.B, precisionBits uint8) {
	b.ReportAllocs()
	b.SetBytes(int64(len(benchInt64Array)))
	b.RunParallel(func(pb *testing.PB) {
		var dst []byte
		for pb.Next() {
			dst, _ = marshalInt64NearestDelta(dst[:0], benchInt64Array, precisionBits)
			atomic.AddUint64(&Sink, uint64(len(dst)))
		}
	})
}

func BenchmarkUnmarshalInt64NearestDelta(b *testing.B) {
	b.ReportAllocs()
	b.SetBytes(int64(len(benchInt64Array)))
	b.RunParallel(func(pb *testing.PB) {
		var dst []int64
		var err error
		for pb.Next() {
			dst, err = unmarshalInt64NearestDelta(dst[:0], benchInt64NearestDeltaData, 0, len(benchInt64Array))
			if err != nil {
				panic(fmt.Errorf("unexpected error: %w", err))
			}
			atomic.AddUint64(&Sink, uint64(len(dst)))
		}
	})
}

var benchInt64NearestDeltaData = func() []byte {
	data, _ := marshalInt64NearestDelta(nil, benchInt64Array, 4)
	return data
}()
