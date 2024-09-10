package logstorage

import (
	"reflect"
	"testing"
)

func TestFilterOr(t *testing.T) {
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
				"a",
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

	// non-empty union
	f(`foo:23 OR foo:abc*`, []int{2, 6, 9})
	f(`foo:abc* OR foo:23`, []int{2, 6, 9})

	// first empty result, second non-empty result
	f(`foo:xabc* OR foo:23`, []int{9})

	// first non-empty result, second empty result
	f(`foo:23 OR foo:xabc*`, []int{9})

	// first match all
	f(`foo:a OR foo:23`, []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9})

	// second match all
	f(`foo:23 OR foo:a`, []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9})

	// both empty results
	f(`foo:x23 OR foo:xabc`, nil)

	// non-existing column (last)
	f(`foo:23 OR bar:xabc*`, []int{9})

	// non-existing column (first)
	f(`bar:xabc* OR foo:23`, []int{9})

	f(`(foo:23 AND bar:"") OR (foo:foo AND bar:*)`, []int{9})
	f(`(foo:23 AND bar:"") OR (foo:foo AND bar:"")`, []int{0, 9})
	f(`(foo:23 AND bar:"") OR (foo:foo AND baz:"")`, []int{0, 9})
	f(`(foo:23 AND bar:abc) OR (foo:foo AND bar:"")`, []int{0})
	f(`(foo:23 AND bar:abc) OR (foo:foo AND bar:*)`, nil)

	// negative filter
	f(`foo:baz or !foo:~foo`, []int{2, 3, 5, 7, 8, 9})
	f(`foo:baz or foo:!~foo`, []int{2, 3, 5, 7, 8, 9})
}

func TestGetCommonTokensForOrFilters(t *testing.T) {
	f := func(qStr string, tokensExpected []fieldTokens) {
		t.Helper()

		q, err := ParseQuery(qStr)
		if err != nil {
			t.Fatalf("unexpected error in ParseQuery: %s", err)
		}
		fo, ok := q.f.(*filterOr)
		if !ok {
			t.Fatalf("unexpected filter type: %T; want *filterOr", q.f)
		}
		tokens := getCommonTokensForOrFilters(fo.filters)

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

	// no common tokens
	f(`foo OR bar`, nil)

	// star filter matches non-empty value; it is skipped, since one of OR filters may contain an empty filter
	f(`* OR foo`, nil)
	f(`foo or *`, nil)
	f(`* or *`, nil)
	f(`"" or * or foo`, nil)

	// empty filter suppresses all the common tokens.
	f(`"" or foo or "foo bar"`, nil)
	f(`foo or ""`, nil)
	f(`"" or ""`, nil)

	// common foo token
	f(`foo OR "foo bar" OR ="a foo" OR ="foo bar"* OR "bar foo "* OR seq("bar foo", "baz") OR ~"a.+ foo bar"`, []fieldTokens{
		{
			field:  "_msg",
			tokens: []string{"foo"},
		},
	})

	// regexp ending on foo token doesn't contain foo token, since it may match foobar.
	f(`foo OR "foo bar" OR ="a foo" OR ="foo bar"* OR "bar foo "* OR seq("bar foo", "baz") OR ~"a.+ foo"`, nil)

	// regexp starting from foo token doesn't contain foo token, since it may match barfoo.
	f(`foo OR "foo bar" OR ="a foo" OR ="foo bar"* OR "bar foo "* OR seq("bar foo", "baz") OR ~"foo bar"`, nil)

	// prefix filter ending on foo doesn't contain foo token, since it may match foobar.
	f(`foo OR "foo bar" OR ="a foo" OR ="foo bar"* OR "bar foo"* OR seq("bar foo", "baz") OR ~"a.+ foo bar"`, nil)

	// bar and foo are common tokens
	f(`"bar foo baz" OR (foo AND "bar x" AND foobar)`, []fieldTokens{
		{
			field:  "_msg",
			tokens: []string{"bar", "foo"},
		},
	})

	// bar and foo are common tokens, x:foobar should be ignored, since it doesn't present in every OR filter
	f(`"bar foo baz" OR (foo AND "bar x" AND x:foobar)`, []fieldTokens{
		{
			field:  "_msg",
			tokens: []string{"bar", "foo"},
		},
	})

	// common tokens for distinct fields
	f(`(foo AND x:a) OR (x:"b a c" AND ~"aaa foo ")`, []fieldTokens{
		{
			field:  "_msg",
			tokens: []string{"foo"},
		},
		{
			field:  "x",
			tokens: []string{"a"},
		},
	})

	// zero common tokens for distinct fields
	f(`(foo AND x:a) OR (x:"b c" AND ~"aaa foo" AND y:z)`, nil)

	// negative filter removes all the matching
	f(`foo OR !"foo bar"`, nil)

	// time filter removes all the matching
	f(`_time:5m or foo`, nil)
}
