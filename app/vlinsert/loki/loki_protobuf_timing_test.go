package loki

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logstorage"
)

func BenchmarkProcessProtobufRequest(b *testing.B) {
	for _, streams := range []int{5, 10} {
		for _, rows := range []int{100, 1000} {
			for _, labels := range []int{10, 50} {
				b.Run(fmt.Sprintf("streams_%d/rows_%d/labels_%d", streams, rows, labels), func(b *testing.B) {
					benchmarkProcessProtobufRequest(b, streams, rows, labels)
				})
			}
		}
	}
}

func benchmarkProcessProtobufRequest(b *testing.B, streams, rows, labels int) {
	body := getProtobufBody(streams, rows, labels)
	b.ReportAllocs()
	b.SetBytes(int64(len(body)))
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := processProtobufRequest(bytes.NewBuffer(body), func(timestamp int64, fields []logstorage.Field) {})
			if err != nil {
				b.Fatalf("unexpected error: %s", err)
			}
		}
	})
}
