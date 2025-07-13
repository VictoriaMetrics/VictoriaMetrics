package timeutil

import (
	"fmt"
	"sync/atomic"
	"testing"
)

func BenchmarkTryParseUnixTimestamp(b *testing.B) {
	b.Run("seconds", func(b *testing.B) {
		a := []string{
			"1234567890",
			"1234567891",
			"1234567892",
			"1234567893",
			"1234567894",
			"1234567895",
		}
		benchmarkTryParseUnixTimestamp(b, a)
	})
	b.Run("milliseconds", func(b *testing.B) {
		a := []string{
			"1234567890123",
			"1234567891123",
			"1234567892123",
			"1234567893123",
			"1234567894123",
			"1234567895123",
		}
		benchmarkTryParseUnixTimestamp(b, a)
	})
	b.Run("microseconds", func(b *testing.B) {
		a := []string{
			"1234567890123456",
			"1234567891123456",
			"1234567892123456",
			"1234567893123456",
			"1234567894123456",
			"1234567895123456",
		}
		benchmarkTryParseUnixTimestamp(b, a)
	})
	b.Run("nanoseconds", func(b *testing.B) {
		a := []string{
			"1234567890123456789",
			"1234567891123456789",
			"1234567892123456789",
			"1234567893123456789",
			"1234567894123456789",
			"1234567895123456789",
		}
		benchmarkTryParseUnixTimestamp(b, a)
	})

	b.Run("scientific-seconds", func(b *testing.B) {
		a := []string{
			"1.234567890e9",
			"1.234567891e9",
			"1.234567892e9",
			"1.234567893e9",
			"1.234567894e9",
			"1.234567895e9",
		}
		benchmarkTryParseUnixTimestamp(b, a)
	})
	b.Run("scientific-milliseconds", func(b *testing.B) {
		a := []string{
			"1.234567890123e12",
			"1.234567891123e12",
			"1.234567892123e12",
			"1.234567893123e12",
			"1.234567894123e12",
			"1.234567895123e12",
		}
		benchmarkTryParseUnixTimestamp(b, a)
	})
	b.Run("scientific-microseconds", func(b *testing.B) {
		a := []string{
			"1.234567890123456e15",
			"1.234567891123456e15",
			"1.234567892123456e15",
			"1.234567893123456e15",
			"1.234567894123456e15",
			"1.234567895123456e15",
		}
		benchmarkTryParseUnixTimestamp(b, a)
	})
	b.Run("scientific-nanoseconds", func(b *testing.B) {
		a := []string{
			"1.234567890123456789e18",
			"1.234567891123456789e18",
			"1.234567892123456789e18",
			"1.234567893123456789e18",
			"1.234567894123456789e18",
			"1.234567895123456789e18",
		}
		benchmarkTryParseUnixTimestamp(b, a)
	})

	b.Run("fractional-milliseconds", func(b *testing.B) {
		a := []string{
			"1234567890.123",
			"1234567891.123",
			"1234567892.123",
			"1234567893.123",
			"1234567894.123",
			"1234567895.123",
		}
		benchmarkTryParseUnixTimestamp(b, a)
	})
	b.Run("fractional-microseconds", func(b *testing.B) {
		a := []string{
			"1234567890.123456",
			"1234567891.123456",
			"1234567892.123456",
			"1234567893.123456",
			"1234567894.123456",
			"1234567895.123456",
		}
		benchmarkTryParseUnixTimestamp(b, a)
	})
	b.Run("fractional-nanoseconds", func(b *testing.B) {
		a := []string{
			"1234567890.123456789",
			"1234567891.123456789",
			"1234567892.123456789",
			"1234567893.123456789",
			"1234567894.123456789",
			"1234567895.123456789",
		}
		benchmarkTryParseUnixTimestamp(b, a)
	})
}

func benchmarkTryParseUnixTimestamp(b *testing.B, a []string) {
	b.SetBytes(int64(len(a)))
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		nSum := int64(0)
		for pb.Next() {
			for _, s := range a {
				n, ok := TryParseUnixTimestamp(s)
				if !ok {
					panic(fmt.Errorf("cannot parse timestamp %q", s))
				}
				nSum += n
			}
		}
		GlobalSink.Add(uint64(nSum))
	})
}

var GlobalSink atomic.Uint64
