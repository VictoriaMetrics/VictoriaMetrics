package loki

import (
	"fmt"
	"strconv"
	"testing"
	"time"

	"github.com/golang/snappy"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vlinsert/insertutils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
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
			if err := parseProtobufRequest(body, blp, false); err != nil {
				panic(fmt.Errorf("unexpected error: %w", err))
			}
		}
	})
}

func getProtobufBody(streamsCount, rowsCount, labelsCount int) []byte {
	var b []byte
	var entries []Entry
	streams := make([]Stream, streamsCount)
	for i := range streams {
		b = b[:0]
		b = append(b, '{')
		for j := 0; j < labelsCount; j++ {
			b = append(b, "label_"...)
			b = strconv.AppendInt(b, int64(j), 10)
			b = append(b, `="value_`...)
			b = strconv.AppendInt(b, int64(j), 10)
			b = append(b, '"')
			if j < labelsCount-1 {
				b = append(b, ',')
			}
		}
		b = append(b, '}')
		labels := string(b)

		var rowsBuf []byte
		entriesLen := len(entries)
		for j := 0; j < rowsCount; j++ {
			rowsBufLen := len(rowsBuf)
			rowsBuf = append(rowsBuf, "value_"...)
			rowsBuf = strconv.AppendInt(rowsBuf, int64(j), 10)
			entries = append(entries, Entry{
				Timestamp: time.Now(),
				Line:      bytesutil.ToUnsafeString(rowsBuf[rowsBufLen:]),
			})
		}

		st := &streams[i]
		st.Labels = labels
		st.Entries = entries[entriesLen:]
	}
	pr := PushRequest{
		Streams: streams,
	}

	body := pr.MarshalProtobuf(nil)
	encodedBody := snappy.Encode(nil, body)

	return encodedBody
}
