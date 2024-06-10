package bytesutil

import (
	"strings"
	"sync/atomic"
	"testing"
)

func BenchmarkFastStringTransformer(b *testing.B) {
	for _, s := range []string{"", "foo", "foo-bar-baz", "http_requests_total"} {
		b.Run(s, func(b *testing.B) {
			benchmarkFastStringTransformer(b, s)
		})
	}
}

func benchmarkFastStringTransformer(b *testing.B, s string) {
	fst := NewFastStringTransformer(strings.ToUpper)
	b.ReportAllocs()
	b.SetBytes(1)
	b.RunParallel(func(pb *testing.PB) {
		n := uint64(0)
		for pb.Next() {
			sTransformed := fst.Transform(s)
			n += uint64(len(sTransformed))
		}
		GlobalSink.Add(n)
	})
}

var GlobalSink atomic.Uint64
