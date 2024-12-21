package prometheus

import (
	"encoding/base64"
	"fmt"
	"math"
	"reflect"
	"testing"
)

func TestGetRowsDiff(t *testing.T) {
	f := func(s1, s2, resultExpected string, countExpected int, binary bool) {
		t.Helper()
		result, count := GetRowsDiff(s1, s2, binary)
		if result != resultExpected {
			t.Fatalf("unexpected result for GetRowsDiff(%q, %q); got %q; want %q", s1, s2, result, resultExpected)
		}
		if count != countExpected {
			t.Fatalf("unexpected count for GetRowsDiff(%q, %q); got %d; want %d", s1, s2, count, countExpected)
		}
	}

	protoMetricGenAndTest := func(totalCount, newCount, oldCount int) {
		families := make([]*MetricFamily, totalCount)
		metricsPerFamily := 5
		for f := range families {
			families[f] = &MetricFamily{
				Name:    fmt.Sprintf("foo_%s", string(rune(f+65))),
				Metrics: make([]*Metric, metricsPerFamily),
			}
			for m := range families[f].Metrics {
				families[f].Metrics[m] = &Metric{
					Tags: []Tag{
						{
							Key:   fmt.Sprintf("name_%s", string(rune(m+65))),
							Value: fmt.Sprintf("value_%s", string(rune(m+65))),
						},
					},
					Counter: &Counter{
						Value: float64(f + m),
					},
				}
			}
		}

		incomingReq := &ProtoRequest{
			Families: families[:totalCount-oldCount],
		}
		cachedReq := &ProtoRequest{
			Families: families[newCount:],
		}
		expectedReq := &ProtoRequest{
			Families: families[:newCount],
		}
		var output []byte
		output = incomingReq.marshalProtobuf(output)
		incomingEnd := len(output)
		output = cachedReq.marshalProtobuf(output)
		cachedEnd := len(output)
		output = expectedReq.marshalProtobuf(output)
		expectedCount := metricsPerFamily * newCount
		f(string(output[:incomingEnd]), string(output[incomingEnd:cachedEnd]), string(output[cachedEnd:]), expectedCount, true)
	}

	f("", "", "", 0, false)
	f("", "foo 1", "", 0, false)
	f("  ", "foo 1", "", 0, false)
	f("foo 123", "", "foo 0\n", 1, false)
	f("foo 123", "bar 3", "foo 0\n", 1, false)
	f("foo 123", "bar 3\nfoo 344", "", 0, false)
	f("foo{x=\"y\", z=\"a a a\"} 123", "bar 3\nfoo{x=\"y\", z=\"b b b\"} 344", "foo{x=\"y\",z=\"a a a\"} 0\n", 1, false)
	f("foo{bar=\"baz\"} 123\nx 3.4 5\ny 5 6", "x 34 342", "foo{bar=\"baz\"} 0\ny 0\n", 2, false)

	// protobuf data
	f("", "", "", 0, true)
	protoMetricGenAndTest(5, 1, 2)
	protoMetricGenAndTest(10, 0, 0)
	protoMetricGenAndTest(10, 10, 0)
	protoMetricGenAndTest(35, 17, 15)
}

func TestAreIdenticalTextSeriesFast(t *testing.T) {
	f := func(s1, s2 string, resultExpected bool) {
		t.Helper()
		result := AreIdenticalTextSeriesFast(s1, s2)
		if result != resultExpected {
			t.Fatalf("unexpected result for AreIdenticalTextSeries(%q, %q); got %v; want %v", s1, s2, result, resultExpected)
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
	f := func(s string, ct ContentType) {
		t.Helper()
		var rows Rows
		if ct == ProtoHeader {
			decoded, _ := base64.StdEncoding.DecodeString(s)
			s = string(decoded)
		}
		rows.Unmarshal(s, ct)
		if len(rows.Rows) != 0 {
			t.Fatalf("unexpected number of rows parsed; got %d; want 0;\nrows:%#v", len(rows.Rows), rows.Rows)
		}

		// Try again
		rows.Unmarshal(s, ct)
		if len(rows.Rows) != 0 {
			t.Fatalf("unexpected number of rows parsed; got %d; want 0;\nrows:%#v", len(rows.Rows), rows.Rows)
		}
	}

	// Empty lines and comments
	f("", TextHeader)
	f(" ", TextHeader)
	f("\t", TextHeader)
	f("\t  \r", TextHeader)
	f("\t\t  \n\n  # foobar", TextHeader)
	f("#foobar", TextHeader)
	f("#foobar\n", TextHeader)

	// invalid tags
	f("a{", TextHeader)
	f("a { ", TextHeader)
	f("a {foo", TextHeader)
	f("a {foo} 3", TextHeader)
	f("a {foo  =", TextHeader)
	f(`a {foo  ="bar`, TextHeader)
	f(`a {foo  ="b\ar`, TextHeader)
	f(`a {foo  = "bar"`, TextHeader)
	f(`a {foo  ="bar",`, TextHeader)
	f(`a {foo  ="bar" , `, TextHeader)
	f(`a {foo  ="bar" , baz } 2`, TextHeader)

	// invalid tags - see https://github.com/VictoriaMetrics/VictoriaMetrics/issues/4284
	f(`a{"__name__":"upsd_time_left_ns","host":"myhost", status_OB="true"} 12`, TextHeader)
	f(`a{host:"myhost"} 12`, TextHeader)
	f(`a{host:"myhost",foo="bar"} 12`, TextHeader)

	// empty metric name
	f(`{foo="bar"}`, TextHeader)

	// Invalid quotes for label value
	f(`{foo='bar'} 23`, TextHeader)
	f("{foo=`bar`} 23", TextHeader)

	// Missing value
	f("aaa", TextHeader)
	f(" aaa", TextHeader)
	f(" aaa ", TextHeader)
	f(" aaa   \n", TextHeader)
	f(` aa{foo="bar"}   `+"\n", TextHeader)

	// Invalid value
	f("foo bar", TextHeader)
	f("foo bar 124", TextHeader)

	// Invalid timestamp
	f("foo 123 bar", TextHeader)
}

func TestRowsUnmarshalSuccess(t *testing.T) {
	f := func(s string, ct ContentType, rowsExpected *Rows) {
		t.Helper()
		if ct == ProtoHeader {
			decoded, _ := base64.StdEncoding.DecodeString(s)
			s = string(decoded)
		}
		var rows Rows
		rows.Unmarshal(s, ct)
		if !reflect.DeepEqual(rows.Rows, rowsExpected.Rows) {
			t.Fatalf("unexpected rows;\ngot\n%+v;\nwant\n%+v", rows.Rows, rowsExpected.Rows)
		}

		// Try unmarshaling again
		rows.Unmarshal(s, ct)
		if !reflect.DeepEqual(rows.Rows, rowsExpected.Rows) {
			t.Fatalf("unexpected rows;\ngot\n%+v;\nwant\n%+v", rows.Rows, rowsExpected.Rows)
		}

		rows.Reset()
		if len(rows.Rows) != 0 {
			t.Fatalf("non-empty rows after reset: %+v", rows.Rows)
		}
	}

	// Empty line or comment
	f("", TextHeader, &Rows{})
	f("\r", TextHeader, &Rows{})
	f("\n\n", TextHeader, &Rows{})
	f("\n\r\n", TextHeader, &Rows{})
	f("\t  \t\n\r\n#foobar\n  # baz", TextHeader, &Rows{})

	// Single line
	f("foobar 78.9", TextHeader, &Rows{
		Rows: []Row{{
			Metric: "foobar",
			Value:  78.9,
		}},
	})
	f("foobar 123.456 789\n", TextHeader, &Rows{
		Rows: []Row{{
			Metric:    "foobar",
			Value:     123.456,
			Timestamp: 789000,
		}},
	})
	f("foobar{} 123.456 789.4354\n", TextHeader, &Rows{
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
cassandra_token_ownership_ratio 78.9`, TextHeader, &Rows{
		Rows: []Row{{
			Metric: "cassandra_token_ownership_ratio",
			Value:  78.9,
		}},
	})

	// `#` char in label value
	f(`foo{bar="#1 az"} 24`, TextHeader, &Rows{
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
	f(`foo{bar#2="#1 az"} 24 456`, TextHeader, &Rows{
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
	f(`foo#qw{bar#2="#1 az"} 24 456 # foobar {baz="x"}`, TextHeader, &Rows{
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
	f(`mssql_sql_server_active_transactions_sec{loginname="domain\somelogin",env="develop"} 56`, TextHeader, &Rows{
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
	   foo   344#bar`, TextHeader, &Rows{
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
	`, TextHeader, &Rows{
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
	f("aaa 1123 429496729600", TextHeader, &Rows{
		Rows: []Row{{
			Metric:    "aaa",
			Value:     1123,
			Timestamp: 429496729600,
		}},
	})

	// Floating-point timestamps in OpenMetric format.
	f("aaa 1123 42949.567", TextHeader, &Rows{
		Rows: []Row{{
			Metric:    "aaa",
			Value:     1123,
			Timestamp: 42949567,
		}},
	})

	// Tags
	f(`foo{bar="baz"} 1 2`, TextHeader, &Rows{
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
	f(`foo{bar="b\"a\\z"} -1.2`, TextHeader, &Rows{
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
	f(`foo {bar="baz",aa="",x="y",="z"} 1 2`, TextHeader, &Rows{
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
	f(`foo{bar="baz",} 1 2`, TextHeader, &Rows{
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
	f("# foo\n # bar ba zzz\nfoo 0.3 2\naaa 3\nbar.baz 0.34 43\n", TextHeader, &Rows{
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
	f("\t foo\t {  } 0.3\t 2\naaa\n  bar.baz 0.34 43\n", TextHeader, &Rows{
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
	f(`vm_accounting	{   name="vminsertRows", accountID = "1" , projectID=	"1"   } 277779100`, TextHeader, &Rows{
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

	// Proto gauge metric
	f("TgodcHJvY2Vzc19yZXNpZGVudF9tZW1vcnlfYnl0ZXMSHlJlc2lkZW50IG1lbW9yeSBzaXplIGluIGJ5dGVzLhgBIgsSCQkAAAAAQM6SQQ==", ProtoHeader, &Rows{
		Rows: []Row{
			{
				Metric:    "process_resident_memory_bytes",
				Value:     78876672.0,
				Timestamp: 0,
			},
		},
	})

	// Proto counter metric
	f("rwEKOXByb21ldGhldXNfdGFyZ2V0X3NjcmFwZV9wb29sX2V4Y2VlZGVkX2xhYmVsX2xpbWl0c190b3RhbBJWVG90YWwgbnVtYmVyIG9mIHRpbWVzIHNjcmFwZSBwb29scyBoaXQgdGhlIGxhYmVsIGxpbWl0cywgZHVyaW5nIHN5bmMgb3IgY29uZmlnIHJlbG9hZC4YACIYGhYJAAAAAAAAAAAaCwjUsJavBhC6tZYl", ProtoHeader, &Rows{
		Rows: []Row{
			{
				Metric:    "prometheus_target_scrape_pool_exceeded_label_limits_total",
				Value:     0,
				Timestamp: 0,
			},
		},
	})

	// Proto histogram metric
	f("nwIKKHByb21ldGhldXNfdHNkYl9jb21wYWN0aW9uX2NodW5rX3NhbXBsZXMSMUZpbmFsIG51bWJlciBvZiBzYW1wbGVzIG9uIHRoZWlyIGZpcnN0IGNvbXBhY3Rpb24YBCK9ATq6AQjrIBEAAAAAsPQcQRoLCAARAAAAAAAAEEAaCwgAEQAAAAAAABhAGgsIABEAAAAAAAAiQBoLCAARAAAAAAAAK0AaCwgAEQAAAAAAQDRAGgsIABEAAAAAAGA+QBoLCAARAAAAAADIRkAaDAjTAxEAAAAAABZRQBoMCNMDEQAAAAAAoVlAGgwI6yARAAAAAMA4Y0AaDAjrIBEAAAAAINVsQBoMCOsgEQAAAADYn3VAegsI1LCWrwYQ3oqRJg==", ProtoHeader, &Rows{
		Rows: []Row{
			{
				Metric:    "prometheus_tsdb_compaction_chunk_samples_count",
				Value:     4203,
				Timestamp: 0,
			},
			{
				Metric:    "prometheus_tsdb_compaction_chunk_samples_sum",
				Value:     474412,
				Timestamp: 0,
			},
			{
				Metric: "prometheus_tsdb_compaction_chunk_samples_bucket",
				Tags: []Tag{
					{
						Key:   "le",
						Value: "4",
					},
				},
				Value:     0,
				Timestamp: 0,
			},
			{
				Metric: "prometheus_tsdb_compaction_chunk_samples_bucket",
				Tags: []Tag{
					{
						Key:   "le",
						Value: "6",
					},
				},
				Value:     0,
				Timestamp: 0,
			},
			{
				Metric: "prometheus_tsdb_compaction_chunk_samples_bucket",
				Tags: []Tag{
					{
						Key:   "le",
						Value: "9",
					},
				},
				Value:     0,
				Timestamp: 0,
			},
			{
				Metric: "prometheus_tsdb_compaction_chunk_samples_bucket",
				Tags: []Tag{
					{
						Key:   "le",
						Value: "13.5",
					},
				},
				Value:     0,
				Timestamp: 0,
			},
			{
				Metric: "prometheus_tsdb_compaction_chunk_samples_bucket",
				Tags: []Tag{
					{
						Key:   "le",
						Value: "20.2",
					},
				},
				Value:     0,
				Timestamp: 0,
			},
			{
				Metric: "prometheus_tsdb_compaction_chunk_samples_bucket",
				Tags: []Tag{
					{
						Key:   "le",
						Value: "30.4",
					},
				},
				Value:     0,
				Timestamp: 0,
			},
			{
				Metric: "prometheus_tsdb_compaction_chunk_samples_bucket",
				Tags: []Tag{
					{
						Key:   "le",
						Value: "45.6",
					},
				},
				Value:     0,
				Timestamp: 0,
			},
			{
				Metric: "prometheus_tsdb_compaction_chunk_samples_bucket",
				Tags: []Tag{
					{
						Key:   "le",
						Value: "68.3",
					},
				},
				Value:     467,
				Timestamp: 0,
			},
			{
				Metric: "prometheus_tsdb_compaction_chunk_samples_bucket",
				Tags: []Tag{
					{
						Key:   "le",
						Value: "103",
					},
				},
				Value:     467,
				Timestamp: 0,
			},
			{
				Metric: "prometheus_tsdb_compaction_chunk_samples_bucket",
				Tags: []Tag{
					{
						Key:   "le",
						Value: "154",
					},
				},
				Value:     4203,
				Timestamp: 0,
			},
			{
				Metric: "prometheus_tsdb_compaction_chunk_samples_bucket",
				Tags: []Tag{
					{
						Key:   "le",
						Value: "231",
					},
				},
				Value:     4203,
				Timestamp: 0,
			},
			{
				Metric: "prometheus_tsdb_compaction_chunk_samples_bucket",
				Tags: []Tag{
					{
						Key:   "le",
						Value: "346",
					},
				},
				Value:     4203,
				Timestamp: 0,
			},
		},
	})

	// Proto summary metric
	f("4wEKKXByb21ldGhldXNfdGFyZ2V0X2ludGVydmFsX2xlbmd0aF9zZWNvbmRzEiFBY3R1YWwgaW50ZXJ2YWxzIGJldHdlZW4gc2NyYXBlcy4YAiKQAQoPCghpbnRlcnZhbBIDMTVzIn0I7gMR5TQoQAHyvEAaEgl7FK5H4XqEPxFAR0Yv7P8tQBoSCZqZmZmZmak/EQ3rnTzy/y1AGhIJAAAAAAAA4D8R3Df0nwAALkAaEgnNzMzMzMzsPxHSCl+hBwAuQBoSCa5H4XoUru8/EayR//cXAC5AIgsI9LCWrwYQ+su9FA==", ProtoHeader, &Rows{
		Rows: []Row{
			{
				Metric: "prometheus_target_interval_length_seconds",
				Tags: []Tag{
					{
						Key:   "interval",
						Value: "15s",
					},
					{
						Key:   "quantile",
						Value: "0.01",
					},
				},
				Value:     14.999848821,
				Timestamp: 0,
			},
			{
				Metric: "prometheus_target_interval_length_seconds",
				Tags: []Tag{
					{
						Key:   "interval",
						Value: "15s",
					},
					{
						Key:   "quantile",
						Value: "0.05",
					},
				},
				Value:     14.999894995,
				Timestamp: 0,
			},
			{
				Metric: "prometheus_target_interval_length_seconds",
				Tags: []Tag{
					{
						Key:   "interval",
						Value: "15s",
					},
					{
						Key:   "quantile",
						Value: "0.5",
					},
				},
				Value:     15.000004767,
				Timestamp: 0,
			},
			{
				Metric: "prometheus_target_interval_length_seconds",
				Tags: []Tag{
					{
						Key:   "interval",
						Value: "15s",
					},
					{
						Key:   "quantile",
						Value: "0.9",
					},
				},
				Value:     15.000058215,
				Timestamp: 0,
			},
			{
				Metric: "prometheus_target_interval_length_seconds",
				Tags: []Tag{
					{
						Key:   "interval",
						Value: "15s",
					},
					{
						Key:   "quantile",
						Value: "0.99",
					},
				},
				Value:     15.000182867,
				Timestamp: 0,
			},
			{
				Metric: "prometheus_target_interval_length_seconds_count",
				Tags: []Tag{
					{
						Key:   "interval",
						Value: "15s",
					},
				},
				Value:     494,
				Timestamp: 0,
			},
			{
				Metric: "prometheus_target_interval_length_seconds_sum",
				Tags: []Tag{
					{
						Key:   "interval",
						Value: "15s",
					},
				},
				Value:     7410.004885209001,
				Timestamp: 0,
			},
		},
	})
}
