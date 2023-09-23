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

	// // With tab as separator
	// // See https://github.com/grobian/carbon-c-relay/commit/f3ffe6cc2b52b07d14acbda649ad3fd6babdd528
	// f("foo.baz\t125.456\t1789\n", &Rows{
	// 	Rows: []Row{{
	// 		Metric:    "foo.baz",
	// 		Value:     125.456,
	// 		Timestamp: 1789,
	// 	}},
	// })
	// // With tab as separator and tags
	// f("foo;baz=bar;bb=;y=x;=z\t1\t2", &Rows{
	// 	Rows: []Row{{
	// 		Metric: "foo",
	// 		Tags: []Tag{
	// 			{
	// 				Key:   "baz",
	// 				Value: "bar",
	// 			},
	// 			{
	// 				Key:   "y",
	// 				Value: "x",
	// 			},
	// 		},
	// 		Value:     1,
	// 		Timestamp: 2,
	// 	}},
	// })

	// // Whitespace after the timestamp
	// // See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/1865
	// f("foo.baz 125 1789 \na 1.34 567\t  ", &Rows{
	// 	Rows: []Row{
	// 		{
	// 			Metric:    "foo.baz",
	// 			Value:     125,
	// 			Timestamp: 1789,
	// 		},
	// 		{
	// 			Metric:    "a",
	// 			Value:     1.34,
	// 			Timestamp: 567,
	// 		},
	// 	},
	// })

	// // Multiple whitespaces as separators
	// f("foo.baz \t125  1789 \t\n", &Rows{
	// 	Rows: []Row{{
	// 		Metric:    "foo.baz",
	// 		Value:     125,
	// 		Timestamp: 1789,
	// 	}},
	// })
}
