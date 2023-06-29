package loki

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logstorage"
)

func BenchmarkProcessJSONRequest(b *testing.B) {
	for _, streams := range []int{5, 10} {
		for _, rows := range []int{100, 1000} {
			for _, labels := range []int{10, 50} {
				b.Run(fmt.Sprintf("streams_%d/rows_%d/labels_%d", streams, rows, labels), func(b *testing.B) {
					benchmarkProcessJSONRequest(b, streams, rows, labels)
				})
			}
		}
	}
}

func benchmarkProcessJSONRequest(b *testing.B, streams, rows, labels int) {
	s := getJSONBody(streams, rows, labels)
	b.ReportAllocs()
	b.SetBytes(int64(len(s)))
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := processJSONRequest(strings.NewReader(s), func(timestamp int64, fields []logstorage.Field) {})
			if err != nil {
				b.Fatalf("unexpected error: %s", err)
			}
		}
	})
}

func getJSONBody(streams, rows, labels int) string {
	body := `{"streams":[`
	now := time.Now().UnixNano()
	valuePrefix := fmt.Sprintf(`["%d","value_`, now)

	for i := 0; i < streams; i++ {
		body += `{"stream":{`

		for j := 0; j < labels; j++ {
			body += `"label_` + strconv.Itoa(j) + `":"value_` + strconv.Itoa(j) + `"`
			if j < labels-1 {
				body += `,`
			}

		}
		body += `}, "values":[`

		for j := 0; j < rows; j++ {
			body += valuePrefix + strconv.Itoa(j) + `"]`
			if j < rows-1 {
				body += `,`
			}
		}

		body += `]}`
		if i < streams-1 {
			body += `,`
		}

	}

	body += `]}`

	return body
}
