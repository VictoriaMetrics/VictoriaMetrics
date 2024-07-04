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

	//non-empty intersection
	fa := &filterAnd{
		filters: []filter{
			&filterPhrase{
				fieldName: "foo",
				phrase:    "a",
			},
			&filterPrefix{
				fieldName: "foo",
				prefix:    "abc",
			},
		},
	}
	testFilterMatchForColumns(t, columns, fa, "foo", []int{2, 6})

	// reverse non-empty intersection
	fa = &filterAnd{
		filters: []filter{
			&filterPrefix{
				fieldName: "foo",
				prefix:    "abc",
			},
			&filterPhrase{
				fieldName: "foo",
				phrase:    "a",
			},
		},
	}
	testFilterMatchForColumns(t, columns, fa, "foo", []int{2, 6})

	// the first filter mismatch
	fa = &filterAnd{
		filters: []filter{
			&filterPrefix{
				fieldName: "foo",
				prefix:    "bc",
			},
			&filterPhrase{
				fieldName: "foo",
				phrase:    "a",
			},
		},
	}
	testFilterMatchForColumns(t, columns, fa, "foo", nil)

	// the last filter mismatch
	fa = &filterAnd{
		filters: []filter{
			&filterPhrase{
				fieldName: "foo",
				phrase:    "abc",
			},
			&filterPrefix{
				fieldName: "foo",
				prefix:    "foo",
			},
		},
	}
	testFilterMatchForColumns(t, columns, fa, "foo", nil)

	// empty intersection
	fa = &filterAnd{
		filters: []filter{
			&filterPhrase{
				fieldName: "foo",
				phrase:    "foo",
			},
			&filterPrefix{
				fieldName: "foo",
				prefix:    "abc",
			},
		},
	}
	testFilterMatchForColumns(t, columns, fa, "foo", nil)

	// reverse empty intersection
	fa = &filterAnd{
		filters: []filter{
			&filterPrefix{
				fieldName: "foo",
				prefix:    "abc",
			},
			&filterPhrase{
				fieldName: "foo",
				phrase:    "foo",
			},
		},
	}
	testFilterMatchForColumns(t, columns, fa, "foo", nil)

	fa = &filterAnd{
		filters: []filter{
			&filterPrefix{
				fieldName: "foo",
				prefix:    "a foo",
			},
			&filterOr{
				filters: []filter{
					&filterExact{
						fieldName: "foo",
						value:     "a foobar",
					},
					&filterExact{
						fieldName: "boo",
						value:     "bbbbbbb",
					},
				},
			},
		},
	}
	testFilterMatchForColumns(t, columns, fa, "foo", []int{1})
}

func TestGetFilterTokens(t *testing.T) {
	// check filterAnd and filterOr
	fa := &filterAnd{
		filters: []filter{
			&filterExact{
				fieldName: "foo",
				value:     "baz",
			},
			&filterOr{
				filters: []filter{
					&filterExact{
						fieldName: "foo",
						value:     "a",
					},
					&filterExact{
						fieldName: "foo",
						value:     "b",
					},
				},
			},
		},
	}
	andToken := fa.getByFieldTokens()
	if !reflect.DeepEqual([]fieldTokens{
		{
			field: "foo",
			tokens: []string{
				"baz",
			},
		},
	}, andToken) {
		t.Fatalf("unexpected tokens %v", andToken)
	}
}
