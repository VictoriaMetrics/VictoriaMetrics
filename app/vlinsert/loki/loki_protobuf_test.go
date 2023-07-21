package loki

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logstorage"
	"github.com/golang/snappy"
)

func TestParseProtobufRequestSuccess(t *testing.T) {
	f := func(s string, resultExpected string) {
		t.Helper()
		var pr PushRequest
		n, err := parseJSONRequest([]byte(s), func(timestamp int64, fields []logstorage.Field) {
			msg := ""
			for _, f := range fields {
				if f.Name == "_msg" {
					msg = f.Value
				}
			}
			var a []string
			for _, f := range fields {
				if f.Name == "_msg" {
					continue
				}
				item := fmt.Sprintf("%s=%q", f.Name, f.Value)
				a = append(a, item)
			}
			labels := "{" + strings.Join(a, ", ") + "}"
			pr.Streams = append(pr.Streams, Stream{
				Labels: labels,
				Entries: []Entry{
					{
						Timestamp: time.Unix(0, timestamp),
						Line:      msg,
					},
				},
			})
		})
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		if n != len(pr.Streams) {
			t.Fatalf("unexpected number of streams; got %d; want %d", len(pr.Streams), n)
		}

		data, err := pr.Marshal()
		if err != nil {
			t.Fatalf("unexpected error when marshaling PushRequest: %s", err)
		}
		encodedData := snappy.Encode(nil, data)

		var lines []string
		n, err = parseProtobufRequest(encodedData, func(timestamp int64, fields []logstorage.Field) {
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

func TestParsePromLabelsSuccess(t *testing.T) {
	f := func(s string) {
		t.Helper()
		fields, err := parsePromLabels(nil, s)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}

		var a []string
		for _, f := range fields {
			a = append(a, fmt.Sprintf("%s=%q", f.Name, f.Value))
		}
		result := "{" + strings.Join(a, ", ") + "}"
		if result != s {
			t.Fatalf("unexpected result;\ngot\n%s\nwant\n%s", result, s)
		}
	}

	f("{}")
	f(`{foo="bar"}`)
	f(`{foo="bar", baz="x", y="z"}`)
	f(`{foo="ba\"r\\z\n", a="", b="\"\\"}`)
}

func TestParsePromLabelsFailure(t *testing.T) {
	f := func(s string) {
		t.Helper()
		fields, err := parsePromLabels(nil, s)
		if err == nil {
			t.Fatalf("expecting non-nil error")
		}
		if len(fields) > 0 {
			t.Fatalf("unexpected non-empty fields: %s", fields)
		}
	}

	f("")
	f("{")
	f(`{foo}`)
	f(`{foo=bar}`)
	f(`{foo="bar}`)
	f(`{foo="ba\",r}`)
	f(`{foo="bar" baz="aa"}`)
	f(`foobar`)
	f(`foo{bar="baz"}`)
}
