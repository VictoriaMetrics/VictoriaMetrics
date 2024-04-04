package stream

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"reflect"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/newrelic"
)

func TestParseFailure(t *testing.T) {
	f := func(req string) {
		t.Helper()

		callback := func(_ []newrelic.Row) error {
			panic(fmt.Errorf("unexpected call into callback"))
		}
		r := bytes.NewReader([]byte(req))
		if err := Parse(r, false, callback); err == nil {
			t.Fatalf("expecting non-empty error")
		}
	}
	f("")
	f("foo")
	f("{}")
	f("[1,2,3]")
}

func TestParseSuccess(t *testing.T) {
	f := func(req string, expectedRows []newrelic.Row) {
		t.Helper()

		callback := func(rows []newrelic.Row) error {
			if !reflect.DeepEqual(rows, expectedRows) {
				return fmt.Errorf("unexpected rows\ngot\n%v\nwant\n%v", rows, expectedRows)
			}
			return nil
		}

		// Parse from uncompressed reader
		r := bytes.NewReader([]byte(req))
		if err := Parse(r, false, callback); err != nil {
			t.Fatalf("unexpected error when parsing uncompressed request: %s", err)
		}

		var bb bytes.Buffer
		zw := gzip.NewWriter(&bb)
		if _, err := zw.Write([]byte(req)); err != nil {
			t.Fatalf("cannot compress request: %s", err)
		}
		if err := zw.Close(); err != nil {
			t.Fatalf("cannot close compressed writer: %s", err)
		}
		if err := Parse(&bb, true, callback); err != nil {
			t.Fatalf("unexpected error when parsing compressed request: %s", err)
		}
	}

	f("[]", nil)
	f(`[{"Events":[]}]`, nil)
	f(`[{
      "EntityID":28257883748326179,
      "IsAgent":true,
      "Events":[
        {
          "eventType":"SystemSample",
          "timestamp":1690286061,
          "entityKey":"macbook-pro.local",
          "dc": "1",
          "diskWritesPerSecond":-34.21,
          "uptime":762376
        }
      ],
      "ReportingAgentID":28257883748326179
}]`, []newrelic.Row{
		{
			Tags: []newrelic.Tag{
				{
					Key:   []byte("eventType"),
					Value: []byte("SystemSample"),
				},
				{
					Key:   []byte("entityKey"),
					Value: []byte("macbook-pro.local"),
				},
				{
					Key:   []byte("dc"),
					Value: []byte("1"),
				},
			},
			Samples: []newrelic.Sample{
				{
					Name:  []byte("diskWritesPerSecond"),
					Value: -34.21,
				},
				{
					Name:  []byte("uptime"),
					Value: 762376,
				},
			},
			Timestamp: 1690286061000,
		},
	})
}
