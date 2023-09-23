package statsd

import (
	"reflect"
	"testing"
)

// TODO: add specs for unmarshalTags
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
			t.Fatalf("unexpected rows on second unmarshal;\ngot\n%+v;\nwant\n%+v", rows.Rows, rowsExpected.Rows)
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
	f(" 123:455", &Rows{
		Rows: []Row{{
			Metric: "123",
			Value:  455,
		}},
	})
	f("123:455 |c", &Rows{
		Rows: []Row{{
			Metric: "123",
			Value:  455,
		}},
	})
	f("foobar:-123.456|c", &Rows{
		Rows: []Row{{
			Metric: "foobar",
			Value:  -123.456,
		}},
	})
	f("foo.bar:123.456|c\n", &Rows{
		Rows: []Row{{
			Metric: "foo.bar",
			Value:  123.456,
		}},
	})

	// with sample rate
	f("foo.bar:1|c|@0.1", &Rows{
		Rows: []Row{{
			Metric: "foo.bar",
			Value:  1,
		}},
	})

	// without specifying metric unit
	f("foo.bar:123", &Rows{
		Rows: []Row{{
			Metric: "foo.bar",
			Value:  123,
		}},
	})
	// without specifying metric unit but with tags
	f("foo.bar:123|#foo:bar", &Rows{
		Rows: []Row{{
			Metric: "foo.bar",
			Value:  123,
			Tags: []Tag{
				{
					Key:   "foo",
					Value: "bar",
				},
			},
		}},
	})

	f("foo.bar:123.456|c|#foo:bar,qwe:asd", &Rows{
		Rows: []Row{{
			Metric: "foo.bar",
			Value:  123.456,
			Tags: []Tag{
				{
					Key:   "foo",
					Value: "bar",
				},
				{
					Key:   "qwe",
					Value: "asd",
				},
			},
		}},
	})

	// // Whitespace in metric name, tag name and tag value
	// // See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3102
	f("s a:1|c|#ta g1:aaa1,tag2:bb b2", &Rows{
		Rows: []Row{{
			Metric: "s a",
			Value:  1,
			Tags: []Tag{
				{
					Key:   "ta g1",
					Value: "aaa1",
				},
				{
					Key:   "tag2",
					Value: "bb b2",
				},
			},
		}},
	})

	// Tags
	// TODO: fix empty tags tests
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1100
	// f("foo:1|c", &Rows{
	// 	Rows: []Row{{
	// 		Metric: "foo",
	// 		Tags:   []Tag{},
	// 		Value:  1,
	// 	}},
	// })
	// f("foo; 1 2", &Rows{
	// 	Rows: []Row{{
	// 		Metric:    "foo",
	// 		Tags:      []Tag{},
	// 		Value:     1,
	// 		Timestamp: 2,
	// 	}},
	// })
	// // Empty tag name or value
	// f("foo;bar 1 2", &Rows{
	// 	Rows: []Row{{
	// 		Metric:    "foo",
	// 		Tags:      []Tag{},
	// 		Value:     1,
	// 		Timestamp: 2,
	// 	}},
	// })
	f("foo:1|#bar:baz,aa:,x:y,:z", &Rows{
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
			Value: 1,
		}},
	})

	// Multi lines
	f("foo:0.3|c\naaa:3|g\nbar.baz:0.34|c\n", &Rows{
		Rows: []Row{
			{
				Metric: "foo",
				Value:  0.3,
			},
			{
				Metric: "aaa",
				Value:  3,
			},
			{
				Metric: "bar.baz",
				Value:  0.34,
			},
		},
	})

	// Multi lines with invalid line
	f("foo:0.3|c\naaa\nbar.baz:0.34\n", &Rows{
		Rows: []Row{
			{
				Metric: "foo",
				Value:  0.3,
			},
			{
				Metric: "bar.baz",
				Value:  0.34,
			},
		},
	})

	// Whitespace after at the end
	f("foo.baz:125|c\na:1.34\t  ", &Rows{
		Rows: []Row{
			{
				Metric: "foo.baz",
				Value:  125,
			},
			{
				Metric: "a",
				Value:  1.34,
			},
		},
	})
}
