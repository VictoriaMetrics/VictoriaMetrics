package logstorage

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func BenchmarkTokenizeStrings(b *testing.B) {
	a := strings.Split(benchLogs, "\n")

	b.ReportAllocs()
	b.SetBytes(int64(len(benchLogs)))
	b.RunParallel(func(pb *testing.PB) {
		var tokens []string
		for pb.Next() {
			tokens = tokenizeStrings(tokens[:0], a)
		}
	})
}

func BenchmarkIsTokenChar(b *testing.B) {
	b.ReportAllocs()
	b.SetBytes(int64(len(benchLogs)))
	b.RunParallel(func(pb *testing.PB) {
		n := 0
		for pb.Next() {
			for i := range benchLogs {
				ch := benchLogs[i]
				if isTokenChar(ch) {
					n++
				}
			}
		}
		GlobalSink.Add(uint64(n))
	})
}

func BenchmarkIsTokenRune(b *testing.B) {
	b.Run("ascii", func(b *testing.B) {
		benchmarkIsTokenRune(b, benchLogs)
	})

	var buf []byte
	for i, ch := range benchLogs {
		if i%10 == 0 {
			ch += 1024
		}
		buf = utf8.AppendRune(buf, ch)
	}
	benchLogsUnicode := string(buf)

	b.Run("unicode", func(b *testing.B) {
		benchmarkIsTokenRune(b, benchLogsUnicode)
	})
}

func benchmarkIsTokenRune(b *testing.B, s string) {
	b.ReportAllocs()
	b.SetBytes(int64(len(s)))
	b.RunParallel(func(pb *testing.PB) {
		n := 0
		for pb.Next() {
			for _, ch := range s {
				if isTokenRune(ch) {
					n++
				}
			}
		}
		GlobalSink.Add(uint64(n))
	})
}
