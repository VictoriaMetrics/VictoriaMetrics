package statsd

import (
	"reflect"
	"testing"
)

func TestUnmarshalTagsSuccess(t *testing.T) {
	f := func(dst []Tag, s string, tagsPoolExpected []Tag) {
		t.Helper()

		tagsPool := unmarshalTags(dst, s)
		if !reflect.DeepEqual(tagsPool, tagsPoolExpected) {
			t.Fatalf("unexpected tags;\ngot\n%+v;\nwant\n%+v", tagsPool, tagsPoolExpected)
		}

		// Try unmarshaling again
		tagsPool = unmarshalTags(dst, s)
		if !reflect.DeepEqual(tagsPool, tagsPoolExpected) {
			t.Fatalf("unexpected tags on second unmarshal;\ngot\n%+v;\nwant\n%+v", tagsPool, tagsPoolExpected)
		}
	}

	f([]Tag{}, "foo:bar", []Tag{
		{
			Key:   "foo",
			Value: "bar",
		},
	})

	f([]Tag{}, "foo:bar,qwe:123", []Tag{
		{
			Key:   "foo",
			Value: "bar",
		},
		{
			Key:   "qwe",
			Value: "123",
		},
	})

	f([]Tag{}, "foo.qwe:bar", []Tag{
		{
			Key:   "foo.qwe",
			Value: "bar",
		},
	})

	f([]Tag{}, "foo:10", []Tag{
		{
			Key:   "foo",
			Value: "10",
		},
	})

	f([]Tag{}, "foo: _qwe", []Tag{
		{
			Key:   "foo",
			Value: " _qwe",
		},
	})

	f([]Tag{}, "foo:qwe    ", []Tag{
		{
			Key:   "foo",
			Value: "qwe    ",
		},
	})

	f([]Tag{}, "foo  asd:qwe    ", []Tag{
		{
			Key:   "foo  asd",
			Value: "qwe    ",
		},
	})

	f([]Tag{}, "foo:var:123", []Tag{
		{
			Key:   "foo",
			Value: "var:123",
		},
	})

	// invalid tags
	f([]Tag{}, ":bar", []Tag{})
	f([]Tag{}, "foo:", []Tag{})
	f([]Tag{}, "   ", []Tag{})
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
	f(" 123:455|c", &Rows{
		Rows: []Row{{
			Metric: "123",
			Values: []float64{455},
			Tags: []Tag{
				{
					Key:   statsdTypeTagName,
					Value: "c",
				},
			},
		}},
	})
	// multiple values statsd dog v1.1
	f(" 123:455:456|c", &Rows{
		Rows: []Row{{
			Metric: "123",
			Values: []float64{455, 456},
			Tags: []Tag{
				{
					Key:   statsdTypeTagName,
					Value: "c",
				},
			},
		}},
	})
	f("123:455 |c", &Rows{
		Rows: []Row{{
			Metric: "123",
			Values: []float64{455},
			Tags: []Tag{
				{
					Key:   statsdTypeTagName,
					Value: "c",
				},
			},
		}},
	})
	f("foobar:-123.456|c", &Rows{
		Rows: []Row{{
			Metric: "foobar",
			Values: []float64{-123.456},
			Tags: []Tag{
				{
					Key:   statsdTypeTagName,
					Value: "c",
				},
			},
		}},
	})
	f("foo.bar:123.456|c\n", &Rows{
		Rows: []Row{{
			Metric: "foo.bar",
			Values: []float64{123.456},
			Tags: []Tag{
				{
					Key:   statsdTypeTagName,
					Value: "c",
				},
			},
		}},
	})

	// with sample rate
	f("foo.bar:1|c|@0.1", &Rows{
		Rows: []Row{{
			Metric: "foo.bar",
			Values: []float64{1},
			Tags: []Tag{
				{
					Key:   statsdTypeTagName,
					Value: "c",
				},
			},
		}},
	})

	// without specifying metric unit
	f("foo.bar:123|h", &Rows{
		Rows: []Row{{
			Metric: "foo.bar",
			Values: []float64{123},
			Tags: []Tag{
				{
					Key:   statsdTypeTagName,
					Value: "h",
				},
			},
		}},
	})
	// without specifying metric unit but with tags
	f("foo.bar:123|s|#foo:bar", &Rows{
		Rows: []Row{{
			Metric: "foo.bar",
			Values: []float64{123},
			Tags: []Tag{
				{
					Key:   statsdTypeTagName,
					Value: "s",
				},

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
			Values: []float64{123.456},
			Tags: []Tag{
				{
					Key:   statsdTypeTagName,
					Value: "c",
				},

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

	// Whitespace in metric name, tag name and tag value
	f("s a:1|c|#ta g1:aaa1,tag2:bb b2", &Rows{
		Rows: []Row{{
			Metric: "s a",
			Values: []float64{1},
			Tags: []Tag{
				{
					Key:   statsdTypeTagName,
					Value: "c",
				},
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
	f("foo:1|c", &Rows{
		Rows: []Row{{
			Metric: "foo",
			Values: []float64{1},
			Tags: []Tag{
				{
					Key:   statsdTypeTagName,
					Value: "c",
				},
			},
		}},
	})
	// Empty tag name
	f("foo:1|d|#:123", &Rows{
		Rows: []Row{{
			Metric: "foo",
			Tags: []Tag{
				{
					Key:   statsdTypeTagName,
					Value: "d",
				},
			},
			Values: []float64{1},
		}},
	})
	// Empty tag value
	f("foo:1|s|#tag1:", &Rows{
		Rows: []Row{{
			Metric: "foo",
			Tags: []Tag{
				{
					Key:   statsdTypeTagName,
					Value: "s",
				},
			},
			Values: []float64{1},
		}},
	})
	f("foo:1|d|#bar:baz,aa:,x:y,:z", &Rows{
		Rows: []Row{{
			Metric: "foo",
			Tags: []Tag{
				{
					Key:   statsdTypeTagName,
					Value: "d",
				},
				{
					Key:   "bar",
					Value: "baz",
				},
				{
					Key:   "x",
					Value: "y",
				},
			},
			Values: []float64{1},
		}},
	})

	// Multi lines
	f("foo:0.3|c\naaa:3|g\nbar.baz:0.34|c\n", &Rows{
		Rows: []Row{
			{
				Metric: "foo",
				Values: []float64{0.3},
				Tags: []Tag{
					{
						Key:   statsdTypeTagName,
						Value: "c",
					},
				},
			},
			{
				Metric: "aaa",
				Values: []float64{3},
				Tags: []Tag{
					{
						Key:   statsdTypeTagName,
						Value: "g",
					},
				},
			},
			{
				Metric: "bar.baz",
				Values: []float64{0.34},
				Tags: []Tag{
					{
						Key:   statsdTypeTagName,
						Value: "c",
					},
				},
			},
		},
	})

	f("foo:0.3|c|#tag1:1,tag2:2\naaa:3|g|#tag3:3,tag4:4", &Rows{
		Rows: []Row{
			{
				Metric: "foo",
				Values: []float64{0.3},
				Tags: []Tag{
					{
						Key:   statsdTypeTagName,
						Value: "c",
					},

					{
						Key:   "tag1",
						Value: "1",
					},
					{
						Key:   "tag2",
						Value: "2",
					},
				},
			},
			{
				Metric: "aaa",
				Values: []float64{3},
				Tags: []Tag{
					{
						Key:   statsdTypeTagName,
						Value: "g",
					},

					{
						Key:   "tag3",
						Value: "3",
					},
					{
						Key:   "tag4",
						Value: "4",
					},
				},
			},
		},
	})

	// Multi lines with invalid line
	f("foo:0.3|c\naaa\nbar.baz:0.34|c\n", &Rows{
		Rows: []Row{
			{
				Metric: "foo",
				Values: []float64{0.3},
				Tags: []Tag{
					{
						Key:   statsdTypeTagName,
						Value: "c",
					},
				},
			},
			{
				Metric: "bar.baz",
				Values: []float64{0.34},
				Tags: []Tag{
					{
						Key:   statsdTypeTagName,
						Value: "c",
					},
				},
			},
		},
	})

	// Whitespace after at the end
	f("foo.baz:125|c\na:1.34|h\t  ", &Rows{
		Rows: []Row{
			{
				Metric: "foo.baz",
				Values: []float64{125},
				Tags: []Tag{
					{
						Key:   statsdTypeTagName,
						Value: "c",
					},
				},
			},
			{
				Metric: "a",
				Values: []float64{1.34},
				Tags: []Tag{
					{
						Key:   statsdTypeTagName,
						Value: "h",
					},
				},
			},
		},
	})

	// ignores sample rate
	f("foo.baz:125|c|@0.5|#tag1:12", &Rows{
		Rows: []Row{
			{
				Metric: "foo.baz",
				Values: []float64{125},
				Tags: []Tag{
					{
						Key:   statsdTypeTagName,
						Value: "c",
					},
					{
						Key:   "tag1",
						Value: "12",
					},
				},
			},
		},
	})
	// ignores container and timestamp
	f("foo.baz:125|c|@0.5|#tag1:12|c:83c0a99c0a54c0c187f461c7980e9b57f3f6a8b0c918c8d93df19a9de6f3fe1d|T1656581400", &Rows{
		Rows: []Row{
			{
				Metric: "foo.baz",
				Values: []float64{125},
				Tags: []Tag{
					{
						Key:   statsdTypeTagName,
						Value: "c",
					},
					{
						Key:   "tag1",
						Value: "12",
					},
				},
			},
		},
	})
}

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

	// random string
	f("aaa")

	// empty value
	f("foo:")

	// empty metric name
	f(":12")

	// empty type
	f("foo:12")

	// bad values
	f("foo:12:baz|c")
}
