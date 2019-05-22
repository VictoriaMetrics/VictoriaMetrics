package opentsdb

import (
	"reflect"
	"testing"
)

func TestRowsUnmarshalFailure(t *testing.T) {
	f := func(s string) {
		t.Helper()
		var rows Rows
		if err := rows.Unmarshal(s); err == nil {
			t.Fatalf("expecting non-nil error when parsing %q", s)
		}

		// Try again
		if err := rows.Unmarshal(s); err == nil {
			t.Fatalf("expecting non-nil error when parsing %q", s)
		}
	}

	// Missing put prefix
	f("xx")

	// Missing timestamp
	f("put aaa")

	// Missing value
	f("put aaa 1123")

	// Invalid timestamp
	f("put aaa timestamp")

	// Missing first tag
	f("put aaa 123 43")

	// Invalid value
	f("put aaa 123 invalid-value")

	// Invalid multiline
	f("put aaa\nbbb 123 34")

	// Invalid tag
	f("put aaa 123 4.5 foo")
	f("put aaa 123 4.5 =")
	f("put aaa 123 4.5 =foo")
	f("put aaa 123 4.5 =foo a=b")
}

func TestRowsUnmarshalSuccess(t *testing.T) {
	f := func(s string, rowsExpected *Rows) {
		t.Helper()
		var rows Rows
		if err := rows.Unmarshal(s); err != nil {
			t.Fatalf("cannot unmarshal %q: %s", s, err)
		}
		if !reflect.DeepEqual(rows.Rows, rowsExpected.Rows) {
			t.Fatalf("unexpected rows;\ngot\n%+v;\nwant\n%+v", rows.Rows, rowsExpected.Rows)
		}

		// Try unmarshaling again
		if err := rows.Unmarshal(s); err != nil {
			t.Fatalf("cannot unmarshal %q: %s", s, err)
		}
		if !reflect.DeepEqual(rows.Rows, rowsExpected.Rows) {
			t.Fatalf("unexpected rows;\ngot\n%+v;\nwant\n%+v", rows.Rows, rowsExpected.Rows)
		}

		rows.Reset()
		if len(rows.Rows) != 0 {
			t.Fatalf("non-empty rows after reset: %+v", rows.Rows)
		}
	}

	// Empty line
	f("", &Rows{})
	f("\n\n", &Rows{})

	// Single line
	f("put foobar 789 -123.456 a=b", &Rows{
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
	// Empty tag value
	f("put foobar 789 -123.456 a= b=c", &Rows{
		Rows: []Row{{
			Metric:    "foobar",
			Value:     -123.456,
			Timestamp: 789,
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
	f("put foobar 789.4 -123.456 a=b", &Rows{
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
	f("put foo.bar 789 123.456 a=b\n", &Rows{
		Rows: []Row{{
			Metric:    "foo.bar",
			Value:     123.456,
			Timestamp: 789,
			Tags: []Tag{{
				Key:   "a",
				Value: "b",
			}},
		}},
	})

	// Tags
	f("put foo 2 1 bar=baz", &Rows{
		Rows: []Row{{
			Metric: "foo",
			Tags: []Tag{{
				Key:   "bar",
				Value: "baz",
			}},
			Value:     1,
			Timestamp: 2,
		}},
	})
	f("put foo 2 1 bar=baz x=y", &Rows{
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
	f("put foo 2 1 bar=baz=aaa x=y", &Rows{
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
			Timestamp: 2,
		}},
	})

	// Multi lines
	f("put foo 2 0.3 a=b\nput bar.baz 43 0.34 a=b\n", &Rows{
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
