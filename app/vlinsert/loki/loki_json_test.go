package loki

import (
	"strings"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vlinsert/insertutils"
)

func TestParseJSONRequest_Failure(t *testing.T) {
	f := func(s string) {
		t.Helper()

		tlp := &insertutils.TestLogMessageProcessor{}
		if err := parseJSONRequest(strings.NewReader(s), "", tlp, nil, false, false); err == nil {
			t.Fatalf("expecting non-nil error")
		}
		if err := tlp.Verify(nil, ""); err != nil {
			t.Fatalf("unexpected error: %s", err)
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
	f(`{"streams":[{"values":[["123","456","789","8123"]]}]}`)

	// Invalid type for timestamp inside `values` individual item
	f(`{"streams":[{"values":[[123,"456"]}]}`)

	// Invalid type for log message
	f(`{"streams":[{"values":[["123",1234]]}]}`)

	// invalid structured metadata type
	f(`{"streams":[{"values":[["1577836800000000001", "foo bar", ["metadata_1", "md_value"]]]}]}`)

	// structured metadata with unexpected value type
	f(`{"streams":[{"values":[["1577836800000000001", "foo bar", {"metadata_1": 1}]] }]}`)
}

func TestParseJSONRequest_Success(t *testing.T) {
	f := func(s string, timestampsExpected []int64, resultExpected string) {
		t.Helper()

		tlp := &insertutils.TestLogMessageProcessor{}

		if err := parseJSONRequest(strings.NewReader(s), "", tlp, nil, false, false); err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		if err := tlp.Verify(timestampsExpected, resultExpected); err != nil {
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
	["1686026123.62", "abc"],
	["147.78369e9", "foobar"]
]}]}`, []int64{1577836800000000001, 1686026123620000000, 147783690000000000}, `{"label1":"value1","label2":"value2","_msg":"foo bar"}
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

	// values with metadata
	f(`{"streams":[{"values":[["1577836800000000001", "foo bar", {"metadata_1": "md_value"}]]}]}`, []int64{1577836800000000001}, `{"metadata_1":"md_value","_msg":"foo bar"}`)
	f(`{"streams":[{"values":[["1577836800000000001", "foo bar", {}]]}]}`, []int64{1577836800000000001}, `{"_msg":"foo bar"}`)
}

func TestParseJSONRequest_ParseMessage(t *testing.T) {
	f := func(s string, msgFields []string, timestampsExpected []int64, resultExpected string) {
		t.Helper()

		tlp := &insertutils.TestLogMessageProcessor{}

		if err := parseJSONRequest(strings.NewReader(s), "", tlp, msgFields, false, true); err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		if err := tlp.Verify(timestampsExpected, resultExpected); err != nil {
			t.Fatal(err)
		}
	}

	f(`{
	"streams": [
		{
			"stream": {
				"foo": "bar",
				"a": "b"
			},
			"values": [
				["1577836800000000001", "{\"user_id\":\"123\"}"],
				["1577836900005000002", "abc", {"trace_id":"pqw"}],
				["1577836900005000003", "{def}"]
			]
		},
		{
			"stream": {
				"x": "y"
			},
			"values": [
				["1877836900005000004", "{\"trace_id\":\"111\",\"parent_id\":\"abc\"}"]
			]
		}
	]
}`, []string{"a", "trace_id"}, []int64{1577836800000000001, 1577836900005000002, 1577836900005000003, 1877836900005000004}, `{"foo":"bar","a":"b","user_id":"123"}
{"foo":"bar","a":"b","trace_id":"pqw","_msg":"abc"}
{"foo":"bar","a":"b","_msg":"{def}"}
{"x":"y","_msg":"111","parent_id":"abc"}`)
}
