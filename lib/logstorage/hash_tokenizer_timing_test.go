package logstorage

import (
	"strings"
	"testing"
)

/*
go test -benchmem -v -run=^$ -bench ^BenchmarkTokenizeHashes$ github.com/VictoriaMetrics/VictoriaMetrics/lib/logstorage

goos: linux
goarch: amd64
pkg: github.com/VictoriaMetrics/VictoriaMetrics/lib/logstorage
cpu: Intel(R) Xeon(R) Platinum 8260 CPU @ 2.40GHz
BenchmarkTokenizeHashes
BenchmarkTokenizeHashes-8         695058              1627 ns/op        2057.93 MB/s           0 B/op          0 allocs/op

goos: linux  performance improve 3.87%
goarch: amd64
pkg: github.com/VictoriaMetrics/VictoriaMetrics/lib/logstorage
cpu: Intel(R) Xeon(R) Platinum 8260 CPU @ 2.40GHz
BenchmarkTokenizeHashes
BenchmarkTokenizeHashes-8         661975              1564 ns/op        2141.33 MB/s           0 B/op          0 allocs/op
*/
func BenchmarkTokenizeHashes(b *testing.B) {
	a := strings.Split(benchLogs, "\n")

	b.ReportAllocs()
	b.SetBytes(int64(len(benchLogs)))
	b.RunParallel(func(pb *testing.PB) {
		var hashes []uint64
		for pb.Next() {
			hashes = tokenizeHashes(hashes[:0], a)
		}
	})
}

/*
go test -benchmem -v -run=^$ -bench ^BenchmarkTokenizeHashesUnicode$ github.com/VictoriaMetrics/VictoriaMetrics/lib/logstorage

goos: linux
goarch: amd64
pkg: github.com/VictoriaMetrics/VictoriaMetrics/lib/logstorage
cpu: Intel(R) Xeon(R) Platinum 8260 CPU @ 2.40GHz
BenchmarkTokenizeHashesUnicode
BenchmarkTokenizeHashesUnicode-8          215118              4855 ns/op         721.72 MB/s           0 B/op          0 allocs/op

goos: linux  performance improve 12.17%
goarch: amd64
pkg: github.com/VictoriaMetrics/VictoriaMetrics/lib/logstorage
cpu: Intel(R) Xeon(R) Platinum 8260 CPU @ 2.40GHz
BenchmarkTokenizeHashesUnicode
BenchmarkTokenizeHashesUnicode-8          274152              4264 ns/op         821.78 MB/s           0 B/op          0 allocs/op
*/
func BenchmarkTokenizeHashesUnicode(b *testing.B) {
	a := strings.Split(benchLogs, "\n")
	var totalLen int
	for i := range a {
		a[i] += "中文"
		totalLen += len(a[i])
	}
	b.ReportAllocs()
	b.SetBytes(int64(totalLen))
	b.RunParallel(func(pb *testing.PB) {
		var hashes []uint64
		for pb.Next() {
			hashes = tokenizeHashes(hashes[:0], a)
		}
	})
}
