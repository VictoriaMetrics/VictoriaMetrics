package logstorage

import (
	"fmt"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/regexutil"
)

func TestFilterRegexp(t *testing.T) {
	t.Parallel()

	t.Run("const-column", func(t *testing.T) {
		t.Parallel()

		columns := []column{
			{
				name: "foo",
				values: []string{
					"127.0.0.1",
					"127.0.0.1",
					"127.0.0.1",
				},
			},
		}

		// match
		fr := &filterRegexp{
			fieldName: "foo",
			re:        mustCompileRegex("0.0"),
		}
		testFilterMatchForColumns(t, columns, fr, "foo", []int{0, 1, 2})

		fr = &filterRegexp{
			fieldName: "foo",
			re:        mustCompileRegex(`^127\.0\.0\.1$`),
		}
		testFilterMatchForColumns(t, columns, fr, "foo", []int{0, 1, 2})

		fr = &filterRegexp{
			fieldName: "non-existing-column",
			re:        mustCompileRegex("foo.+bar|"),
		}
		testFilterMatchForColumns(t, columns, fr, "foo", []int{0, 1, 2})

		// mismatch
		fr = &filterRegexp{
			fieldName: "foo",
			re:        mustCompileRegex("foo.+bar"),
		}
		testFilterMatchForColumns(t, columns, fr, "foo", nil)

		fr = &filterRegexp{
			fieldName: "non-existing-column",
			re:        mustCompileRegex("foo.+bar"),
		}
		testFilterMatchForColumns(t, columns, fr, "foo", nil)
	})

	t.Run("dict", func(t *testing.T) {
		t.Parallel()

		columns := []column{
			{
				name: "foo",
				values: []string{
					"",
					"127.0.0.1",
					"Abc",
					"127.255.255.255",
					"10.4",
					"foo 127.0.0.1",
					"127.0.0.1 bar",
					"127.0.0.1",
				},
			},
		}

		// match
		fr := &filterRegexp{
			fieldName: "foo",
			re:        mustCompileRegex("foo|bar|^$"),
		}
		testFilterMatchForColumns(t, columns, fr, "foo", []int{0, 5, 6})

		fr = &filterRegexp{
			fieldName: "foo",
			re:        mustCompileRegex("27.0"),
		}
		testFilterMatchForColumns(t, columns, fr, "foo", []int{1, 5, 6, 7})

		// mismatch
		fr = &filterRegexp{
			fieldName: "foo",
			re:        mustCompileRegex("bar.+foo"),
		}
		testFilterMatchForColumns(t, columns, fr, "foo", nil)
	})

	t.Run("strings", func(t *testing.T) {
		t.Parallel()

		columns := []column{
			{
				name: "foo",
				values: []string{
					"A FOO",
					"a 10",
					"127.0.0.1",
					"20",
					"15.5",
					"-5",
					"a fooBaR",
					"a 127.0.0.1 dfff",
					"a ТЕСТЙЦУК НГКШ ",
					"a !!,23.(!1)",
				},
			},
		}

		// match
		fr := &filterRegexp{
			fieldName: "foo",
			re:        mustCompileRegex("(?i)foo|йцу"),
		}
		testFilterMatchForColumns(t, columns, fr, "foo", []int{0, 6, 8})

		// mismatch
		fr = &filterRegexp{
			fieldName: "foo",
			re:        mustCompileRegex("qwe.+rty|^$"),
		}
		testFilterMatchForColumns(t, columns, fr, "foo", nil)
	})

	t.Run("uint8", func(t *testing.T) {
		t.Parallel()

		columns := []column{
			{
				name: "foo",
				values: []string{
					"123",
					"12",
					"32",
					"0",
					"0",
					"12",
					"1",
					"2",
					"3",
					"4",
					"5",
				},
			},
		}

		// match
		fr := &filterRegexp{
			fieldName: "foo",
			re:        mustCompileRegex("[32][23]?"),
		}
		testFilterMatchForColumns(t, columns, fr, "foo", []int{0, 1, 2, 5, 7, 8})

		// mismatch
		fr = &filterRegexp{
			fieldName: "foo",
			re:        mustCompileRegex("foo|bar"),
		}
		testFilterMatchForColumns(t, columns, fr, "foo", nil)
	})

	t.Run("uint16", func(t *testing.T) {
		t.Parallel()

		columns := []column{
			{
				name: "foo",
				values: []string{
					"123",
					"12",
					"32",
					"0",
					"0",
					"65535",
					"1",
					"2",
					"3",
					"4",
					"5",
				},
			},
		}

		// match
		fr := &filterRegexp{
			fieldName: "foo",
			re:        mustCompileRegex("[32][23]?"),
		}
		testFilterMatchForColumns(t, columns, fr, "foo", []int{0, 1, 2, 5, 7, 8})

		// mismatch
		fr = &filterRegexp{
			fieldName: "foo",
			re:        mustCompileRegex("foo|bar"),
		}
		testFilterMatchForColumns(t, columns, fr, "foo", nil)
	})

	t.Run("uint32", func(t *testing.T) {
		t.Parallel()

		columns := []column{
			{
				name: "foo",
				values: []string{
					"123",
					"12",
					"32",
					"0",
					"0",
					"65536",
					"1",
					"2",
					"3",
					"4",
					"5",
				},
			},
		}

		// match
		fr := &filterRegexp{
			fieldName: "foo",
			re:        mustCompileRegex("[32][23]?"),
		}
		testFilterMatchForColumns(t, columns, fr, "foo", []int{0, 1, 2, 5, 7, 8})

		// mismatch
		fr = &filterRegexp{
			fieldName: "foo",
			re:        mustCompileRegex("foo|bar"),
		}
		testFilterMatchForColumns(t, columns, fr, "foo", nil)
	})

	t.Run("uint64", func(t *testing.T) {
		t.Parallel()

		columns := []column{
			{
				name: "foo",
				values: []string{
					"123",
					"12",
					"32",
					"0",
					"0",
					"12345678901",
					"1",
					"2",
					"3",
					"4",
					"5",
				},
			},
		}

		// match
		fr := &filterRegexp{
			fieldName: "foo",
			re:        mustCompileRegex("[32][23]?"),
		}
		testFilterMatchForColumns(t, columns, fr, "foo", []int{0, 1, 2, 5, 7, 8})

		// mismatch
		fr = &filterRegexp{
			fieldName: "foo",
			re:        mustCompileRegex("foo|bar"),
		}
		testFilterMatchForColumns(t, columns, fr, "foo", nil)
	})

	t.Run("float64", func(t *testing.T) {
		t.Parallel()

		columns := []column{
			{
				name: "foo",
				values: []string{
					"123",
					"12",
					"32",
					"0",
					"0",
					"123456.78901",
					"-0.2",
					"2",
					"-334",
					"4",
					"5",
				},
			},
		}

		// match
		fr := &filterRegexp{
			fieldName: "foo",
			re:        mustCompileRegex("[32][23]?"),
		}
		testFilterMatchForColumns(t, columns, fr, "foo", []int{0, 1, 2, 5, 6, 7, 8})

		// mismatch
		fr = &filterRegexp{
			fieldName: "foo",
			re:        mustCompileRegex("foo|bar"),
		}
		testFilterMatchForColumns(t, columns, fr, "foo", nil)
	})

	t.Run("ipv4", func(t *testing.T) {
		t.Parallel()

		columns := []column{
			{
				name: "foo",
				values: []string{
					"1.2.3.4",
					"0.0.0.0",
					"127.0.0.1",
					"254.255.255.255",
					"127.0.0.1",
					"127.0.0.1",
					"127.0.4.2",
					"127.0.0.1",
					"12.0.127.6",
					"55.55.12.55",
					"66.66.66.66",
					"7.7.7.7",
				},
			},
		}

		// match
		fr := &filterRegexp{
			fieldName: "foo",
			re:        mustCompileRegex("127.0.[40].(1|2)"),
		}
		testFilterMatchForColumns(t, columns, fr, "foo", []int{2, 4, 5, 6, 7})

		// mismatch
		fr = &filterRegexp{
			fieldName: "foo",
			re:        mustCompileRegex("foo|bar|834"),
		}
		testFilterMatchForColumns(t, columns, fr, "foo", nil)
	})

	t.Run("timestamp-iso8601", func(t *testing.T) {
		t.Parallel()

		columns := []column{
			{
				name: "_msg",
				values: []string{
					"2006-01-02T15:04:05.001Z",
					"2006-01-02T15:04:05.002Z",
					"2006-01-02T15:04:05.003Z",
					"2006-01-02T15:04:05.004Z",
					"2006-01-02T15:04:05.005Z",
					"2006-01-02T15:04:05.006Z",
					"2006-01-02T15:04:05.007Z",
					"2006-01-02T15:04:05.008Z",
					"2006-01-02T15:04:05.009Z",
				},
			},
		}

		// match
		fr := &filterRegexp{
			fieldName: "_msg",
			re:        mustCompileRegex("2006-[0-9]{2}-.+?(2|5)Z"),
		}
		testFilterMatchForColumns(t, columns, fr, "_msg", []int{1, 4})

		// mismatch
		fr = &filterRegexp{
			fieldName: "_msg",
			re:        mustCompileRegex("^01|04$"),
		}
		testFilterMatchForColumns(t, columns, fr, "_msg", nil)
	})
}

func TestSkipFirstLastToken(t *testing.T) {
	t.Parallel()

	f := func(s, resultExpected string) {
		t.Helper()

		result := skipFirstLastToken(s)
		if result != resultExpected {
			t.Fatalf("unexpected result in skipFirstLastToken(%q); got %q; want %q", s, result, resultExpected)
		}
	}

	f("", "")
	f("foobar", "")
	f("foo bar", " ")
	f("foo bar baz", " bar ")
	f(" foo bar baz", " foo bar ")
	f(",foo bar baz!", ",foo bar baz!")
	f("фыад длоа д!", " длоа д!")
}

func mustCompileRegex(expr string) *regexutil.Regex {
	re, err := regexutil.NewRegex(expr)
	if err != nil {
		panic(fmt.Errorf("BUG: cannot compile %q: %w", expr, err))
	}
	return re
}
