package opentsdbhttp

import (
	"github.com/valyala/fastjson"
	"reflect"
	"testing"
)

var parserPool fastjson.ParserPool

func TestRowsUnmarshalFailure(t *testing.T) {
	f := func(s string) {
		t.Helper()
		var rows Rows
		p := parserPool.Get()
		v, err := p.Parse(s)
		if err == nil {
			if err := rows.Unmarshal(v); err == nil {
				parserPool.Put(p)
				t.Fatalf("expecting non-nil error when parsing %q", s)
			}

			// Try again
			if err := rows.Unmarshal(v); err == nil {
				parserPool.Put(p)
				t.Fatalf("expecting non-nil error when parsing %q", s)
			}

		}
		parserPool.Put(p)
	}

	// invalid json
	f("{g")

	// Missing timestamp
	f(`{"metric": "aaa"}`)

	// Missing value
	f(`{"metric": "aaa", "timestamp": 1122}`)

	// Invalid timestamp
	f(`{"metric": "aaa", "timestamp": "tststs"}`)

	// Missing first tag
	f(`{"metric": "aaa", "timestamp": 1122, "value": 33}`)

	// Invalid value
	f(`{"metric": "aaa", "timestamp": 1122, "value": "0.0.0"}`)

	// Invalid multiline
	f(`[{"metric": "aaa", "timestamp": 1122, "value": "trt"}, {"metric": "aaa", "timestamp": 1122, "value": 111}]`)

	// Invalid tag
	f(`{"metric": "aaa", "timestamp": 1122, "value": 0.45, "tags": 1}`)
}

func TestRowsUnmarshalSuccess(t *testing.T) {
	f := func(s string, rowsExpected *Rows) {
		t.Helper()
		var rows Rows

		p := parserPool.Get()
		v, err := p.Parse(s)
		if err != nil {
			t.Fatalf("cannot parse json %q %s", s, err)
		}
		if err := rows.Unmarshal(v); err != nil {
			t.Fatalf("cannot unmarshal %q: %s", v, err)
		}
		if !reflect.DeepEqual(rows.Rows, rowsExpected.Rows) {
			t.Fatalf("unexpected rows;\ngot\n%+v;\nwant\n%+v", rows.Rows, rowsExpected.Rows)
		}

		// Try unmarshaling again
		if err := rows.Unmarshal(v); err != nil {
			t.Fatalf("cannot unmarshal %q: %s", v, err)
		}
		if !reflect.DeepEqual(rows.Rows, rowsExpected.Rows) {
			t.Fatalf("unexpected rows;\ngot\n%+v;\nwant\n%+v", rows.Rows, rowsExpected.Rows)
		}

		rows.Reset()
		if len(rows.Rows) != 0 {
			t.Fatalf("non-empty rows after reset: %+v", rows.Rows)
		}
		parserPool.Put(p)
	}

	// Single line
	f(`{"metric": "foobar", "timestamp": 789, "value": -123.456, "tags": {"a":"b"}}`, &Rows{
		Rows: []Row{{
			Metric:    "foobar",
			Value:     -123.456,
			Timestamp: 789000,
			Tags: []Tag{{
				Key:   "a",
				Value: "b",
			}},
		}},
	})
	// Empty tag value
	f(`{"metric": "foobar", "timestamp": "0", "value": -123.456, "tags": {"a":"", "b":"c"}}`, &Rows{
		Rows: []Row{{
			Metric:    "foobar",
			Value:     -123.456,
			Timestamp: 0,
			Tags: []Tag{
				{
					Key:   "a",
					Value: "",
				},
				{
					Key:   "b",
					Value: "c",
				},
			},
		}},
	})
	// Fractional timestamp that is supported by Akumuli.
	f(`{"metric": "foobar", "timestamp": 1565647665.4, "value": -123.456, "tags": {"a":"b"}}`, &Rows{
		Rows: []Row{{
			Metric:    "foobar",
			Value:     -123.456,
			Timestamp: 1565647665400,
			Tags: []Tag{{
				Key:   "a",
				Value: "b",
			}},
		}},
	})
	f(`{"metric": "foo.bar", "timestamp": "0.0", "value": 123.456, "tags": {"a":"b"}}`, &Rows{
		Rows: []Row{{
			Metric:    "foo.bar",
			Value:     123.456,
			Timestamp: 0,
			Tags: []Tag{{
				Key:   "a",
				Value: "b",
			}},
		}},
	})

	// Tags
	f(`{"metric": "foo", "value": 1, "timestamp": 2, "tags": {"bar":"baz"}}`, &Rows{
		Rows: []Row{{
			Metric: "foo",
			Tags: []Tag{{
				Key:   "bar",
				Value: "baz",
			}},
			Value:     1,
			Timestamp: 2000,
		}},
	})
	f(`{"metric": "foo", "value": 1, "timestamp": 2, "tags": {"bar":"baz", "x": "y"}}`, &Rows{
		Rows: []Row{{
			Metric: "foo",
			Tags: []Tag{
				{
					Key:   "bar",
					Value: "baz",
				},
				{
					Key:   "x",
					Value: "y",
				},
			},
			Value:     1,
			Timestamp: 2000,
		}},
	})
	f(`{"metric": "foo", "value": "1", "timestamp": 2, "tags": {"bar":"baz=aaa", "x": "y"}}`, &Rows{
		Rows: []Row{{
			Metric: "foo",
			Tags: []Tag{
				{
					Key:   "bar",
					Value: "baz=aaa",
				},
				{
					Key:   "x",
					Value: "y",
				},
			},
			Value:     1,
			Timestamp: 2000,
		}},
	})

	// Multi lines
	f(`[{"metric": "foo", "value": "0.3", "timestamp": 2, "tags": {"a":"b"}},
{"metric": "bar.baz", "value": 0.34, "timestamp": 43, "tags": {"a":"b"}}]`, &Rows{
		Rows: []Row{
			{
				Metric:    "foo",
				Value:     0.3,
				Timestamp: 2000,
				Tags: []Tag{{
					Key:   "a",
					Value: "b",
				}},
			},
			{
				Metric:    "bar.baz",
				Value:     0.34,
				Timestamp: 43000,
				Tags: []Tag{{
					Key:   "a",
					Value: "b",
				}},
			},
		},
	})
}
