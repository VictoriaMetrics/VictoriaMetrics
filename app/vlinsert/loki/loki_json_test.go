package loki

import (
	"reflect"
	"strings"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logstorage"
)

func TestProcessJSONRequest(t *testing.T) {
	type item struct {
		ts     int64
		fields []logstorage.Field
	}

	same := func(s string, expected []item) {
		t.Helper()
		r := strings.NewReader(s)
		actual := make([]item, 0)
		n, err := processJSONRequest(r, func(timestamp int64, fields []logstorage.Field) {
			actual = append(actual, item{
				ts:     timestamp,
				fields: fields,
			})
		})

		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}

		if len(actual) != len(expected) || n != len(expected) {
			t.Fatalf("unexpected len(actual)=%d; expecting %d", len(actual), len(expected))
		}

		for i, actualItem := range actual {
			expectedItem := expected[i]
			if actualItem.ts != expectedItem.ts {
				t.Fatalf("unexpected timestamp for item #%d; got %d; expecting %d", i, actualItem.ts, expectedItem.ts)
			}
			if !reflect.DeepEqual(actualItem.fields, expectedItem.fields) {
				t.Fatalf("unexpected fields for item #%d; got %v; expecting %v", i, actualItem.fields, expectedItem.fields)
			}
		}
	}

	fail := func(s string) {
		t.Helper()
		r := strings.NewReader(s)
		actual := make([]item, 0)
		_, err := processJSONRequest(r, func(timestamp int64, fields []logstorage.Field) {
			actual = append(actual, item{
				ts:     timestamp,
				fields: fields,
			})
		})

		if err == nil {
			t.Fatalf("expected to fail with body: %q", s)
		}

	}

	same(`{"streams":[{"stream":{"foo":"bar"},"values":[["1577836800000000000","baz"]]}]}`, []item{
		{
			ts: 1577836800000000000,
			fields: []logstorage.Field{
				{
					Name:  "foo",
					Value: "bar",
				},
				{
					Name:  "_msg",
					Value: "baz",
				},
			},
		},
	})

	fail(``)
	fail(`{"streams":[{"stream":{"foo" = "bar"},"values":[["1577836800000000000","baz"]]}]}`)
	fail(`{"streams":[{"stream":{"foo": "bar"}`)
}

func Test_parseLokiTimestamp(t *testing.T) {
	f := func(s string, expected int64) {
		t.Helper()
		actual, err := parseLokiTimestamp(s)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		if actual != expected {
			t.Fatalf("unexpected timestamp; got %d; expecting %d", actual, expected)
		}
	}

	f("1687510468000000000", 1687510468000000000)
	f("1577836800000000000", 1577836800000000000)
}
