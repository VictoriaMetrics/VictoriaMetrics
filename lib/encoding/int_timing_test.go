package encoding

import (
	"fmt"
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
		Sink.Add(sink)
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
		Sink.Add(sink)
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
		Sink.Add(sink)
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
		Sink.Add(sink)
	})
}

func BenchmarkMarshalVarUint64s(b *testing.B) {
	b.Run("up-to-(1<<7)-1", func(b *testing.B) {
		benchmarkMarshalVarUint64s(b, (1<<7)-1)
	})
	b.Run("up-to-(1<<14)-1", func(b *testing.B) {
		benchmarkMarshalVarUint64s(b, (1<<14)-1)
	})
	b.Run("up-to-(1<<28)-1", func(b *testing.B) {
		benchmarkMarshalVarUint64s(b, (1<<28)-1)
	})
	b.Run("up-to-(1<<64)-1", func(b *testing.B) {
		benchmarkMarshalVarUint64s(b, (1<<64)-1)
	})
}

func benchmarkMarshalVarUint64s(b *testing.B, maxValue uint64) {
	const numsCount = 8000
	var data []uint64
	n := maxValue
	for i := 0; i < numsCount; i++ {
		if n > maxValue {
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
			dst = MarshalVarUint64s(dst[:0], data)
			sink += uint64(len(dst))
		}
		Sink.Add(sink)
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
		if n < -maxValue {
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
		Sink.Add(sink)
	})
}

func BenchmarkUnmarshalVarUint64(b *testing.B) {
	b.Run("up-to-(1<<7)-1", func(b *testing.B) {
		benchmarkUnmarshalVarUint64(b, (1<<7)-1)
	})
	b.Run("up-to-(1<<14)-1", func(b *testing.B) {
		benchmarkUnmarshalVarUint64(b, (1<<14)-1)
	})
	b.Run("up-to-(1<<28)-1", func(b *testing.B) {
		benchmarkUnmarshalVarUint64(b, (1<<28)-1)
	})
	b.Run("up-to-(1<<64)-1", func(b *testing.B) {
		benchmarkUnmarshalVarUint64(b, (1<<64)-1)
	})
}

func benchmarkUnmarshalVarUint64(b *testing.B, maxValue uint64) {
	const numsCount = 8000
	var data []byte
	n := maxValue
	for i := 0; i < numsCount; i++ {
		if n > maxValue {
			n = maxValue
		}
		data = MarshalVarUint64(data, n)
		n--
	}
	b.ResetTimer()
	b.ReportAllocs()
	b.SetBytes(numsCount)
	b.RunParallel(func(pb *testing.PB) {
		var sink uint64
		for pb.Next() {
			src := data
			for len(src) > 0 {
				n, nSize := UnmarshalVarUint64(src)
				if nSize <= 0 {
					panic(fmt.Errorf("unexpected error"))
				}
				src = src[nSize:]
				sink += n
			}
		}
		Sink.Add(sink)
	})
}

func BenchmarkUnmarshalVarUint64s(b *testing.B) {
	b.Run("up-to-(1<<7)-1", func(b *testing.B) {
		benchmarkUnmarshalVarUint64s(b, (1<<7)-1)
	})
	b.Run("up-to-(1<<14)-1", func(b *testing.B) {
		benchmarkUnmarshalVarUint64s(b, (1<<14)-1)
	})
	b.Run("up-to-(1<<28)-1", func(b *testing.B) {
		benchmarkUnmarshalVarUint64s(b, (1<<28)-1)
	})
	b.Run("up-to-(1<<64)-1", func(b *testing.B) {
		benchmarkUnmarshalVarUint64s(b, (1<<64)-1)
	})
}

func benchmarkUnmarshalVarUint64s(b *testing.B, maxValue uint64) {
	const numsCount = 8000
	var data []byte
	n := maxValue
	for i := 0; i < numsCount; i++ {
		if n > maxValue {
			n = maxValue
		}
		data = MarshalVarUint64(data, n)
		n--
	}
	b.ResetTimer()
	b.ReportAllocs()
	b.SetBytes(numsCount)
	b.RunParallel(func(pb *testing.PB) {
		var sink uint64
		dst := make([]uint64, numsCount)
		for pb.Next() {
			tail, err := UnmarshalVarUint64s(dst, data)
			if err != nil {
				panic(fmt.Errorf("unexpected error: %w", err))
			}
			if len(tail) > 0 {
				panic(fmt.Errorf("unexpected non-empty tail with len=%d: %X", len(tail), tail))
			}
			sink += uint64(len(dst))
		}
		Sink.Add(sink)
	})
}

func BenchmarkUnmarshalVarInt64(b *testing.B) {
	b.Run("up-to-(1<<6)-1", func(b *testing.B) {
		benchmarkUnmarshalVarInt64(b, (1<<6)-1)
	})
	b.Run("up-to-(1<<13)-1", func(b *testing.B) {
		benchmarkUnmarshalVarInt64(b, (1<<13)-1)
	})
	b.Run("up-to-(1<<27)-1", func(b *testing.B) {
		benchmarkUnmarshalVarInt64(b, (1<<27)-1)
	})
	b.Run("up-to-(1<<63)-1", func(b *testing.B) {
		benchmarkUnmarshalVarInt64(b, (1<<63)-1)
	})
}

func benchmarkUnmarshalVarInt64(b *testing.B, maxValue int64) {
	const numsCount = 8000
	var data []byte
	n := maxValue
	for i := 0; i < numsCount; i++ {
		if n < -maxValue {
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
		for pb.Next() {
			src := data
			for len(src) > 0 {
				n, nSize := UnmarshalVarInt64(src)
				if nSize <= 0 {
					panic(fmt.Errorf("unexpected error"))
				}
				src = src[nSize:]
				sink += uint64(n)
			}
		}
		Sink.Add(sink)
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
		if n < -maxValue {
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
		Sink.Add(sink)
	})
}

func BenchmarkMarshalVarUint64(b *testing.B) {
	b.Run("small-ints", func(b *testing.B) {
		benchmarkMarshalVarUint64(b, []uint64{1, 2, 3, 4, 5, 67, 127})
	})
	b.Run("big-ints", func(b *testing.B) {
		benchmarkMarshalVarUint64(b, []uint64{12355, 89832432, 8989843, 8989989, 883443, 9891233, 8232434342})
	})
}

func benchmarkMarshalVarUint64(b *testing.B, a []uint64) {
	b.ReportAllocs()
	b.SetBytes(int64(len(a)))
	b.RunParallel(func(pb *testing.PB) {
		var buf []byte
		var sink uint64
		for pb.Next() {
			buf = buf[:0]
			for _, n := range a {
				buf = MarshalVarUint64(buf, n)
			}
			sink += uint64(len(buf))
		}
		Sink.Add(sink)
	})
}

func BenchmarkMarshalVarInt64(b *testing.B) {
	b.Run("small-ints", func(b *testing.B) {
		benchmarkMarshalVarInt64(b, []int64{1, 2, 3, -4, 5, -60, 63})
	})
	b.Run("big-ints", func(b *testing.B) {
		benchmarkMarshalVarInt64(b, []int64{12355, -89832432, 8989843, -8989989, 883443, -9891233, 8232434342})
	})
}

func benchmarkMarshalVarInt64(b *testing.B, a []int64) {
	b.ReportAllocs()
	b.SetBytes(int64(len(a)))
	b.RunParallel(func(pb *testing.PB) {
		var buf []byte
		var sink uint64
		for pb.Next() {
			buf = buf[:0]
			for _, n := range a {
				buf = MarshalVarInt64(buf, n)
			}
			sink += uint64(len(buf))
		}
		Sink.Add(sink)
	})
}

var testMarshaledInt64Data = MarshalInt64(nil, 1234567890)
var testMarshaledUint64Data = MarshalUint64(nil, 1234567890)
