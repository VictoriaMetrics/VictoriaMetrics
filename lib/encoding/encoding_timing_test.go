package encoding

import (
	"fmt"
	"math/rand"
	"sync/atomic"
	"testing"
)

func BenchmarkMarshalGaugeArray(b *testing.B) {
	b.ReportAllocs()
	b.SetBytes(int64(len(benchGaugeArray)))
	b.RunParallel(func(pb *testing.PB) {
		var dst []byte
		var mt MarshalType
		for pb.Next() {
			dst, mt, _ = marshalInt64Array(dst[:0], benchGaugeArray, 4)
			if mt != MarshalTypeZSTDNearestDelta {
				panic(fmt.Errorf("unexpected marshal type; got %d; expecting %d", mt, MarshalTypeZSTDNearestDelta))
			}
			atomic.AddUint64(&Sink, uint64(len(dst)))
		}
	})
}

var Sink uint64

func BenchmarkUnmarshalGaugeArray(b *testing.B) {
	b.ReportAllocs()
	b.SetBytes(int64(len(benchGaugeArray)))
	b.RunParallel(func(pb *testing.PB) {
		var dst []int64
		var err error
		for pb.Next() {
			dst, err = unmarshalInt64Array(dst[:0], benchMarshaledGaugeArray, MarshalTypeZSTDNearestDelta, benchGaugeArray[0], len(benchGaugeArray))
			if err != nil {
				panic(fmt.Errorf("cannot unmarshal gauge array: %w", err))
			}
			atomic.AddUint64(&Sink, uint64(len(dst)))
		}
	})
}

var benchGaugeArray = func() []int64 {
	a := make([]int64, 8*1024)
	v := int64(0)
	for i := 0; i < len(a); i++ {
		v += int64(rand.NormFloat64() * 100)
		a[i] = v
	}
	return a
}()

var benchMarshaledGaugeArray = func() []byte {
	b, _, _ := marshalInt64Array(nil, benchGaugeArray, 4)
	return b
}()

func BenchmarkMarshalDeltaConstArray(b *testing.B) {
	b.ReportAllocs()
	b.SetBytes(int64(len(benchDeltaConstArray)))
	b.RunParallel(func(pb *testing.PB) {
		var dst []byte
		var mt MarshalType
		for pb.Next() {
			dst, mt, _ = marshalInt64Array(dst[:0], benchDeltaConstArray, 4)
			if mt != MarshalTypeDeltaConst {
				panic(fmt.Errorf("unexpected marshal type; got %d; expecting %d", mt, MarshalTypeDeltaConst))
			}
			atomic.AddUint64(&Sink, uint64(len(dst)))
		}
	})
}

func BenchmarkUnmarshalDeltaConstArray(b *testing.B) {
	b.ReportAllocs()
	b.SetBytes(int64(len(benchDeltaConstArray)))
	b.RunParallel(func(pb *testing.PB) {
		var dst []int64
		var err error
		for pb.Next() {
			dst, err = unmarshalInt64Array(dst[:0], benchMarshaledDeltaConstArray, MarshalTypeDeltaConst, benchDeltaConstArray[0], len(benchDeltaConstArray))
			if err != nil {
				panic(fmt.Errorf("cannot unmarshal delta const array: %w", err))
			}
			atomic.AddUint64(&Sink, uint64(len(dst)))
		}
	})
}

var benchDeltaConstArray = func() []int64 {
	a := make([]int64, 8*1024)
	v := int64(0)
	for i := 0; i < len(a); i++ {
		v += 12345
		a[i] = v
	}
	return a
}()

var benchMarshaledDeltaConstArray = func() []byte {
	b, _, _ := marshalInt64Array(nil, benchDeltaConstArray, 4)
	return b
}()

func BenchmarkMarshalConstArray(b *testing.B) {
	b.ReportAllocs()
	b.SetBytes(int64(len(benchConstArray)))
	b.RunParallel(func(pb *testing.PB) {
		var dst []byte
		var mt MarshalType
		for pb.Next() {
			dst, mt, _ = marshalInt64Array(dst[:0], benchConstArray, 4)
			if mt != MarshalTypeConst {
				panic(fmt.Errorf("unexpected marshal type; got %d; expecting %d", mt, MarshalTypeConst))
			}
			atomic.AddUint64(&Sink, uint64(len(dst)))
		}
	})
}

func BenchmarkUnmarshalConstArray(b *testing.B) {
	b.ReportAllocs()
	b.SetBytes(int64(len(benchConstArray)))
	b.RunParallel(func(pb *testing.PB) {
		var dst []int64
		var err error
		for pb.Next() {
			dst, err = unmarshalInt64Array(dst[:0], benchMarshaledConstArray, MarshalTypeConst, benchConstArray[0], len(benchConstArray))
			if err != nil {
				panic(fmt.Errorf("cannot unmarshal const array: %w", err))
			}
			atomic.AddUint64(&Sink, uint64(len(dst)))
		}
	})
}

var benchConstArray = func() []int64 {
	a := make([]int64, 8*1024)
	for i := 0; i < len(a); i++ {
		a[i] = 1234567890
	}
	return a
}()

var benchMarshaledConstArray = func() []byte {
	b, _, _ := marshalInt64Array(nil, benchConstArray, 4)
	return b
}()

func BenchmarkMarshalZeroConstArray(b *testing.B) {
	b.ReportAllocs()
	b.SetBytes(int64(len(benchZeroConstArray)))
	b.RunParallel(func(pb *testing.PB) {
		var dst []byte
		var mt MarshalType
		for pb.Next() {
			dst, mt, _ = marshalInt64Array(dst[:0], benchZeroConstArray, 4)
			if mt != MarshalTypeConst {
				panic(fmt.Errorf("unexpected marshal type; got %d; expecting %d", mt, MarshalTypeConst))
			}
			atomic.AddUint64(&Sink, uint64(len(dst)))
		}
	})
}

func BenchmarkUnmarshalZeroConstArray(b *testing.B) {
	b.ReportAllocs()
	b.SetBytes(int64(len(benchZeroConstArray)))
	b.RunParallel(func(pb *testing.PB) {
		var dst []int64
		var err error
		for pb.Next() {
			dst, err = unmarshalInt64Array(dst[:0], benchMarshaledZeroConstArray, MarshalTypeConst, benchZeroConstArray[0], len(benchZeroConstArray))
			if err != nil {
				panic(fmt.Errorf("cannot unmarshal zero const array: %w", err))
			}
			atomic.AddUint64(&Sink, uint64(len(dst)))
		}
	})
}

var benchZeroConstArray = make([]int64, 8*1024)

var benchMarshaledZeroConstArray = func() []byte {
	b, _, _ := marshalInt64Array(nil, benchZeroConstArray, 4)
	return b
}()

func BenchmarkMarshalInt64Array(b *testing.B) {
	b.ReportAllocs()
	b.SetBytes(int64(len(benchInt64Array)))
	b.RunParallel(func(pb *testing.PB) {
		var dst []byte
		var mt MarshalType
		for pb.Next() {
			dst, mt, _ = marshalInt64Array(dst[:0], benchInt64Array, 4)
			if mt != benchMarshalType {
				panic(fmt.Errorf("unexpected marshal type; got %d; expecting %d", mt, benchMarshalType))
			}
			atomic.AddUint64(&Sink, uint64(len(dst)))
		}
	})
}

func BenchmarkUnmarshalInt64Array(b *testing.B) {
	b.ReportAllocs()
	b.SetBytes(int64(len(benchInt64Array)))
	b.RunParallel(func(pb *testing.PB) {
		var dst []int64
		var err error
		for pb.Next() {
			dst, err = unmarshalInt64Array(dst[:0], benchMarshaledInt64Array, benchMarshalType, benchInt64Array[0], len(benchInt64Array))
			if err != nil {
				panic(fmt.Errorf("cannot unmarshal int64 array: %w", err))
			}
			atomic.AddUint64(&Sink, uint64(len(dst)))
		}
	})
}

var benchMarshaledInt64Array = func() []byte {
	b, _, _ := marshalInt64Array(nil, benchInt64Array, 4)
	return b
}()

var benchMarshalType = func() MarshalType {
	_, mt, _ := marshalInt64Array(nil, benchInt64Array, 4)
	return mt
}()

var benchInt64Array = func() []int64 {
	var a []int64
	var v int64
	for i := 0; i < 8*1024; i++ {
		v += 30e3 + int64(rand.NormFloat64()*1e3)
		a = append(a, v)
	}
	return a
}()
