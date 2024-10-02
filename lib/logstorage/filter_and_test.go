package logstorage

import (
	"reflect"
	"testing"
)

func TestFilterAnd(t *testing.T) {
	t.Parallel()

	columns := []column{
		{
			name: "foo",
			values: []string{
				"a foo",
				"a foobar",
				"aa abc a",
				"ca afdf a,foobar baz",
				"a fddf foobarbaz",
				"",
				"a foobar abcdef",
				"a kjlkjf dfff",
				"a ТЕСТЙЦУК НГКШ ",
				"a !!,23.(!1)",
			},
		},
	}

	f := func(qStr string, expectedRowIdxs []int) {
		t.Helper()

		q, err := ParseQuery(qStr)
		if err != nil {
			t.Fatalf("unexpected error in ParseQuery: %s", err)
		}
		testFilterMatchForColumns(t, columns, q.f, "foo", expectedRowIdxs)
	}

	// non-empty intersection
	f(`foo:a AND foo:abc*`, []int{2, 6})

	// reverse non-empty intersection
	f(`foo:abc* AND foo:a`, []int{2, 6})

	// the first filter mismatch
	f(`foo:bc* AND foo:a`, nil)

	// the last filter mismatch
	f(`foo:abc AND foo:foo*`, nil)

	// empty intersection
	f(`foo:foo AND foo:abc*`, nil)
	f(`foo:abc* AND foo:foo`, nil)

	// empty value
	f(`foo:"" AND bar:""`, []int{5})

	// non-existing field with empty value
	f(`foo:foo* AND bar:""`, []int{0, 1, 3, 4, 6})
	f(`bar:"" AND foo:foo*`, []int{0, 1, 3, 4, 6})

	// non-existing field with non-empty value
	f(`foo:foo* AND bar:*`, nil)
	f(`bar:* AND foo:foo*`, nil)

	// https://github.com/VictoriaMetrics/VictoriaMetrics/issues/6554
	f(`foo:"a foo"* AND (foo:="a foobar" OR boo:bbbbbbb)`, []int{1})

	f(`foo:"a foo"* AND (foo:"abcd foobar" OR foo:foobar)`, []int{1, 6})
	f(`(foo:foo* OR bar:baz) AND (bar:x OR foo:a)`, []int{0, 1, 3, 4, 6})
	f(`(foo:foo* OR bar:baz) AND (bar:x OR foo:xyz)`, nil)
	f(`(foo:foo* OR bar:baz) AND (bar:* OR foo:xyz)`, nil)
	f(`(foo:foo* OR bar:baz) AND (bar:"" OR foo:xyz)`, []int{0, 1, 3, 4, 6})

	// negative filters
	f(`foo:foo* AND !foo:~bar`, []int{0})
	f(`foo:foo* AND foo:!~bar`, []int{0})
}

func TestGetCommonTokensForAndFilters(t *testing.T) {
	f := func(qStr string, tokensExpected []fieldTokens) {
		t.Helper()

		q, err := ParseQuery(qStr)
		if err != nil {
			t.Fatalf("unexpected error in ParseQuery: %s", err)
		}
		fa, ok := q.f.(*filterAnd)
		if !ok {
			t.Fatalf("unexpected filter type: %T; want *filterAnd", q.f)
		}
		tokens := getCommonTokensForAndFilters(fa.filters)

		if len(tokens) != len(tokensExpected) {
			t.Fatalf("unexpected len(tokens); got %d; want %d\ntokens\n%#v\ntokensExpected\n%#v", len(tokens), len(tokensExpected), tokens, tokensExpected)
		}
		for i, ft := range tokens {
			ftExpected := tokensExpected[i]
			if ft.field != ftExpected.field {
				t.Fatalf("unexpected field; got %q; want %q\ntokens\n%q\ntokensExpected\n%q", ft.field, ftExpected.field, ft.tokens, ftExpected.tokens)
			}
			if !reflect.DeepEqual(ft.tokens, ftExpected.tokens) {
				t.Fatalf("unexpected tokens for field %q; got %q; want %q", ft.field, ft.tokens, ftExpected.tokens)
			}
		}
	}

	f(`foo AND bar`, []fieldTokens{
		{
			field:  "_msg",
			tokens: []string{"foo", "bar"},
		},
	})

	f(`="foo bar" AND ="a foo"* AND "bar foo" AND "foo bar"* AND ~"foo qwe bar.+" AND seq(x, bar, "foo qwe")`, []fieldTokens{
		{
			field:  "_msg",
			tokens: []string{"foo", "bar", "a", "qwe", "x"},
		},
	})

	// extract common tokens from OR filters
	f(`foo AND (bar OR ~"x bar baz")`, []fieldTokens{
		{
			field:  "_msg",
			tokens: []string{"foo", "bar"},
		},
	})

	// star matches any non-empty token, so it is skipped
	f(`foo bar *`, []fieldTokens{
		{
			field:  "_msg",
			tokens: []string{"foo", "bar"},
		},
	})
	f(`* *`, nil)

	// empty filter must be skipped
	f(`foo "" bar`, []fieldTokens{
		{
			field:  "_msg",
			tokens: []string{"foo", "bar"},
		},
	})
	f(`"" ""`, nil)

	// unknown filters must be skipped
	f(`_time:5m !foo "bar baz" x`, []fieldTokens{
		{
			field:  "_msg",
			tokens: []string{"bar", "baz", "x"},
		},
	})

	// distinct field names
	f(`foo:x bar:"a bc" (foo:y OR (bar:qwe AND foo:"z y a"))`, []fieldTokens{
		{
			field:  "foo",
			tokens: []string{"x", "y"},
		},
		{
			field:  "bar",
			tokens: []string{"a", "bc"},
		},
	})
}
