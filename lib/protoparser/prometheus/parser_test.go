package prometheus

import (
	"math"
	"reflect"
	"testing"
)

func TestGetRowsDiff(t *testing.T) {
	f := func(s1, s2, resultExpected string) {
		t.Helper()
		result := GetRowsDiff(s1, s2)
		if result != resultExpected {
			t.Fatalf("unexpected result for GetRowsDiff(%q, %q); got %q; want %q", s1, s2, result, resultExpected)
		}
	}
	f("", "", "")
	f("", "foo 1", "")
	f("  ", "foo 1", "")
	f("foo 123", "", "foo 0\n")
	f("foo 123", "bar 3", "foo 0\n")
	f("foo 123", "bar 3\nfoo 344", "")
	f("foo{x=\"y\", z=\"a a a\"} 123", "bar 3\nfoo{x=\"y\", z=\"b b b\"} 344", "foo{x=\"y\",z=\"a a a\"} 0\n")
	f("foo{bar=\"baz\"} 123\nx 3.4 5\ny 5 6", "x 34 342", "foo{bar=\"baz\"} 0\ny 0\n")
}

func TestAreIdenticalSeriesFast(t *testing.T) {
	f := func(s1, s2 string, resultExpected bool) {
		t.Helper()
		result := AreIdenticalSeriesFast(s1, s2)
		if result != resultExpected {
			t.Fatalf("unexpected result for AreIdenticalSeries(%q, %q); got %v; want %v", s1, s2, result, resultExpected)
		}
	}
	f("", "", true)
	f("", "a 1", false)   // different number of metrics
	f(" ", " a 1", false) // different number of metrics
	f("a 1", "", false)   // different number of metrics
	f(" a 1", " ", false) // different number of metrics
	f("foo", "foo", true) // consider series identical if they miss value
	f("foo 1", "foo 1", true)
	f("foo 1", "foo 2", true)
	f("foo 1 ", "foo 2 ", true)
	f("foo 1  ", "foo 2 ", false) // different number of spaces
	f("foo 1 ", "foo 2  ", false) // different number of spaces
	f("foo nan", "foo -inf", true)
	f("foo 1 # coment x", "foo 2 #comment y", true)
	f(" foo 1", " foo 1", true)
	f(" foo 1", "  foo 1", false) // different number of spaces in front of metric
	f("  foo 1", " foo 1", false) // different number of spaces in front of metric
	f("foo 1", "bar 1", false)    // different metric name
	f("foo 1", "fooo 1", false)   // different metric name
	f("foo  123", "foo  32.32", true)
	f(`foo{bar="x"} -3.3e-6`, `foo{bar="x"} 23343`, true)
	f(`foo{} 1`, `foo{} 234`, true)
	f(`foo {x="y   x" }  234`, `foo {x="y   x" }  43.342`, true)
	f(`foo {x="y x"} 234`, `foo{x="y x"} 43.342`, false) // different spaces
	f("foo 2\nbar 3", "foo 34.43\nbar -34.3", true)
	f("foo 2\nbar 3", "foo 34.43\nbarz -34.3", false) // different metric names
	f("\nfoo 13\n", "\nfoo 3.4\n", true)
	f("\nfoo 13", "\nfoo 3.4\n", false) // different number of blank lines
	f("\nfoo 13\n", "\nfoo 3.4", false) // different number of blank lines
	f("\n\nfoo 1", "\n\nfoo 34.43", true)
	f("\n\nfoo 3434\n", "\n\nfoo 43\n", true)
	f("\nfoo 1", "\n\nfoo 34.43", false) // different number of blank lines
	f("#foo{bar}", "#baz", true)
	f("", "#baz", false)             // different number of comments
	f("#foo{bar}", "", false)        // different number of comments
	f("#foo{bar}", "bar 3", false)   // different number of comments
	f("foo{bar} 2", "#bar 3", false) // different number of comments
	f("#foo\n", "#bar", false)       // different number of blank lines
	f("#foo{bar}\n#baz", "#baz\n#xdsfds dsf", true)
	f("# foo\nbar 234\nbaz{x=\"y\", z=\"\"} 3", "# foo\nbar 3.3\nbaz{x=\"y\", z=\"\"} 4323", true)
	f("# foo\nbar 234\nbaz{x=\"z\", z=\"\"} 3", "# foo\nbar 3.3\nbaz{x=\"y\", z=\"\"} 4323", false) // different label value
	f("foo {bar=\"xfdsdsffdsa\"} 1", "foo {x=\"y\"} 2", false)                                      // different labels
	f("foo {x=\"z\"} 1", "foo {x=\"y\"} 2", false)                                                  // different label value

	// Lines with timestamps
	f("foo 1 2", "foo 234 4334", true)
	f("foo 2", "foo 3 4", false) // missing timestamp
	f("foo 2 1", "foo 3", false) // missing timestamp
	f("foo{bar=\"b az\"} 2 5", "foo{bar=\"b az\"} +6.3 7.43", true)
	f("foo{bar=\"b az\"} 2 5 # comment ss ", "foo{bar=\"b az\"} +6.3 7.43 # comment as ", true)
	f("foo{bar=\"b az\"} 2 5 #comment", "foo{bar=\"b az\"} +6.3 7.43 #comment {foo=\"bar\"} 21.44", true)
	f("foo{bar=\"b az\"} +Inf 5", "foo{bar=\"b az\"} NaN 7.43", true)
	f("foo{bar=\"b az\"} +Inf 5", "foo{bar=\"b az\"} nan 7.43", true)
	f("foo{bar=\"b az\"} +Inf 5", "foo{bar=\"b az\"} nansf 7.43", false) // invalid value

	// False positive - whitespace after the numeric char in the label.
	f(`foo{bar=" 12.3 "} 1`, `foo{bar=" 13 "} 23`, true)
	f(`foo{bar=" 12.3 "} 1 3443`, `foo{bar=" 13 "} 23 4345`, true)
	f(`foo{bar=" 12.3 "} 1 3443 # {} 34`, `foo{bar=" 13 "} 23 4345 # {foo=" bar "} 34`, true)

	// Metrics and labels with '#' chars
	f(`foo{bar="#1"} 1`, `foo{bar="#1"} 1`, true)
	f(`foo{bar="a#1"} 1`, `foo{bar="b#1"} 1.4`, false)
	f(`foo{bar=" #1"} 1`, `foo{bar=" #1"} 3`, true)
	f(`foo{bar=" #1 "} 1`, `foo{bar=" #1 "} -2.34 343.34 # {foo="#bar"} `, true)
	f(`foo{bar=" #1"} 1`, `foo{bar="#1"} 1`, false)
	f(`foo{b#ar=" #1"} 1`, `foo{b#ar=" #1"} 1.23`, true)
	f(`foo{z#ar=" #1"} 1`, `foo{b#ar=" #1"} 1.23`, false)
	f(`fo#o{b#ar="#1"} 1`, `fo#o{b#ar="#1"} 1.23`, true)
	f(`fo#o{b#ar="#1"} 1`, `fa#o{b#ar="#1"} 1.23`, false)

	// False positive - the value after '#' char can be arbitrary
	f(`fo#o{b#ar="#1"} 1`, `fo#osdf 1.23`, true)
}

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

func TestUnescapeValue(t *testing.T) {
	f := func(s, resultExpected string) {
		t.Helper()
		result := unescapeValue(s)
		if result != resultExpected {
			t.Fatalf("unexpected result; got %q; want %q", result, resultExpected)
		}
	}
	f(``, "")
	f(`f`, "f")
	f(`foobar`, "foobar")
	f(`\"\n\t`, "\"\n\\t")

	// Edge cases
	f(`foo\bar`, "foo\\bar")
	f(`foo\`, "foo\\")
}

func TestAppendEscapedValue(t *testing.T) {
	f := func(s, resultExpected string) {
		t.Helper()
		result := appendEscapedValue(nil, s)
		if string(result) != resultExpected {
			t.Fatalf("unexpected result; got %q; want %q", result, resultExpected)
		}
	}
	f(``, ``)
	f(`f`, `f`)
	f(`foobar`, `foobar`)
	f("\"\n\t\\xyz", "\\\"\\n\t\\\\xyz")
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

	// invalid tags - see https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4284
	f(`a{"__name__":"upsd_time_left_ns","host":"myhost", status_OB="true"} 12`)
	f(`a{host:"myhost"} 12`)
	f(`a{host:"myhost",foo="bar"} 12`)

	// empty metric name
	f(`{foo="bar"}`)

	// Invalid quotes for label value
	f(`{foo='bar'} 23`)
	f("{foo=`bar`} 23")

	// Missing value
	f("aaa")
	f(" aaa")
	f(" aaa ")
	f(" aaa   \n")
	f(` aa{foo="bar"}   ` + "\n")

	// Invalid value
	f("foo bar")
	f("foo bar 124")

	// Invalid timestamp
	f("foo 123 bar")
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
			Timestamp: 789000,
		}},
	})
	f("foobar{} 123.456 789.4354\n", &Rows{
		Rows: []Row{{
			Metric:    "foobar",
			Value:     123.456,
			Timestamp: 789435,
		}},
	})
	f(`#                                    _                                            _
#   ___ __ _ ___ ___  __ _ _ __   __| |_ __ __ _        _____  ___ __   ___  _ __| |_ ___ _ __
`+"#  / __/ _` / __/ __|/ _` | '_ \\ / _` | '__/ _` |_____ / _ \\ \\/ / '_ \\ / _ \\| '__| __/ _ \\ '__|\n"+`
# | (_| (_| \__ \__ \ (_| | | | | (_| | | | (_| |_____|  __/>  <| |_) | (_) | |  | ||  __/ |
#  \___\__,_|___/___/\__,_|_| |_|\__,_|_|  \__,_|      \___/_/\_\ .__/ \___/|_|   \__\___|_|
#                                                               |_|
#
# TYPE cassandra_token_ownership_ratio gauge
cassandra_token_ownership_ratio 78.9`, &Rows{
		Rows: []Row{{
			Metric: "cassandra_token_ownership_ratio",
			Value:  78.9,
		}},
	})

	// `#` char in label value
	f(`foo{bar="#1 az"} 24`, &Rows{
		Rows: []Row{{
			Metric: "foo",
			Tags: []Tag{{
				Key:   "bar",
				Value: "#1 az",
			}},
			Value: 24,
		}},
	})

	// `#` char in label name and label value
	f(`foo{bar#2="#1 az"} 24 456`, &Rows{
		Rows: []Row{{
			Metric: "foo",
			Tags: []Tag{{
				Key:   "bar#2",
				Value: "#1 az",
			}},
			Value:     24,
			Timestamp: 456000,
		}},
	})

	// `#` char in metric name, label name and label value
	f(`foo#qw{bar#2="#1 az"} 24 456 # foobar {baz="x"}`, &Rows{
		Rows: []Row{{
			Metric: "foo#qw",
			Tags: []Tag{{
				Key:   "bar#2",
				Value: "#1 az",
			}},
			Value:     24,
			Timestamp: 456000,
		}},
	})

	// Incorrectly escaped backlash. This is real-world case, which must be supported.
	f(`mssql_sql_server_active_transactions_sec{loginname="domain\somelogin",env="develop"} 56`, &Rows{
		Rows: []Row{{
			Metric: "mssql_sql_server_active_transactions_sec",
			Tags: []Tag{
				{
					Key:   "loginname",
					Value: "domain\\somelogin",
				},
				{
					Key:   "env",
					Value: "develop",
				},
			},
			Value: 56,
		}},
	})

	// Exemplars - see https://github.com/OpenObservability/OpenMetrics/blob/master/OpenMetrics.md#exemplars-1
	f(`foo_bucket{le="10",a="#b"} 17 # {trace_id="oHg5SJ#YRHA0"} 9.8 1520879607.789
	   abc 123 456 # foobar
	   foo   344#bar`, &Rows{
		Rows: []Row{
			{
				Metric: "foo_bucket",
				Tags: []Tag{
					{
						Key:   "le",
						Value: "10",
					},
					{
						Key:   "a",
						Value: "#b",
					},
				},
				Value: 17,
			},
			{
				Metric:    "abc",
				Value:     123,
				Timestamp: 456000,
			},
			{
				Metric: "foo",
				Value:  344,
			},
		},
	})

	// "Infinity" word - this has been added in OpenMetrics.
	// See https://github.com/OpenObservability/OpenMetrics/blob/master/OpenMetrics.md
	// Checks for https://github.com/VictoriaMetrics/VictoriaMetrics/issues/924
	inf := math.Inf(1)
	f(`
		foo Infinity
		bar +Infinity
		baz -infinity
		aaa +inf
		bbb -INF
		ccc INF
	`, &Rows{
		Rows: []Row{
			{
				Metric: "foo",
				Value:  inf,
			},
			{
				Metric: "bar",
				Value:  inf,
			},
			{
				Metric: "baz",
				Value:  -inf,
			},
			{
				Metric: "aaa",
				Value:  inf,
			},
			{
				Metric: "bbb",
				Value:  -inf,
			},
			{
				Metric: "ccc",
				Value:  inf,
			},
		},
	})

	// Timestamp bigger than 1<<31.
	// It should be parsed in milliseconds.
	f("aaa 1123 429496729600", &Rows{
		Rows: []Row{{
			Metric:    "aaa",
			Value:     1123,
			Timestamp: 429496729600,
		}},
	})

	// Floating-point timestamps in OpenMetric format.
	f("aaa 1123 42949.567", &Rows{
		Rows: []Row{{
			Metric:    "aaa",
			Value:     1123,
			Timestamp: 42949567,
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
			Timestamp: 2000,
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
			Timestamp: 2000,
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
			Timestamp: 2000,
		}},
	})

	// Multi lines
	f("# foo\n # bar ba zzz\nfoo 0.3 2\naaa 3\nbar.baz 0.34 43\n", &Rows{
		Rows: []Row{
			{
				Metric:    "foo",
				Value:     0.3,
				Timestamp: 2000,
			},
			{
				Metric: "aaa",
				Value:  3,
			},
			{
				Metric:    "bar.baz",
				Value:     0.34,
				Timestamp: 43000,
			},
		},
	})

	// Multi lines with invalid line
	f("\t foo\t {  } 0.3\t 2\naaa\n  bar.baz 0.34 43\n", &Rows{
		Rows: []Row{
			{
				Metric:    "foo",
				Value:     0.3,
				Timestamp: 2000,
			},
			{
				Metric:    "bar.baz",
				Value:     0.34,
				Timestamp: 43000,
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
