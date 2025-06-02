package logstorage

import (
	"strings"
	"testing"
)

// go test -benchmem -v -run=^$ -benchmem -bench ^BenchmarkTokenizeStrings$ github.com/VictoriaMetrics/VictoriaMetrics/lib/logstorage
/*
BenchmarkTokenizeStrings-8        285720              4444 ns/op         753.30 MB/s           0 B/op          0 allocs/op
*/
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
