package opentelemetry

import (
	"fmt"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vlinsert/insertutils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/opentelemetry/pb"
)

func BenchmarkParseProtobufRequest(b *testing.B) {
	for _, scopes := range []int{1, 2} {
		for _, rows := range []int{100, 1000} {
			for _, attributes := range []int{5, 10} {
				b.Run(fmt.Sprintf("scopes_%d/rows_%d/attributes_%d", scopes, rows, attributes), func(b *testing.B) {
					benchmarkParseProtobufRequest(b, scopes, rows, attributes)
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
			_, err := pushProtobufRequest(body, blp)
			if err != nil {
				panic(fmt.Errorf("unexpected error: %w", err))
			}
		}
	})
}

func getProtobufBody(scopesCount, rowsCount, attributesCount int) []byte {
	msg := "12345678910"

	attrValues := []*pb.AnyValue{
		{StringValue: ptrTo("string-attribute")},
		{IntValue: ptrTo[int64](12345)},
		{DoubleValue: ptrTo(3.14)},
	}
	attrs := make([]*pb.KeyValue, attributesCount)
	for j := 0; j < attributesCount; j++ {
		attrs[j] = &pb.KeyValue{
			Key:   fmt.Sprintf("key-%d", j),
			Value: attrValues[j%3],
		}
	}
	entries := make([]pb.LogRecord, rowsCount)
	for j := 0; j < rowsCount; j++ {
		entries[j] = pb.LogRecord{
			TimeUnixNano: 12345678910, ObservedTimeUnixNano: 12345678910, Body: pb.AnyValue{StringValue: &msg},
		}
	}
	scopes := make([]pb.ScopeLogs, scopesCount)

	for j := 0; j < scopesCount; j++ {
		scopes[j] = pb.ScopeLogs{
			LogRecords: entries,
		}
	}

	pr := pb.ExportLogsServiceRequest{
		ResourceLogs: []pb.ResourceLogs{
			{
				Resource: pb.Resource{
					Attributes: attrs,
				},
				ScopeLogs: scopes,
			},
		},
	}

	return pr.MarshalProtobuf(nil)
}
