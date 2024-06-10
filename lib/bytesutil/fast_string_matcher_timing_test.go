package bytesutil

import (
	"strings"
	"testing"
)

func BenchmarkFastStringMatcher(b *testing.B) {
	for _, s := range []string{"", "foo", "foo-bar-baz", "http_requests_total"} {
		b.Run(s, func(b *testing.B) {
			benchmarkFastStringMatcher(b, s)
		})
	}
}

func benchmarkFastStringMatcher(b *testing.B, s string) {
	fsm := NewFastStringMatcher(func(s string) bool {
		return strings.HasPrefix(s, "foo")
	})
	b.ReportAllocs()
	b.SetBytes(1)
	b.RunParallel(func(pb *testing.PB) {
		n := uint64(0)
		for pb.Next() {
			v := fsm.Match(s)
			if v {
				n++
			}
		}
		GlobalSink.Add(n)
	})
}
