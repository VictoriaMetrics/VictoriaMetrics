package graphite

import (
	"reflect"
	"testing"
)

func TestRowsUnmarshalFailure(t *testing.T) {
	f := func(s string) {
		t.Helper()
		var rows Rows
		rows.Unmarshal(s)
		if len(rows.Rows) != 0 {
			t.Fatalf("unexpected number of rows parsed; got %d; want 0", len(rows.Rows))
		}

		// Try again
		rows.Unmarshal(s)
		if len(rows.Rows) != 0 {
			t.Fatalf("unexpected number of rows parsed; got %d; want 0", len(rows.Rows))
		}
	}

	// Missing metric
	f(" 123 455")

	// Missing value
	f("aaa")

	// missing tag
	f("aa; 12 34")

	// missing tag value
	f("aa;bb 23 34")
}

func TestRowsUnmarshalSuccess(t *testing.T) {
	f := func(s string, rowsExpected *Rows) {
		t.Helper()
		var rows Rows
		rows.Unmarshal(s)
		if !reflect.DeepEqual(rows.Rows, rowsExpected.Rows) {
			t.Fatalf("unexpected rows;\ngot\n%+v;\nwant\n%+v", rows.Rows, rowsExpected.Rows)
		}

		// Try unmarshaling again
		rows.Unmarshal(s)
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
	f("\r", &Rows{})
	f("\n\n", &Rows{})
	f("\n\r\n", &Rows{})

	// Single line
	f("foobar -123.456 789", &Rows{
		Rows: []Row{{
			Metric:    "foobar",
			Value:     -123.456,
			Timestamp: 789,
		}},
	})
	f("foo.bar 123.456 789\n", &Rows{
		Rows: []Row{{
			Metric:    "foo.bar",
			Value:     123.456,
			Timestamp: 789,
		}},
	})

	// Missing timestamp
	f("aaa 1123", &Rows{
		Rows: []Row{{
			Metric: "aaa",
			Value:  1123,
		}},
	})

	// Timestamp bigger than 1<<31
	f("aaa 1123 429496729600", &Rows{
		Rows: []Row{{
			Metric:    "aaa",
			Value:     1123,
			Timestamp: 429496729600,
		}},
	})

	// Tags
	f("foo;bar=baz 1 2", &Rows{
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
	// Empty tags
	f("foo;bar=baz;aa=;x=y;=z 1 2", &Rows{
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
	f("foo 0.3 2\naaa 3\nbar.baz 0.34 43\n", &Rows{
		Rows: []Row{
			{
				Metric:    "foo",
				Value:     0.3,
				Timestamp: 2,
			},
			{
				Metric: "aaa",
				Value:  3,
			},
			{
				Metric:    "bar.baz",
				Value:     0.34,
				Timestamp: 43,
			},
		},
	})

	// Multi lines with invalid line
	f("foo 0.3 2\naaa\nbar.baz 0.34 43\n", &Rows{
		Rows: []Row{
			{
				Metric:    "foo",
				Value:     0.3,
				Timestamp: 2,
			},
			{
				Metric:    "bar.baz",
				Value:     0.34,
				Timestamp: 43,
			},
		},
	})
}
