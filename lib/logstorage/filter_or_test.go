package logstorage

import (
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
