package prometheus

import (
	"reflect"
	"testing"
)

func TestPrevBackslashesCount(t *testing.T) {
	f := func(s string, nExpected int) {
		t.Helper()
		n := prevBackslashesCount(s)
		if n != nExpected {
			t.Fatalf("unexpected value returned from prevBackslashesCount(%q); got %d; want %d", s, n, nExpected)
		}
	}
	f(``, 0)
	f(`foo`, 0)
	f(`\`, 1)
	f(`\\`, 2)
	f(`\\\`, 3)
	f(`\\\a`, 0)
	f(`foo\bar`, 0)
	f(`foo\\`, 2)
	f(`\\foo\`, 1)
	f(`\\foo\\\\`, 4)
}

func TestFindClosingQuote(t *testing.T) {
	f := func(s string, nExpected int) {
		t.Helper()
		n := findClosingQuote(s)
		if n != nExpected {
			t.Fatalf("unexpected value returned from findClosingQuote(%q); got %d; want %d", s, n, nExpected)
		}
	}
	f(``, -1)
	f(`x`, -1)
	f(`"`, -1)
	f(`""`, 1)
	f(`foobar"`, -1)
	f(`"foo"`, 4)
	f(`"\""`, 3)
	f(`"\\"`, 3)
	f(`"\"`, -1)
	f(`"foo\"bar\"baz"`, 14)
}

func TestUnescapeValueFailure(t *testing.T) {
	f := func(s string) {
		t.Helper()
		ss, err := unescapeValue(s)
		if err == nil {
			t.Fatalf("expecting error")
		}
		if ss != "" {
			t.Fatalf("expecting empty string; got %q", ss)
		}
	}
	f(``)
	f(`foobar`)
	f(`"foobar`)
	f(`foobar"`)
	f(`"foobar\"`)
	f(` "foobar"`)
	f(`"foobar" `)
}

func TestUnescapeValueSuccess(t *testing.T) {
	f := func(s, resultExpected string) {
		t.Helper()
		result, err := unescapeValue(s)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		if result != resultExpected {
			t.Fatalf("unexpected result; got %q; want %q", result, resultExpected)
		}
	}
	f(`""`, "")
	f(`"f"`, "f")
	f(`"foobar"`, "foobar")
	f(`"\"\n\t"`, "\"\n\t")
}

func TestRowsUnmarshalFailure(t *testing.T) {
	f := func(s string) {
		t.Helper()
		var rows Rows
		rows.Unmarshal(s)
		if len(rows.Rows) != 0 {
			t.Fatalf("unexpected number of rows parsed; got %d; want 0;\nrows:%#v", len(rows.Rows), rows.Rows)
		}

		// Try again
		rows.Unmarshal(s)
		if len(rows.Rows) != 0 {
			t.Fatalf("unexpected number of rows parsed; got %d; want 0;\nrows:%#v", len(rows.Rows), rows.Rows)
		}
	}

	// Empty lines and comments
	f("")
	f(" ")
	f("\t")
	f("\t  \r")
	f("\t\t  \n\n  # foobar")
	f("#foobar")
	f("#foobar\n")

	// invalid tags
	f("a{")
	f("a { ")
	f("a {foo")
	f("a {foo} 3")
	f("a {foo  =")
	f(`a {foo  ="bar`)
	f(`a {foo  ="b\ar`)
	f(`a {foo  = "bar"`)
	f(`a {foo  ="bar",`)
	f(`a {foo  ="bar" , `)
	f(`a {foo  ="bar" , baz } 2`)

	// empty metric name
	f(`{foo="bar"}`)

	// Missing value
	f("aaa")
	f(" aaa")
	f(" aaa ")
	f(" aaa   \n")
	f(` aa{foo="bar"}   ` + "\n")
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

	// Empty line or comment
	f("", &Rows{})
	f("\r", &Rows{})
	f("\n\n", &Rows{})
	f("\n\r\n", &Rows{})
	f("\t  \t\n\r\n#foobar\n  # baz", &Rows{})

	// Single line
	f("foobar 78.9", &Rows{
		Rows: []Row{{
			Metric: "foobar",
			Value:  78.9,
		}},
	})
	f("foobar 123.456 789\n", &Rows{
		Rows: []Row{{
			Metric:    "foobar",
			Value:     123.456,
			Timestamp: 789,
		}},
	})
	f("foobar{} 123.456 789\n", &Rows{
		Rows: []Row{{
			Metric:    "foobar",
			Value:     123.456,
			Timestamp: 789,
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
	f(`foo{bar="baz"} 1 2`, &Rows{
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
	f(`foo{bar="b\"a\\z"} -1.2`, &Rows{
		Rows: []Row{{
			Metric: "foo",
			Tags: []Tag{{
				Key:   "bar",
				Value: "b\"a\\z",
			}},
			Value: -1.2,
		}},
	})
	// Empty tags
	f(`foo {bar="baz",aa="",x="y",="z"} 1 2`, &Rows{
		Rows: []Row{{
			Metric: "foo",
			Tags: []Tag{
				{
					Key:   "bar",
					Value: "baz",
				},
				{
					Key:   "aa",
					Value: "",
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

	// Trailing comma after tag
	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/350
	f(`foo{bar="baz",} 1 2`, &Rows{
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

	// Multi lines
	f("# foo\n # bar ba zzz\nfoo 0.3 2\naaa 3\nbar.baz 0.34 43\n", &Rows{
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
	f("\t foo\t {  } 0.3\t 2\naaa\n  bar.baz 0.34 43\n", &Rows{
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

	// Spaces around tags
	f(`vm_accounting	{   name="vminsertRows", accountID = "1" , projectID=	"1"   } 277779100`, &Rows{
		Rows: []Row{
			{
				Metric: "vm_accounting",
				Tags: []Tag{
					{
						Key:   "name",
						Value: "vminsertRows",
					},
					{
						Key:   "accountID",
						Value: "1",
					},
					{
						Key:   "projectID",
						Value: "1",
					},
				},
				Value:     277779100,
				Timestamp: 0,
			},
		},
	})
}
