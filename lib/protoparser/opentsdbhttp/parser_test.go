package opentsdbhttp

import (
	"reflect"
	"testing"
)

func TestRowsUnmarshalFailure(t *testing.T) {
	f := func(s string) {
		t.Helper()
		var rows Rows
		p := GetParser()
		defer PutParser(p)
		v, err := p.Parse(s)
		if err != nil {
			// Expected JSON parser error
			return
		}
		// Verify OpenTSDB body parsing error
		rows.Unmarshal(v)
		if len(rows.Rows) != 0 {
			t.Fatalf("unexpected number of rows parsed; got %d; want 0", len(rows.Rows))
		}
		// Try again
		rows.Unmarshal(v)
		if len(rows.Rows) != 0 {
			t.Fatalf("unexpected number of rows parsed; got %d; want 0", len(rows.Rows))
		}
	}

	// invalid json
	f("{g")

	// Invalid json type
	f(`1`)
	f(`"foo"`)
	f(`[1,2]`)
	f(`null`)

	// Incomplete object
	f(`{}`)
	f(`{"metric": "aaa"}`)
	f(`{"metric": "aaa", "timestamp": 1122}`)
	f(`{"metric": "aaa", "timestamp": "tststs"}`)
	f(`{"timestamp": 1122, "value": 33}`)
	f(`{"value": 33}`)
	f(`{"value": 33, "tags": {"fooo":"bar"}}`)

	// Invalid value
	f(`{"metric": "aaa", "timestamp": 1122, "value": "0.0.0"}`)

	// Invalid metric type
	f(`{"metric": "", "timestamp": 1122, "value": 0.45, "tags": {"foo": "bar"}}`)
	f(`{"metric": ["aaa"], "timestamp": 1122, "value": 0.45, "tags": {"foo": "bar"}}`)
	f(`{"metric": {"aaa":1}, "timestamp": 1122, "value": 0.45, "tags": {"foo": "bar"}}`)
	f(`{"metric": 1, "timestamp": 1122, "value": 0.45, "tags": {"foo": "bar"}}`)

	// Invalid timestamp type
	f(`{"metric": "aaa", "timestamp": "foobar", "value": 0.45, "tags": {"foo": "bar"}}`)
	f(`{"metric": "aaa", "timestamp": [1,2], "value": 0.45, "tags": {"foo": "bar"}}`)
	f(`{"metric": "aaa", "timestamp": {"a":1}, "value": 0.45, "tags": {"foo": "bar"}}`)

	// Invalid value type
	f(`{"metric": "aaa", "timestamp": 1122, "value": [0,1], "tags": {"foo":"bar"}}`)
	f(`{"metric": "aaa", "timestamp": 1122, "value": {"a":1}, "tags": {"foo":"bar"}}`)
	f(`{"metric": "aaa", "timestamp": 1122, "value": "foobar", "tags": {"foo":"bar"}}`)

	// Invalid tags type
	f(`{"metric": "aaa", "timestamp": 1122, "value": 0.45, "tags": 1}`)
	f(`{"metric": "aaa", "timestamp": 1122, "value": 0.45, "tags": [1,2]}`)
	f(`{"metric": "aaa", "timestamp": 1122, "value": 0.45, "tags": "foo"}`)

	// Invalid tag value type
	f(`{"metric": "aaa", "timestamp": 1122, "value": 0.45, "tags": {"foo": ["bar"]}}`)
	f(`{"metric": "aaa", "timestamp": 1122, "value": 0.45, "tags": {"foo": {"bar":"baz"}}}`)
	f(`{"metric": "aaa", "timestamp": 1122, "value": 0.45, "tags": {"foo": 1}}`)

	// Invalid multiline
	f(`[{"metric": "aaa", "timestamp": 1122, "value": "trt", "tags":{"foo":"bar"}}, {"metric": "aaa", "timestamp": [1122], "value": 111}]`)
}

func TestRowsUnmarshalSuccess(t *testing.T) {
	f := func(s string, rowsExpected *Rows) {
		t.Helper()
		var rows Rows

		p := GetParser()
		defer PutParser(p)
		v, err := p.Parse(s)
		if err != nil {
			t.Fatalf("cannot parse json %s: %s", s, err)
		}
		rows.Unmarshal(v)
		if !reflect.DeepEqual(rows.Rows, rowsExpected.Rows) {
			t.Fatalf("unexpected rows;\ngot\n%+v;\nwant\n%+v", rows.Rows, rowsExpected.Rows)
		}

		// Try unmarshaling again
		rows.Unmarshal(v)
		if !reflect.DeepEqual(rows.Rows, rowsExpected.Rows) {
			t.Fatalf("unexpected rows;\ngot\n%+v;\nwant\n%+v", rows.Rows, rowsExpected.Rows)
		}

		rows.Reset()
		if len(rows.Rows) != 0 {
			t.Fatalf("non-empty rows after reset: %+v", rows.Rows)
		}
	}

	// Normal line
	f(`{"metric": "foobar", "timestamp": 789, "value": -123.456, "tags": {"a":"b"}}`, &Rows{
		Rows: []Row{{
			Metric:    "foobar",
			Value:     -123.456,
			Timestamp: 789,
			Tags: []Tag{{
				Key:   "a",
				Value: "b",
			}},
		}},
	})
	// Timestamp as string
	f(`{"metric": "foobar", "timestamp": "1789", "value": -123.456, "tags": {"a":"b"}}`, &Rows{
		Rows: []Row{{
			Metric:    "foobar",
			Value:     -123.456,
			Timestamp: 1789,
			Tags: []Tag{{
				Key:   "a",
				Value: "b",
			}},
		}},
	})
	// Timestamp as float64 (it is truncated to integer)
	f(`{"metric": "foobar", "timestamp": 17.89, "value": -123.456, "tags": {"a":"b"}}`, &Rows{
		Rows: []Row{{
			Metric:    "foobar",
			Value:     -123.456,
			Timestamp: 17,
			Tags: []Tag{{
				Key:   "a",
				Value: "b",
			}},
		}},
	})
	// Empty tags
	f(`{"metric": "foobar", "timestamp": 789, "value": -123.456, "tags": {}}`, &Rows{
		Rows: []Row{{
			Metric:    "foobar",
			Value:     -123.456,
			Timestamp: 789,
			Tags:      nil,
		}},
	})
	// Missing tags
	f(`{"metric": "foobar", "timestamp": 789, "value": -123.456}`, &Rows{
		Rows: []Row{{
			Metric:    "foobar",
			Value:     -123.456,
			Timestamp: 789,
			Tags:      nil,
		}},
	})
	// Empty tag value
	f(`{"metric": "foobar", "timestamp": 123, "value": -123.456, "tags": {"a":"", "b":"c", "": "d"}}`, &Rows{
		Rows: []Row{{
			Metric:    "foobar",
			Value:     -123.456,
			Timestamp: 123,
			Tags: []Tag{
				{
					Key:   "b",
					Value: "c",
				},
			},
		}},
	})
	// Value as string
	f(`{"metric": "foobar", "timestamp": 789, "value": "-12.456", "tags": {"a":"b"}}`, &Rows{
		Rows: []Row{{
			Metric:    "foobar",
			Value:     -12.456,
			Timestamp: 789,
			Tags: []Tag{{
				Key:   "a",
				Value: "b",
			}},
		}},
	})
	// Missing timestamp
	f(`{"metric": "foobar", "value": "-12.456", "tags": {"a":"b"}}`, &Rows{
		Rows: []Row{{
			Metric:    "foobar",
			Value:     -12.456,
			Timestamp: 0,
			Tags: []Tag{{
				Key:   "a",
				Value: "b",
			}},
		}},
	})

	// Multiple tags
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
			Timestamp: 2,
		}},
	})

	// Multi lines
	f(`[{"metric": "foo", "value": "0.3", "timestamp": 2, "tags": {"a":"b"}},
{"metric": "bar.baz", "value": 0.34, "timestamp": 43, "tags": {"a":"b"}}]`, &Rows{
		Rows: []Row{
			{
				Metric:    "foo",
				Value:     0.3,
				Timestamp: 2,
				Tags: []Tag{{
					Key:   "a",
					Value: "b",
				}},
			},
			{
				Metric:    "bar.baz",
				Value:     0.34,
				Timestamp: 43,
				Tags: []Tag{{
					Key:   "a",
					Value: "b",
				}},
			},
		},
	})
}
