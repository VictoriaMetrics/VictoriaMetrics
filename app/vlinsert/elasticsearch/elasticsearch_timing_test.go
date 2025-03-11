package elasticsearch

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vlinsert/insertutils"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
)

func BenchmarkReadBulkRequest(b *testing.B) {
	b.Run("encoding:none", func(b *testing.B) {
		benchmarkReadBulkRequest(b, "")
	})
	b.Run("encoding:gzip", func(b *testing.B) {
		benchmarkReadBulkRequest(b, "gzip")
	})
	b.Run("encoding:zstd", func(b *testing.B) {
		benchmarkReadBulkRequest(b, "zstd")
	})
	b.Run("encoding:deflate", func(b *testing.B) {
		benchmarkReadBulkRequest(b, "deflate")
	})
	b.Run("encoding:snappy", func(b *testing.B) {
		benchmarkReadBulkRequest(b, "snappy")
	})
}

func benchmarkReadBulkRequest(b *testing.B, encoding string) {
	data := `{"create":{"_index":"filebeat-8.8.0"}}
{"@timestamp":"2023-06-06T04:48:11.735Z","log":{"offset":71770,"file":{"path":"/var/log/auth.log"}},"message":"foobar"}
{"create":{"_index":"filebeat-8.8.0"}}
{"@timestamp":"2023-06-06T04:48:12.735Z","message":"baz"}
{"create":{"_index":"filebeat-8.8.0"}}
{"message":"xyz","@timestamp":"2023-06-06T04:48:13.735Z","x":"y"}
`
	if encoding != "" {
		data = compressData(data, encoding)
	}
	dataBytes := bytesutil.ToUnsafeBytes(data)

	timeField := "@timestamp"
	msgFields := []string{"message"}
	blp := &insertutils.BenchmarkLogMessageProcessor{}

	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	b.RunParallel(func(pb *testing.PB) {
		r := &bytes.Reader{}
		for pb.Next() {
			r.Reset(dataBytes)
			_, err := readBulkRequest("test", r, encoding, timeField, msgFields, blp)
			if err != nil {
				panic(fmt.Errorf("unexpected error: %w", err))
			}
		}
	})
}
