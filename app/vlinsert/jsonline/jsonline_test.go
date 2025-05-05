package jsonline

import (
	"bytes"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vlinsert/insertutil"
)

func TestProcessStreamInternal(t *testing.T) {
	f := func(data, timeField, msgField string, timestampsExpected []int64, resultExpected string) {
		t.Helper()

		msgFields := []string{msgField}
		tlp := &insertutil.TestLogMessageProcessor{}
		r := bytes.NewBufferString(data)
		processStreamInternal("test", r, timeField, msgFields, tlp)

		if err := tlp.Verify(timestampsExpected, resultExpected); err != nil {
			t.Fatal(err)
		}
	}

	data := `{"@timestamp":"2023-06-06T04:48:11.735Z","log":{"offset":71770,"file":{"path":"/var/log/auth.log"}},"message":"foobar"}
{"@timestamp":"2023-06-06T04:48:12.735+01:00","message":"baz"}
{"message":"xyz","@timestamp":"2023-06-06 04:48:13.735Z","x":"y"}
`
	timeField := "@timestamp"
	msgField := "message"
	timestampsExpected := []int64{1686026891735000000, 1686023292735000000, 1686026893735000000}
	resultExpected := `{"log.offset":"71770","log.file.path":"/var/log/auth.log","_msg":"foobar"}
{"_msg":"baz"}
{"_msg":"xyz","x":"y"}`
	f(data, timeField, msgField, timestampsExpected, resultExpected)

	// Non-existing msgField
	data = `{"@timestamp":"2023-06-06T04:48:11.735Z","log":{"offset":71770,"file":{"path":"/var/log/auth.log"}},"message":"foobar"}
{"@timestamp":"2023-06-06T04:48:12.735+01:00","message":"baz"}
`
	timeField = "@timestamp"
	msgField = "foobar"
	timestampsExpected = []int64{1686026891735000000, 1686023292735000000}
	resultExpected = `{"log.offset":"71770","log.file.path":"/var/log/auth.log","message":"foobar"}
{"message":"baz"}`
	f(data, timeField, msgField, timestampsExpected, resultExpected)

	// invalid json
	data = "foobar"
	timeField = "@timestamp"
	msgField = "aaa"
	timestampsExpected = nil
	resultExpected = ``
	f(data, timeField, msgField, timestampsExpected, resultExpected)

	// invalid timestamp field
	data = `{"time":"foobar"}`
	timeField = "time"
	msgField = "abc"
	timestampsExpected = nil
	resultExpected = ``
	f(data, timeField, msgField, timestampsExpected, resultExpected)

	// invalid lines among valid lines
	data = `
dsfodmasd

{"time":"2023-06-06T04:48:11.735Z","log":{"offset":71770,"file":{"path":"/var/log/auth.log"}},"message":"foobar"}
invalid line
{"time":"2023-06-06T04:48:12.735+01:00","message":"baz"}
asbsdf

`
	timeField = "time"
	msgField = "message"
	timestampsExpected = []int64{1686026891735000000, 1686023292735000000}
	resultExpected = `{"log.offset":"71770","log.file.path":"/var/log/auth.log","_msg":"foobar"}
{"_msg":"baz"}`
	f(data, timeField, msgField, timestampsExpected, resultExpected)
}
