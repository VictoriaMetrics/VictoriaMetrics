package stringsutil

import (
	"testing"
)

func BenchmarkLessNatural(b *testing.B) {
	b.Run("distinct_string_prefixes", func(b *testing.B) {
		benchmarkLessNatural(b, []string{
			"aaa", "bbb", "ccc", "ddd", "eee", "fff",
		})
	})
}

func benchmarkLessNatural(b *testing.B, a []string) {
	b.ReportAllocs()
	b.SetBytes(int64(len(a) - 1))
	b.RunParallel(func(pb *testing.PB) {
		n := uint64(0)
		for pb.Next() {
			for i := 1; i < len(a); i++ {
				if LessNatural(a[i-1], a[i]) {
					n++
				}
			}
		}
		GlobalSink.Add(n)
	})
}
