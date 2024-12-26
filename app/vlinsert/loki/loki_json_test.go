package loki

import (
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vlinsert/insertutils"
)

func TestParseJSONRequest_Failure(t *testing.T) {
	f := func(s string, msgFields []string) {
		t.Helper()

		tlp := &insertutils.TestLogMessageProcessor{}
		if err := parseJSONRequest([]byte(s), msgFields, tlp, false); err == nil {
			t.Fatalf("expecting non-nil error")
		}
		if err := tlp.Verify(nil, ""); err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
	}
	f(``, nil)

	// Invalid json
	f(`{}`, nil)
	f(`[]`, nil)
	f(`"foo"`, nil)
	f(`123`, nil)

	// invalid type for `streams` item
	f(`{"streams":123}`, nil)

	// Missing `values` item
	f(`{"streams":[{}]}`, nil)

	// Invalid type for `values` item
	f(`{"streams":[{"values":"foobar"}]}`, nil)

	// Invalid type for `stream` item
	f(`{"streams":[{"stream":[],"values":[]}]}`, nil)

	// Invalid type for `values` individual item
	f(`{"streams":[{"values":[123]}]}`, nil)

	// Invalid length of `values` individual item
	f(`{"streams":[{"values":[[]]}]}`, nil)
	f(`{"streams":[{"values":[["123"]]}]}`, nil)
	f(`{"streams":[{"values":[["123","456","789","8123"]]}]}`, nil)

	// Invalid type for timestamp inside `values` individual item
	f(`{"streams":[{"values":[[123,"456"]}]}`, nil)

	// Invalid type for log message
	f(`{"streams":[{"values":[["123",1234]]}]}`, nil)

	// invalid structured metadata type
	f(`{"streams":[{"values":[["1577836800000000001", "foo bar", ["metadata_1", "md_value"]]]}]}`, nil)

	// structured metadata with unexpected value type
	f(`{"streams":[{"values":[["1577836800000000001", "foo bar", {"metadata_1": 1}]] }]}`, nil)

	// json message
	f(`{"streams":[{"values":[["1577836800000000001", {"message": "foo bar"}, {"metadata_1": 1}]] }]}`, []string{"message"})
}

func TestParseJSONRequest_Success(t *testing.T) {
	f := func(s string, msgFields []string, timestampsExpected []int64, resultExpected string) {
		t.Helper()

		tlp := &insertutils.TestLogMessageProcessor{}

		if err := parseJSONRequest([]byte(s), msgFields, tlp, false); err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		if err := tlp.Verify(timestampsExpected, resultExpected); err != nil {
			t.Fatal(err)
		}
	}

	// Empty streams
	f(`{"streams":[]}`, nil, nil, ``)
	f(`{"streams":[{"values":[]}]}`, nil, nil, ``)
	f(`{"streams":[{"stream":{},"values":[]}]}`, nil, nil, ``)
	f(`{"streams":[{"stream":{"foo":"bar"},"values":[]}]}`, nil, nil, ``)

	// Empty stream labels
	f(`{"streams":[{"values":[["1577836800000000001", "foo bar"]]}]}`, nil, []int64{1577836800000000001}, `{"_msg":"foo bar"}`)
	f(`{"streams":[{"stream":{},"values":[["1577836800000000001", "foo bar"]]}]}`, nil, []int64{1577836800000000001}, `{"_msg":"foo bar"}`)

	// Non-empty stream labels
	f(`{"streams":[{"stream":{
	"label1": "value1",
	"label2": "value2"
},"values":[
	["1577836800000000001", "foo bar"],
	["1477836900005000002", "abc"],
	["147.78369e9", "foobar"]
]}]}`, nil, []int64{1577836800000000001, 1477836900005000002, 147783690000}, `{"label1":"value1","label2":"value2","_msg":"foo bar"}
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
}`, nil, []int64{1577836800000000001, 1577836900005000002, 1877836900005000002}, `{"foo":"bar","a":"b","_msg":"foo bar"}
{"foo":"bar","a":"b","_msg":"abc"}
{"x":"y","_msg":"yx"}`)

	// values with metadata
	f(`{"streams":[{"values":[["1577836800000000001", "foo bar", {"metadata_1": "md_value"}]]}]}`, nil, []int64{1577836800000000001}, `{"_msg":"foo bar","metadata_1":"md_value"}`)
	f(`{"streams":[{"values":[["1577836800000000001", "foo bar", {}]]}]}`, nil, []int64{1577836800000000001}, `{"_msg":"foo bar"}`)
}
