package jsonline

import (
	"bytes"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vlinsert/insertutils"
)

func TestProcessStreamInternal_Success(t *testing.T) {
	f := func(data, timeField, msgField string, rowsExpected int, timestampsExpected []int64, resultExpected string) {
		t.Helper()

		tlp := &insertutils.TestLogMessageProcessor{}
		r := bytes.NewBufferString(data)
		if err := processStreamInternal(r, timeField, msgField, tlp); err != nil {
			t.Fatalf("unexpected error: %s", err)
		}

		if err := tlp.Verify(rowsExpected, timestampsExpected, resultExpected); err != nil {
			t.Fatal(err)
		}
	}

	data := `{"@timestamp":"2023-06-06T04:48:11.735Z","log":{"offset":71770,"file":{"path":"/var/log/auth.log"}},"message":"foobar"}
{"@timestamp":"2023-06-06T04:48:12.735+01:00","message":"baz"}
{"message":"xyz","@timestamp":"2023-06-06 04:48:13.735Z","x":"y"}
`
	timeField := "@timestamp"
	msgField := "message"
	rowsExpected := 3
	timestampsExpected := []int64{1686026891735000000, 1686023292735000000, 1686026893735000000}
	resultExpected := `{"@timestamp":"","log.offset":"71770","log.file.path":"/var/log/auth.log","_msg":"foobar"}
{"@timestamp":"","_msg":"baz"}
{"_msg":"xyz","@timestamp":"","x":"y"}`
	f(data, timeField, msgField, rowsExpected, timestampsExpected, resultExpected)
}

func TestProcessStreamInternal_Failure(t *testing.T) {
	f := func(data string) {
		t.Helper()

		tlp := &insertutils.TestLogMessageProcessor{}
		r := bytes.NewBufferString(data)
		if err := processStreamInternal(r, "time", "", tlp); err == nil {
			t.Fatalf("expecting non-nil error")
		}
	}

	// invalid json
	f("foobar")

	// invalid timestamp field
	f(`{"time":"foobar"}`)
}
