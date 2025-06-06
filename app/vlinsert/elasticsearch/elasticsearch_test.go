package elasticsearch

import (
	"bytes"
	"fmt"
	"io"
	"testing"

	"github.com/golang/snappy"
	"github.com/klauspost/compress/gzip"
	"github.com/klauspost/compress/zlib"
	"github.com/klauspost/compress/zstd"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vlinsert/insertutil"
)

func TestReadBulkRequest_Failure(t *testing.T) {
	f := func(data string) {
		t.Helper()

		tlp := &insertutil.TestLogMessageProcessor{}
		r := bytes.NewBufferString(data)
		rows, parseErrors, err := readBulkRequest("test", r, "", []string{"_time"}, []string{"_msg"}, tlp)
		if err == nil {
			t.Fatalf("expecting non-empty error")
		}
		if rows != 0 {
			t.Fatalf("unexpected non-zero rows=%d", rows)
		}
		if len(parseErrors) > 0 {
			t.Fatalf("unexpected parse errors: %v", parseErrors)
		}
	}
	f("foobar")
	f(`{}`)
	f(`{"create":{}}`)
	f(`{"creat":{}}
{}`)
}

func TestReadBulkRequest_Success(t *testing.T) {
	f := func(data, encoding, timeField, msgField string, rowsExpected int, errPositionsExpected []int, timestampsExpected []int64, resultExpected string) {
		t.Helper()

		timeFields := []string{"non_existing_foo", timeField, "non_existing_bar"}
		msgFields := []string{"non_existing_foo", msgField, "non_exiting_bar"}
		tlp := &insertutil.TestLogMessageProcessor{}

		// Read the request without compression
		r := bytes.NewBufferString(data)
		rows, parseErrors, err := readBulkRequest("test", r, "", timeFields, msgFields, tlp)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		if rows != rowsExpected {
			t.Fatalf("unexpected rows read; got %d; want %d", rows, rowsExpected)
		}
		if err := tlp.Verify(timestampsExpected, resultExpected); err != nil {
			t.Fatal(err)
		}
		if len(parseErrors) != len(errPositionsExpected) {
			t.Fatalf("unexpected parse errors: %#v", parseErrors)
		}
		for i, errPos := range errPositionsExpected {
			if parseErrors[i].pos != errPos {
				t.Fatalf("unexpected position of parse error; got %d; want %d", parseErrors[i].pos, errPos)
			}
		}

		// Read the request with compression
		tlp = &insertutil.TestLogMessageProcessor{}
		if encoding != "" {
			data = compressData(data, encoding)
		}
		r = bytes.NewBufferString(data)
		rows, parseErrors, err = readBulkRequest("test", r, encoding, timeFields, msgFields, tlp)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		if rows != rowsExpected {
			t.Fatalf("unexpected rows read; got %d; want %d", rows, rowsExpected)
		}
		if err := tlp.Verify(timestampsExpected, resultExpected); err != nil {
			t.Fatalf("verification failure after compression: %s", err)
		}
		if len(parseErrors) != len(errPositionsExpected) {
			t.Fatalf("unexpected parse errors: %#v", parseErrors)
		}
		for i, errPos := range errPositionsExpected {
			if parseErrors[i].pos != errPos {
				t.Fatalf("unexpected position of parse error; got %d; want %d", parseErrors[i].pos, errPos)
			}
		}
	}

	// Verify an empty data
	f("", "gzip", "_time", "_msg", 0, nil, nil, "")
	f("\n", "gzip", "_time", "_msg", 0, nil, nil, "")
	f("\n\n", "gzip", "_time", "_msg", 0, nil, nil, "")

	// Do not return an error if all log entry lines are invalid
	f(`{"create":{}}
foobar`, "gzip", "_time", "_msg", 1, []int{0}, nil, "")
	f(`{"create":{}}
foobar
{"create":{}}
foobar
{"create":{}}
foobar`, "gzip", "_time", "_msg", 3, []int{0, 1, 2}, nil, "")

	// Verify non-empty data
	data := `{"create":{"_index":"filebeat-8.8.0"}}
{"@timestamp":"2023-06-06T04:48:11.735Z","log":{"offset":71770,"file":{"path":"/var/log/auth.log"}},"message":"foobar"}
{"create":{"_index":"filebeat-8.8.0"}}
{"_msg":"foo bar","@timestamp":"invalid"}
{"create":{"_index":"filebeat-8.8.0"}}
{"@timestamp":"2023-06-06 04:48:12.735+01:00","message":"baz"}
{"index":{"_index":"filebeat-8.8.0"}}
{"message":"xyz","@timestamp":"1686026893735","x":"y"}
{"create":{"_index":"filebeat-8.8.0"}}
must skip invalid json lines
{"create":{"_index":"filebeat-8.8.0"}}
"must skip non-object JSON lines"
{"create":{"_index":"filebeat-8.8.0"}}
{"message":"qwe rty","@timestamp":"1686026893"}
{"create":{"_index":"filebeat-8.8.0"}}
42
{"create":{"_index":"filebeat-8.8.0"}}
{"message":"qwe rty float","@timestamp":"1686026123.62"}
`
	timeField := "@timestamp"
	msgField := "message"
	timestampsExpected := []int64{1686026891735000000, 1686023292735000000, 1686026893735000000, 1686026893000000000, 1686026123620000000}
	errsPosExpected := []int{1, 4, 5, 7}
	resultExpected := `{"log.offset":"71770","log.file.path":"/var/log/auth.log","_msg":"foobar"}
{"_msg":"baz"}
{"_msg":"xyz","x":"y"}
{"_msg":"qwe rty"}
{"_msg":"qwe rty float"}`
	f(data, "zstd", timeField, msgField, 9, errsPosExpected, timestampsExpected, resultExpected)
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
		return string(snappy.Encode(nil, []byte(s)))
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
