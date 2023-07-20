package loki

import (
	"fmt"
	"strings"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logstorage"
)

func TestParseJSONRequestFailure(t *testing.T) {
	f := func(s string) {
		t.Helper()
		n, err := parseJSONRequest([]byte(s), func(timestamp int64, fields []logstorage.Field) {
			t.Fatalf("unexpected call to parseJSONRequest callback!")
		})
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

func TestParseJSONRequestSuccess(t *testing.T) {
	f := func(s string, resultExpected string) {
		t.Helper()
		var lines []string
		n, err := parseJSONRequest([]byte(s), func(timestamp int64, fields []logstorage.Field) {
			var a []string
			for _, f := range fields {
				a = append(a, f.String())
			}
			line := fmt.Sprintf("_time:%d %s", timestamp, strings.Join(a, " "))
			lines = append(lines, line)
		})
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		if n != len(lines) {
			t.Fatalf("unexpected number of lines parsed; got %d; want %d", n, len(lines))
		}
		result := strings.Join(lines, "\n")
		if result != resultExpected {
			t.Fatalf("unexpected result;\ngot\n%s\nwant\n%s", result, resultExpected)
		}
	}

	// Empty streams
	f(`{"streams":[]}`, ``)
	f(`{"streams":[{"values":[]}]}`, ``)
	f(`{"streams":[{"stream":{},"values":[]}]}`, ``)
	f(`{"streams":[{"stream":{"foo":"bar"},"values":[]}]}`, ``)

	// Empty stream labels
	f(`{"streams":[{"values":[["1577836800000000001", "foo bar"]]}]}`, `_time:1577836800000000001 "_msg":"foo bar"`)
	f(`{"streams":[{"stream":{},"values":[["1577836800000000001", "foo bar"]]}]}`, `_time:1577836800000000001 "_msg":"foo bar"`)

	// Non-empty stream labels
	f(`{"streams":[{"stream":{
	"label1": "value1",
	"label2": "value2"
},"values":[
	["1577836800000000001", "foo bar"],
	["1477836900005000002", "abc"],
	["147.78369e9", "foobar"]
]}]}`, `_time:1577836800000000001 "label1":"value1" "label2":"value2" "_msg":"foo bar"
_time:1477836900005000002 "label1":"value1" "label2":"value2" "_msg":"abc"
_time:147783690000 "label1":"value1" "label2":"value2" "_msg":"foobar"`)

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
}`, `_time:1577836800000000001 "foo":"bar" "a":"b" "_msg":"foo bar"
_time:1577836900005000002 "foo":"bar" "a":"b" "_msg":"abc"
_time:1877836900005000002 "x":"y" "_msg":"yx"`)
}
