package elasticsearch

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logstorage"
)

func TestReadBulkRequestFailure(t *testing.T) {
	f := func(data string) {
		t.Helper()

		processLogMessage := func(timestamp int64, fields []logstorage.Field) {
			t.Fatalf("unexpected call to processLogMessage with timestamp=%d, fields=%s", timestamp, fields)
		}

		r := bytes.NewBufferString(data)
		rows, err := readBulkRequest(r, false, "_time", "_msg", processLogMessage)
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

func TestReadBulkRequestSuccess(t *testing.T) {
	f := func(data, timeField, msgField string, rowsExpected int, timestampsExpected []int64, resultExpected string) {
		t.Helper()

		var timestamps []int64
		var result string
		processLogMessage := func(timestamp int64, fields []logstorage.Field) {
			timestamps = append(timestamps, timestamp)

			a := make([]string, len(fields))
			for i, f := range fields {
				a[i] = fmt.Sprintf("%q:%q", f.Name, f.Value)
			}
			s := "{" + strings.Join(a, ",") + "}\n"
			result += s
		}

		// Read the request without compression
		r := bytes.NewBufferString(data)
		rows, err := readBulkRequest(r, false, timeField, msgField, processLogMessage)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		if rows != rowsExpected {
			t.Fatalf("unexpected rows read; got %d; want %d", rows, rowsExpected)
		}

		if !reflect.DeepEqual(timestamps, timestampsExpected) {
			t.Fatalf("unexpected timestamps;\ngot\n%d\nwant\n%d", timestamps, timestampsExpected)
		}
		if result != resultExpected {
			t.Fatalf("unexpected result;\ngot\n%s\nwant\n%s", result, resultExpected)
		}

		// Read the request with compression
		timestamps = nil
		result = ""
		compressedData := compressData(data)
		r = bytes.NewBufferString(compressedData)
		rows, err = readBulkRequest(r, true, timeField, msgField, processLogMessage)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		if rows != rowsExpected {
			t.Fatalf("unexpected rows read; got %d; want %d", rows, rowsExpected)
		}

		if !reflect.DeepEqual(timestamps, timestampsExpected) {
			t.Fatalf("unexpected timestamps;\ngot\n%d\nwant\n%d", timestamps, timestampsExpected)
		}
		if result != resultExpected {
			t.Fatalf("unexpected result;\ngot\n%s\nwant\n%s", result, resultExpected)
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
{"@timestamp":"2023-06-06T04:48:12.735Z","message":"baz"}
{"index":{"_index":"filebeat-8.8.0"}}
{"message":"xyz","@timestamp":"2023-06-06T04:48:13.735Z","x":"y"}
`
	timeField := "@timestamp"
	msgField := "message"
	rowsExpected := 3
	timestampsExpected := []int64{1686026891735000000, 1686026892735000000, 1686026893735000000}
	resultExpected := `{"@timestamp":"","log.offset":"71770","log.file.path":"/var/log/auth.log","_msg":"foobar"}
{"@timestamp":"","_msg":"baz"}
{"_msg":"xyz","@timestamp":"","x":"y"}
`
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
