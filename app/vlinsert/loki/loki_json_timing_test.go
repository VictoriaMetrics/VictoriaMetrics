package loki

import (
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logstorage"
)

func BenchmarkParseJSONRequest(b *testing.B) {
	for _, streams := range []int{5, 10} {
		for _, rows := range []int{100, 1000} {
			for _, labels := range []int{10, 50} {
				b.Run(fmt.Sprintf("streams_%d/rows_%d/labels_%d", streams, rows, labels), func(b *testing.B) {
					benchmarkParseJSONRequest(b, streams, rows, labels)
				})
			}
		}
	}
}

func benchmarkParseJSONRequest(b *testing.B, streams, rows, labels int) {
	b.ReportAllocs()
	b.SetBytes(int64(streams * rows))
	b.RunParallel(func(pb *testing.PB) {
		data := getJSONBody(streams, rows, labels)
		for pb.Next() {
			_, err := parseJSONRequest(data, func(timestamp int64, fields []logstorage.Field) {})
			if err != nil {
				panic(fmt.Errorf("unexpected error: %w", err))
			}
		}
	})
}

func getJSONBody(streams, rows, labels int) []byte {
	body := append([]byte{}, `{"streams":[`...)
	now := time.Now().UnixNano()
	valuePrefix := fmt.Sprintf(`["%d","value_`, now)

	for i := 0; i < streams; i++ {
		body = append(body, `{"stream":{`...)

		for j := 0; j < labels; j++ {
			body = append(body, `"label_`...)
			body = strconv.AppendInt(body, int64(j), 10)
			body = append(body, `":"value_`...)
			body = strconv.AppendInt(body, int64(j), 10)
			body = append(body, '"')
			if j < labels-1 {
				body = append(body, ',')
			}

		}
		body = append(body, `}, "values":[`...)

		for j := 0; j < rows; j++ {
			body = append(body, valuePrefix...)
			body = strconv.AppendInt(body, int64(j), 10)
			body = append(body, `"]`...)
			if j < rows-1 {
				body = append(body, ',')
			}
		}

		body = append(body, `]}`...)
		if i < streams-1 {
			body = append(body, ',')
		}

	}

	body = append(body, `]}`...)

	return body
}
