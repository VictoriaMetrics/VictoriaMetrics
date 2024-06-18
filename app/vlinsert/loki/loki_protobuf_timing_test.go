package loki

import (
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/golang/snappy"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vlinsert/insertutils"
)

func BenchmarkParseProtobufRequest(b *testing.B) {
	for _, streams := range []int{5, 10} {
		for _, rows := range []int{100, 1000} {
			for _, labels := range []int{10, 50} {
				b.Run(fmt.Sprintf("streams_%d/rows_%d/labels_%d", streams, rows, labels), func(b *testing.B) {
					benchmarkParseProtobufRequest(b, streams, rows, labels)
				})
			}
		}
	}
}

func benchmarkParseProtobufRequest(b *testing.B, streams, rows, labels int) {
	blp := &insertutils.BenchmarkLogMessageProcessor{}
	b.ReportAllocs()
	b.SetBytes(int64(streams * rows))
	b.RunParallel(func(pb *testing.PB) {
		body := getProtobufBody(streams, rows, labels)
		for pb.Next() {
			_, err := parseProtobufRequest(body, blp)
			if err != nil {
				panic(fmt.Errorf("unexpected error: %w", err))
			}
		}
	})
}

func getProtobufBody(streams, rows, labels int) []byte {
	var pr PushRequest

	for i := 0; i < streams; i++ {
		var st Stream

		st.Labels = `{`
		for j := 0; j < labels; j++ {
			st.Labels += `label_` + strconv.Itoa(j) + `="value_` + strconv.Itoa(j) + `"`
			if j < labels-1 {
				st.Labels += `,`
			}
		}
		st.Labels += `}`

		for j := 0; j < rows; j++ {
			st.Entries = append(st.Entries, Entry{Timestamp: time.Now(), Line: "value_" + strconv.Itoa(j)})
		}

		pr.Streams = append(pr.Streams, st)
	}

	body, _ := pr.Marshal()
	encodedBody := snappy.Encode(nil, body)

	return encodedBody
}
