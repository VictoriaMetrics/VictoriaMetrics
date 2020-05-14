package fasttime

import (
	"sync/atomic"
	"testing"
	"time"
)

func BenchmarkUnixTimestamp(b *testing.B) {
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		var ts uint64
		for pb.Next() {
			ts += UnixTimestamp()
		}
		atomic.StoreUint64(&Sink, ts)
	})
}

func BenchmarkTimeNowUnix(b *testing.B) {
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		var ts uint64
		for pb.Next() {
			ts += uint64(time.Now().Unix())
		}
		atomic.StoreUint64(&Sink, ts)
	})
}

// Sink should prevent from code elimination by optimizing compiler
var Sink uint64
