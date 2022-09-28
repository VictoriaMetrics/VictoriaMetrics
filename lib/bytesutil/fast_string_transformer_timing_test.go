package bytesutil

import (
	"strings"
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
		for pb.Next() {
			sTransformed := fst.Transform(s)
			GlobalSink += len(sTransformed)
		}
	})
}

var GlobalSink int
