package logstorage

import (
	"fmt"
	"testing"
)

func BenchmarkTryParseTimestampISO8601(b *testing.B) {
	a := []string{
		"2023-01-15T23:45:51.123Z",
		"2023-02-15T23:45:51.123Z",
		"2023-02-15T23:45:51.123+01:00",
		"2023-02-15T22:45:51.123-10:30",
		"2023-02-15T22:45:51.000Z",
	}

	b.SetBytes(int64(len(a)))
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			for _, s := range a {
				_, ok := tryParseTimestampISO8601(s)
				if !ok {
					panic(fmt.Errorf("cannot parse timestamp %q", s))
				}
			}
		}
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
		for pb.Next() {
			for _, s := range a {
				_, ok := tryParseIPv4(s)
				if !ok {
					panic(fmt.Errorf("cannot parse ipv4 %q", s))
				}
			}
		}
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
		for pb.Next() {
			for _, s := range a {
				_, ok := tryParseUint64(s)
				if !ok {
					panic(fmt.Errorf("cannot parse uint %q", s))
				}
			}
		}
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
		for pb.Next() {
			for _, s := range a {
				_, ok := tryParseFloat64(s)
				if !ok {
					panic(fmt.Errorf("cannot parse float64 %q", s))
				}
			}
		}
	})
}
