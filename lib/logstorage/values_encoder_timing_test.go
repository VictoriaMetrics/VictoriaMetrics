package logstorage

import (
	"fmt"
	"sync/atomic"
	"testing"
)

func BenchmarkTryParseTimestampRFC3339Nano(b *testing.B) {
	a := []string{
		"2023-01-15T23:45:51Z",
		"2023-02-15T23:45:51.123Z",
		"2024-02-15T23:45:51.123456Z",
		"2025-02-15T22:45:51.123456789Z",
		"2023-02-15T22:45:51.000000000Z",
	}

	b.SetBytes(int64(len(a)))
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		nSum := int64(0)
		for pb.Next() {
			for _, s := range a {
				n, ok := TryParseTimestampRFC3339Nano(s)
				if !ok {
					panic(fmt.Errorf("cannot parse timestamp %q", s))
				}
				nSum += n
			}
		}
		GlobalSink.Add(uint64(nSum))
	})
}

func BenchmarkTryParseTimestampISO8601(b *testing.B) {
	a := []string{
		"2023-01-15T23:45:51.123Z",
		"2023-02-15T23:45:51.123Z",
		"2024-02-15T23:45:51.123Z",
		"2025-02-15T22:45:51.123Z",
		"2023-02-15T22:45:51.000Z",
	}

	b.SetBytes(int64(len(a)))
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		nSum := int64(0)
		for pb.Next() {
			for _, s := range a {
				n, ok := tryParseTimestampISO8601(s)
				if !ok {
					panic(fmt.Errorf("cannot parse timestamp %q", s))
				}
				nSum += n
			}
		}
		GlobalSink.Add(uint64(nSum))
	})
}

func BenchmarkTryParseIPv4(b *testing.B) {
	a := []string{
		"1.2.3.4",
		"127.0.0.1",
		"255.255.255.255",
		"192.43.234.22",
		"32.34.54.198",
	}

	b.SetBytes(int64(len(a)))
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		nSum := uint32(0)
		for pb.Next() {
			for _, s := range a {
				n, ok := tryParseIPv4(s)
				if !ok {
					panic(fmt.Errorf("cannot parse ipv4 %q", s))
				}
				nSum += n
			}
		}
		GlobalSink.Add(uint64(nSum))
	})
}

func BenchmarkTryParseUint64(b *testing.B) {
	a := []string{
		"1234",
		"483932",
		"28494",
		"90012",
		"889111",
	}

	b.SetBytes(int64(len(a)))
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		var nSum uint64
		for pb.Next() {
			for _, s := range a {
				n, ok := tryParseUint64(s)
				if !ok {
					panic(fmt.Errorf("cannot parse uint %q", s))
				}
				nSum += n
			}
		}
		GlobalSink.Add(nSum)
	})
}

func BenchmarkTryParseFloat64(b *testing.B) {
	a := []string{
		"1.234",
		"4.545",
		"456.5645",
		"-123.434",
		"434.322",
	}

	b.SetBytes(int64(len(a)))
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		var fSum float64
		for pb.Next() {
			for _, s := range a {
				f, ok := tryParseFloat64(s)
				if !ok {
					panic(fmt.Errorf("cannot parse float64 %q", s))
				}
				fSum += f
			}
		}
		GlobalSink.Add(uint64(fSum))
	})
}

func BenchmarkMarshalUint8String(b *testing.B) {
	b.SetBytes(256)
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		var buf []byte
		n := 0
		for pb.Next() {
			for i := 0; i < 256; i++ {
				buf = marshalUint8String(buf[:0], uint8(i))
				n += len(buf)
			}
		}
		GlobalSink.Add(uint64(n))
	})
}

var GlobalSink atomic.Uint64
