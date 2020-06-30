package encoding

import (
	"fmt"
	"sync/atomic"
	"testing"
)

func BenchmarkMarshalUint64(b *testing.B) {
	b.ReportAllocs()
	b.SetBytes(1)
	b.RunParallel(func(pb *testing.PB) {
		var dst []byte
		var sink uint64
		for pb.Next() {
			dst = MarshalUint64(dst[:0], sink)
			sink += uint64(len(dst))
		}
		atomic.AddUint64(&Sink, sink)
	})
}

func BenchmarkUnmarshalUint64(b *testing.B) {
	b.ReportAllocs()
	b.SetBytes(1)
	b.RunParallel(func(pb *testing.PB) {
		var sink uint64
		for pb.Next() {
			v := UnmarshalUint64(testMarshaledUint64Data)
			sink += v
		}
		atomic.AddUint64(&Sink, sink)
	})
}

func BenchmarkMarshalInt64(b *testing.B) {
	b.ReportAllocs()
	b.SetBytes(1)
	b.RunParallel(func(pb *testing.PB) {
		var dst []byte
		var sink uint64
		for pb.Next() {
			dst = MarshalInt64(dst[:0], int64(sink))
			sink += uint64(len(dst))
		}
		atomic.AddUint64(&Sink, sink)
	})
}

func BenchmarkUnmarshalInt64(b *testing.B) {
	b.ReportAllocs()
	b.SetBytes(1)
	b.RunParallel(func(pb *testing.PB) {
		var sink uint64
		for pb.Next() {
			v := UnmarshalInt64(testMarshaledInt64Data)
			sink += uint64(v)
		}
		atomic.AddUint64(&Sink, sink)
	})
}

func BenchmarkMarshalVarInt64s(b *testing.B) {
	b.Run("up-to-(1<<6)-1", func(b *testing.B) {
		benchmarkMarshalVarInt64s(b, (1<<6)-1)
	})
	b.Run("up-to-(1<<13)-1", func(b *testing.B) {
		benchmarkMarshalVarInt64s(b, (1<<13)-1)
	})
	b.Run("up-to-(1<<27)-1", func(b *testing.B) {
		benchmarkMarshalVarInt64s(b, (1<<27)-1)
	})
	b.Run("up-to-(1<<63)-1", func(b *testing.B) {
		benchmarkMarshalVarInt64s(b, (1<<63)-1)
	})
}

func benchmarkMarshalVarInt64s(b *testing.B, maxValue int64) {
	const numsCount = 8000
	var data []int64
	n := maxValue
	for i := 0; i < numsCount; i++ {
		if n <= 0 {
			n = maxValue
		}
		data = append(data, n)
		n--
	}
	b.ResetTimer()
	b.ReportAllocs()
	b.SetBytes(numsCount)
	b.RunParallel(func(pb *testing.PB) {
		var sink uint64
		var dst []byte
		for pb.Next() {
			dst = MarshalVarInt64s(dst[:0], data)
			sink += uint64(len(dst))
		}
		atomic.AddUint64(&Sink, sink)
	})
}

func BenchmarkUnmarshalVarInt64s(b *testing.B) {
	b.Run("up-to-(1<<6)-1", func(b *testing.B) {
		benchmarkUnmarshalVarInt64s(b, (1<<6)-1)
	})
	b.Run("up-to-(1<<13)-1", func(b *testing.B) {
		benchmarkUnmarshalVarInt64s(b, (1<<13)-1)
	})
	b.Run("up-to-(1<<27)-1", func(b *testing.B) {
		benchmarkUnmarshalVarInt64s(b, (1<<27)-1)
	})
	b.Run("up-to-(1<<63)-1", func(b *testing.B) {
		benchmarkUnmarshalVarInt64s(b, (1<<63)-1)
	})
}

func benchmarkUnmarshalVarInt64s(b *testing.B, maxValue int64) {
	const numsCount = 8000
	var data []byte
	n := maxValue
	for i := 0; i < numsCount; i++ {
		if n <= 0 {
			n = maxValue
		}
		data = MarshalVarInt64(data, n)
		n--
	}
	b.ResetTimer()
	b.ReportAllocs()
	b.SetBytes(numsCount)
	b.RunParallel(func(pb *testing.PB) {
		var sink uint64
		dst := make([]int64, numsCount)
		for pb.Next() {
			tail, err := UnmarshalVarInt64s(dst, data)
			if err != nil {
				panic(fmt.Errorf("unexpected error: %w", err))
			}
			if len(tail) > 0 {
				panic(fmt.Errorf("unexpected non-empty tail with len=%d: %X", len(tail), tail))
			}
			sink += uint64(len(dst))
		}
		atomic.AddUint64(&Sink, sink)
	})
}

var testMarshaledInt64Data = MarshalInt64(nil, 1234567890)
var testMarshaledUint64Data = MarshalUint64(nil, 1234567890)
