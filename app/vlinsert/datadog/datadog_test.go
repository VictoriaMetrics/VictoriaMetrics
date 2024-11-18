package datadog

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logstorage"
)

func TestReadLogsRequestFailure(t *testing.T) {
	f := func(data string) {
		t.Helper()

		ts := time.Now().UnixNano()

		processLogMessage := func(timestamp int64, fields []logstorage.Field) {
			t.Fatalf("unexpected call to processLogMessage with timestamp=%d, fields=%s", timestamp, fields)
		}

		rows, err := readLogsRequest(ts, []byte(data), processLogMessage)
		if err == nil {
			t.Fatalf("expecting non-empty error")
		}
		if rows != 0 {
			t.Fatalf("unexpected non-zero rows=%d", rows)
		}
	}
	f("foobar")
	f(`{}`)
	f(`["create":{}]`)
	f(`{"create":{}}
foobar`)
}

func TestReadLogsRequestSuccess(t *testing.T) {
	f := func(data string, rowsExpected int, resultExpected string) {
		t.Helper()

		ts := time.Now().UnixNano()
		var result string
		processLogMessage := func(_ int64, fields []logstorage.Field) {
			a := make([]string, len(fields))
			for i, f := range fields {
				a[i] = fmt.Sprintf("%q:%q", f.Name, f.Value)
			}
			if len(result) > 0 {
				result = result + "\n"
			}
			s := "{" + strings.Join(a, ",") + "}"
			result += s
		}

		// Read the request without compression
		rows, err := readLogsRequest(ts, []byte(data), processLogMessage)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		if rows != rowsExpected {
			t.Fatalf("unexpected rows read; got %d; want %d", rows, rowsExpected)
		}

		if result != resultExpected {
			t.Fatalf("unexpected result;\ngot\n%s\nwant\n%s", result, resultExpected)
		}
	}

	// Verify non-empty data
	data := `[
		{
			"ddsource":"nginx",
			"ddtags":"tag1:value1,tag2:value2",
			"hostname":"127.0.0.1",
			"message":"bar",
			"service":"test"
		}, {
			"ddsource":"nginx",
			"ddtags":"tag1:value1,tag2:value2",
			"hostname":"127.0.0.1",
			"message":"foobar",
			"service":"test"
		}, {
			"ddsource":"nginx",
			"ddtags":"tag1:value1,tag2:value2",
			"hostname":"127.0.0.1",
			"message":"baz",
			"service":"test"
		}, {
			"ddsource":"nginx",
			"ddtags":"tag1:value1,tag2:value2",
			"hostname":"127.0.0.1",
			"message":"xyz",
			"service":"test"
		}, {
			"ddsource": "nginx",
			"ddtags":"tag1:value1,tag2:value2,",
			"hostname":"127.0.0.1",
                        "message":"xyz",
                        "service":"test"
                }, {
			"ddsource":"nginx",
			"ddtags":",tag1:value1,tag2:value2",
			"hostname":"127.0.0.1",
                        "message":"xyz",
                        "service":"test"
		}
	]`
	rowsExpected := 6
	resultExpected := `{"ddsource":"nginx","tag1":"value1","tag2":"value2","hostname":"127.0.0.1","_msg":"bar","service":"test"}
{"ddsource":"nginx","tag1":"value1","tag2":"value2","hostname":"127.0.0.1","_msg":"foobar","service":"test"}
{"ddsource":"nginx","tag1":"value1","tag2":"value2","hostname":"127.0.0.1","_msg":"baz","service":"test"}
{"ddsource":"nginx","tag1":"value1","tag2":"value2","hostname":"127.0.0.1","_msg":"xyz","service":"test"}
{"ddsource":"nginx","tag1":"value1","tag2":"value2","hostname":"127.0.0.1","_msg":"xyz","service":"test"}
{"ddsource":"nginx","tag1":"value1","tag2":"value2","hostname":"127.0.0.1","_msg":"xyz","service":"test"}`
	f(data, rowsExpected, resultExpected)
}
