package datadog

import (
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vlinsert/insertutils"
)

func TestReadLogsRequestFailure(t *testing.T) {
	f := func(data string) {
		t.Helper()

		ts := time.Now().UnixNano()

		lmp := &insertutils.TestLogMessageProcessor{}
		if err := readLogsRequest(ts, []byte(data), lmp); err == nil {
			t.Fatalf("expecting non-empty error")
		}
		if err := lmp.Verify(nil, ""); err != nil {
			t.Fatalf("unexpected error: %s", err)
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
		var timestampsExpected []int64
		for i := 0; i < rowsExpected; i++ {
			timestampsExpected = append(timestampsExpected, ts)
		}
		lmp := &insertutils.TestLogMessageProcessor{}
		if err := readLogsRequest(ts, []byte(data), lmp); err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		if err := lmp.Verify(timestampsExpected, resultExpected); err != nil {
			t.Fatalf("unexpected error: %s", err)
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
