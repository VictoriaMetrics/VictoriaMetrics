package jsonline

import (
	"bufio"
	"bytes"
	"fmt"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logstorage"
	"reflect"
	"strings"
	"testing"
)

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
		sc := bufio.NewScanner(r)
		rows := 0
		for {
			ok, err := readLine(sc, timeField, msgField, processLogMessage)
			if err != nil {
				t.Fatalf("unexpected error: %s", err)
			}
			if !ok {
				break
			}
			rows++
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

	// Verify non-empty data
	data := `{"@timestamp":"2023-06-06T04:48:11.735Z","log":{"offset":71770,"file":{"path":"/var/log/auth.log"}},"message":"foobar"}
{"@timestamp":"2023-06-06T04:48:12.735Z","message":"baz"}
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
