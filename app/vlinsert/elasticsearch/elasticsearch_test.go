package elasticsearch

import (
	"bytes"
	"fmt"
	"github.com/golang/snappy"
	"github.com/klauspost/compress/gzip"
	"github.com/klauspost/compress/zlib"
	"github.com/klauspost/compress/zstd"
	"io"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vlinsert/insertutils"
)

func TestReadBulkRequest_Failure(t *testing.T) {
	f := func(data string) {
		t.Helper()

		tlp := &insertutils.TestLogMessageProcessor{}
		r := bytes.NewBufferString(data)
		rows, err := readBulkRequest("test", r, "", "_time", []string{"_msg"}, tlp)
		if err == nil {
			t.Fatalf("expecting non-empty error")
		}
		if rows != 0 {
			t.Fatalf("unexpected non-zero rows=%d", rows)
		}
	}
	f("foobar")
	f(`{}`)
	f(`{"create":{}}`)
	f(`{"creat":{}}
{}`)
	f(`{"create":{}}
foobar`)
}

func TestReadBulkRequest_Success(t *testing.T) {
	f := func(data, encoding, timeField, msgField string, timestampsExpected []int64, resultExpected string) {
		t.Helper()

		msgFields := []string{"non_existing_foo", msgField, "non_exiting_bar"}
		tlp := &insertutils.TestLogMessageProcessor{}

		// Read the request without compression
		r := bytes.NewBufferString(data)
		rows, err := readBulkRequest("test", r, "", timeField, msgFields, tlp)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		if rows != len(timestampsExpected) {
			t.Fatalf("unexpected rows read; got %d; want %d", rows, len(timestampsExpected))
		}
		if err := tlp.Verify(timestampsExpected, resultExpected); err != nil {
			t.Fatal(err)
		}

		// Read the request with compression
		tlp = &insertutils.TestLogMessageProcessor{}
		if encoding != "" {
			data = compressData(data, encoding)
		}
		r = bytes.NewBufferString(data)
		rows, err = readBulkRequest("test", r, encoding, timeField, msgFields, tlp)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		if rows != len(timestampsExpected) {
			t.Fatalf("unexpected rows read; got %d; want %d", rows, len(timestampsExpected))
		}
		if err := tlp.Verify(timestampsExpected, resultExpected); err != nil {
			t.Fatalf("verification failure after compression: %s", err)
		}
	}

	// Verify an empty data
	f("", "gzip", "_time", "_msg", nil, "")
	f("\n", "gzip", "_time", "_msg", nil, "")
	f("\n\n", "gzip", "_time", "_msg", nil, "")

	// Verify non-empty data
	data := `{"create":{"_index":"filebeat-8.8.0"}}
{"@timestamp":"2023-06-06T04:48:11.735Z","log":{"offset":71770,"file":{"path":"/var/log/auth.log"}},"message":"foobar"}
{"create":{"_index":"filebeat-8.8.0"}}
{"@timestamp":"2023-06-06 04:48:12.735+01:00","message":"baz"}
{"index":{"_index":"filebeat-8.8.0"}}
{"message":"xyz","@timestamp":"1686026893735","x":"y"}
{"create":{"_index":"filebeat-8.8.0"}}
{"message":"qwe rty","@timestamp":"1686026893"}
{"create":{"_index":"filebeat-8.8.0"}}
{"message":"qwe rty float","@timestamp":"1686026123.62"}
`
	timeField := "@timestamp"
	msgField := "message"
	timestampsExpected := []int64{1686026891735000000, 1686023292735000000, 1686026893735000000, 1686026893000000000, 1686026123620000000}
	resultExpected := `{"log.offset":"71770","log.file.path":"/var/log/auth.log","_msg":"foobar"}
{"_msg":"baz"}
{"_msg":"xyz","x":"y"}
{"_msg":"qwe rty"}
{"_msg":"qwe rty float"}`
	f(data, "zstd", timeField, msgField, timestampsExpected, resultExpected)
}

func compressData(s string, encoding string) string {
	var bb bytes.Buffer
	var zw io.WriteCloser
	switch encoding {
	case "gzip":
		zw = gzip.NewWriter(&bb)
	case "zstd":
		zw, _ = zstd.NewWriter(&bb)
	case "snappy":
		zw = snappy.NewBufferedWriter(&bb)
	case "deflate":
		zw = zlib.NewWriter(&bb)
	default:
		panic(fmt.Errorf("%q encoding is not supported", encoding))
	}
	if _, err := zw.Write([]byte(s)); err != nil {
		panic(fmt.Errorf("unexpected error when compressing data: %w", err))
	}
	if err := zw.Close(); err != nil {
		panic(fmt.Errorf("unexpected error when closing gzip writer: %w", err))
	}
	return bb.String()
}
