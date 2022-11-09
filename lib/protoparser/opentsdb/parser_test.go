package opentsdb

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

	// Missing put prefix
	f("xx")

	// Missing metric
	f("put  111 34")

	// Missing timestamp
	f("put aaa")

	// Missing value
	f("put aaa 1123")

	// Invalid timestamp
	f("put aaa timestamp")
	f("put foobar 3df4 -123456 a=b")

	// Invalid value
	f("put aaa 123 invalid-value")
	f("put foobar 789 -123foo456 a=b")

	// Invalid multiline
	f("put aaa\nbbb 123 34")

	// Invalid tag
	f("put aaa 123 4.5 foo")
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
	// Empty tag
	f("put foobar 789 -123.456 a= b=c =d", &Rows{
		Rows: []Row{{
			Metric:    "foobar",
			Value:     -123.456,
			Timestamp: 789,
			Tags: []Tag{
				{
					Key:   "b",
					Value: "c",
				},
			},
		}},
	})
	// Missing first tag
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3290
	f("put aaa 123 43", &Rows{
		Rows: []Row{{
			Metric:    "aaa",
			Value:     43,
			Timestamp: 123,
		}},
	})
	f("put aaa 123 43 ", &Rows{
		Rows: []Row{{
			Metric:    "aaa",
			Value:     43,
			Timestamp: 123,
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
	// Multi lines with invalid line
	f("put foo 2 0.3 a=b\naaa bbb\nput bar.baz 43 0.34 a=b\n", &Rows{
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

	// Multi spaces
	f("put  foobar 789 -123.456 a=b", &Rows{
		Rows: []Row{{
			Metric:    "foobar",
			Value:     -123.456,
			Timestamp: 789,
			Tags: []Tag{
				{
					Key:   "a",
					Value: "b",
				},
			},
		}},
	})
	f("put foobar  789 -123.456 a=b", &Rows{
		Rows: []Row{{
			Metric:    "foobar",
			Value:     -123.456,
			Timestamp: 789,
			Tags: []Tag{
				{
					Key:   "a",
					Value: "b",
				},
			},
		}},
	})
	f("put foobar 789  -123.456 a=b", &Rows{
		Rows: []Row{{
			Metric:    "foobar",
			Value:     -123.456,
			Timestamp: 789,
			Tags: []Tag{
				{
					Key:   "a",
					Value: "b",
				},
			},
		}},
	})
	f("put foobar 789 -123.456  a=b", &Rows{
		Rows: []Row{{
			Metric:    "foobar",
			Value:     -123.456,
			Timestamp: 789,
			Tags: []Tag{
				{
					Key:   "a",
					Value: "b",
				},
			},
		}},
	})
	f("put foobar 789 -123.456 a=b  c=d", &Rows{
		Rows: []Row{{
			Metric:    "foobar",
			Value:     -123.456,
			Timestamp: 789,
			Tags: []Tag{
				{
					Key:   "a",
					Value: "b",
				},
				{
					Key:   "c",
					Value: "d",
				},
			},
		}},
	})
	// Soace after tags
	f("put foobar 789 -123.456 a=b ", &Rows{
		Rows: []Row{{
			Metric:    "foobar",
			Value:     -123.456,
			Timestamp: 789,
			Tags: []Tag{
				{
					Key:   "a",
					Value: "b",
				},
			},
		}},
	})

}
