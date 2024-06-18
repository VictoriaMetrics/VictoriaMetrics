package loki

import (
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vlinsert/insertutils"
)

func TestParseJSONRequest_Failure(t *testing.T) {
	f := func(s string) {
		t.Helper()

		tlp := &insertutils.TestLogMessageProcessor{}
		n, err := parseJSONRequest([]byte(s), tlp)
		if err == nil {
			t.Fatalf("expecting non-nil error")
		}
		if n != 0 {
			t.Fatalf("unexpected number of parsed lines: %d; want 0", n)
		}
	}
	f(``)

	// Invalid json
	f(`{}`)
	f(`[]`)
	f(`"foo"`)
	f(`123`)

	// invalid type for `streams` item
	f(`{"streams":123}`)

	// Missing `values` item
	f(`{"streams":[{}]}`)

	// Invalid type for `values` item
	f(`{"streams":[{"values":"foobar"}]}`)

	// Invalid type for `stream` item
	f(`{"streams":[{"stream":[],"values":[]}]}`)

	// Invalid type for `values` individual item
	f(`{"streams":[{"values":[123]}]}`)

	// Invalid length of `values` individual item
	f(`{"streams":[{"values":[[]]}]}`)
	f(`{"streams":[{"values":[["123"]]}]}`)
	f(`{"streams":[{"values":[["123","456","789"]]}]}`)

	// Invalid type for timestamp inside `values` individual item
	f(`{"streams":[{"values":[[123,"456"]}]}`)

	// Invalid type for log message
	f(`{"streams":[{"values":[["123",1234]]}]}`)
}

func TestParseJSONRequest_Success(t *testing.T) {
	f := func(s string, timestampsExpected []int64, resultExpected string) {
		t.Helper()

		tlp := &insertutils.TestLogMessageProcessor{}

		n, err := parseJSONRequest([]byte(s), tlp)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		if err := tlp.Verify(n, timestampsExpected, resultExpected); err != nil {
			t.Fatal(err)
		}
	}

	// Empty streams
	f(`{"streams":[]}`, nil, ``)
	f(`{"streams":[{"values":[]}]}`, nil, ``)
	f(`{"streams":[{"stream":{},"values":[]}]}`, nil, ``)
	f(`{"streams":[{"stream":{"foo":"bar"},"values":[]}]}`, nil, ``)

	// Empty stream labels
	f(`{"streams":[{"values":[["1577836800000000001", "foo bar"]]}]}`, []int64{1577836800000000001}, `{"_msg":"foo bar"}`)
	f(`{"streams":[{"stream":{},"values":[["1577836800000000001", "foo bar"]]}]}`, []int64{1577836800000000001}, `{"_msg":"foo bar"}`)

	// Non-empty stream labels
	f(`{"streams":[{"stream":{
	"label1": "value1",
	"label2": "value2"
},"values":[
	["1577836800000000001", "foo bar"],
	["1477836900005000002", "abc"],
	["147.78369e9", "foobar"]
]}]}`, []int64{1577836800000000001, 1477836900005000002, 147783690000}, `{"label1":"value1","label2":"value2","_msg":"foo bar"}
{"label1":"value1","label2":"value2","_msg":"abc"}
{"label1":"value1","label2":"value2","_msg":"foobar"}`)

	// Multiple streams
	f(`{
	"streams": [
		{
			"stream": {
				"foo": "bar",
				"a": "b"
			},
			"values": [
				["1577836800000000001", "foo bar"],
				["1577836900005000002", "abc"]
			]
		},
		{
			"stream": {
				"x": "y"
			},
			"values": [
				["1877836900005000002", "yx"]
			]
		}
	]
}`, []int64{1577836800000000001, 1577836900005000002, 1877836900005000002}, `{"foo":"bar","a":"b","_msg":"foo bar"}
{"foo":"bar","a":"b","_msg":"abc"}
{"x":"y","_msg":"yx"}`)
}
