package elasticsearch

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vlinsert/insertutils"
)

func TestReadBulkRequest_Failure(t *testing.T) {
	f := func(data string) {
		t.Helper()

		tlp := &insertutils.TestLogMessageProcessor{}
		r := bytes.NewBufferString(data)
		rows, err := readBulkRequest(r, false, "_time", "_msg", tlp)
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
	f := func(data, timeField, msgField string, rowsExpected int, timestampsExpected []int64, resultExpected string) {
		t.Helper()

		tlp := &insertutils.TestLogMessageProcessor{}

		// Read the request without compression
		r := bytes.NewBufferString(data)
		rows, err := readBulkRequest(r, false, timeField, msgField, tlp)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		if rows != rowsExpected {
			t.Fatalf("unexpected rows read; got %d; want %d", rows, rowsExpected)
		}
		if err := tlp.Verify(rowsExpected, timestampsExpected, resultExpected); err != nil {
			t.Fatal(err)
		}

		// Read the request with compression
		tlp = &insertutils.TestLogMessageProcessor{}
		compressedData := compressData(data)
		r = bytes.NewBufferString(compressedData)
		rows, err = readBulkRequest(r, true, timeField, msgField, tlp)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		if rows != rowsExpected {
			t.Fatalf("unexpected rows read; got %d; want %d", rows, rowsExpected)
		}
		if err := tlp.Verify(rowsExpected, timestampsExpected, resultExpected); err != nil {
			t.Fatalf("verification failure after compression: %s", err)
		}
	}

	// Verify an empty data
	f("", "_time", "_msg", 0, nil, "")
	f("\n", "_time", "_msg", 0, nil, "")
	f("\n\n", "_time", "_msg", 0, nil, "")

	// Verify non-empty data
	data := `{"create":{"_index":"filebeat-8.8.0"}}
{"@timestamp":"2023-06-06T04:48:11.735Z","log":{"offset":71770,"file":{"path":"/var/log/auth.log"}},"message":"foobar"}
{"create":{"_index":"filebeat-8.8.0"}}
{"@timestamp":"2023-06-06 04:48:12.735+01:00","message":"baz"}
{"index":{"_index":"filebeat-8.8.0"}}
{"message":"xyz","@timestamp":"1686026893735","x":"y"}
{"create":{"_index":"filebeat-8.8.0"}}
{"message":"qwe rty","@timestamp":"1686026893"}
`
	timeField := "@timestamp"
	msgField := "message"
	rowsExpected := 4
	timestampsExpected := []int64{1686026891735000000, 1686023292735000000, 1686026893735000000, 1686026893000000000}
	resultExpected := `{"@timestamp":"","log.offset":"71770","log.file.path":"/var/log/auth.log","_msg":"foobar"}
{"@timestamp":"","_msg":"baz"}
{"_msg":"xyz","@timestamp":"","x":"y"}
{"_msg":"qwe rty","@timestamp":""}`
	f(data, timeField, msgField, rowsExpected, timestampsExpected, resultExpected)
}

func compressData(s string) string {
	var bb bytes.Buffer
	zw := gzip.NewWriter(&bb)
	if _, err := zw.Write([]byte(s)); err != nil {
		panic(fmt.Errorf("unexpected error when compressing data: %w", err))
	}
	if err := zw.Close(); err != nil {
		panic(fmt.Errorf("unexpected error when closing gzip writer: %w", err))
	}
	return bb.String()
}
