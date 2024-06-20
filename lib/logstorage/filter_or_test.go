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

	// non-empty union
	fo := &filterOr{
		filters: []filter{
			&filterPhrase{
				fieldName: "foo",
				phrase:    "23",
			},
			&filterPrefix{
				fieldName: "foo",
				prefix:    "abc",
			},
		},
	}
	testFilterMatchForColumns(t, columns, fo, "foo", []int{2, 6, 9})

	// reverse non-empty union
	fo = &filterOr{
		filters: []filter{
			&filterPrefix{
				fieldName: "foo",
				prefix:    "abc",
			},
			&filterPhrase{
				fieldName: "foo",
				phrase:    "23",
			},
		},
	}
	testFilterMatchForColumns(t, columns, fo, "foo", []int{2, 6, 9})

	// first empty result, second non-empty result
	fo = &filterOr{
		filters: []filter{
			&filterPrefix{
				fieldName: "foo",
				prefix:    "xabc",
			},
			&filterPhrase{
				fieldName: "foo",
				phrase:    "23",
			},
		},
	}
	testFilterMatchForColumns(t, columns, fo, "foo", []int{9})

	// first non-empty result, second empty result
	fo = &filterOr{
		filters: []filter{
			&filterPhrase{
				fieldName: "foo",
				phrase:    "23",
			},
			&filterPrefix{
				fieldName: "foo",
				prefix:    "xabc",
			},
		},
	}
	testFilterMatchForColumns(t, columns, fo, "foo", []int{9})

	// first match all
	fo = &filterOr{
		filters: []filter{
			&filterPhrase{
				fieldName: "foo",
				phrase:    "a",
			},
			&filterPrefix{
				fieldName: "foo",
				prefix:    "23",
			},
		},
	}
	testFilterMatchForColumns(t, columns, fo, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9})

	// second match all
	fo = &filterOr{
		filters: []filter{
			&filterPrefix{
				fieldName: "foo",
				prefix:    "23",
			},
			&filterPhrase{
				fieldName: "foo",
				phrase:    "a",
			},
		},
	}
	testFilterMatchForColumns(t, columns, fo, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9})

	// both empty results
	fo = &filterOr{
		filters: []filter{
			&filterPhrase{
				fieldName: "foo",
				phrase:    "x23",
			},
			&filterPrefix{
				fieldName: "foo",
				prefix:    "xabc",
			},
		},
	}
	testFilterMatchForColumns(t, columns, fo, "foo", nil)
}
