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
	// foo:23 OR foo:abc
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
	// foo:abc OR foo:23
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
	// foo:xabc* OR foo:23
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
	// foo:23 OR foo:xabc*
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
	// foo:a OR foo:23
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
	// foo:23 OR foo:a
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
	// foo:x23 OR foo:xabc
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

	// non-existing column (last)
	// foo:23 OR bar:xabc*
	fo = &filterOr{
		filters: []filter{
			&filterPhrase{
				fieldName: "foo",
				phrase:    "23",
			},
			&filterPrefix{
				fieldName: "bar",
				prefix:    "xabc",
			},
		},
	}
	testFilterMatchForColumns(t, columns, fo, "foo", []int{9})

	// non-existing column (first)
	// bar:xabc* OR foo:23
	fo = &filterOr{
		filters: []filter{
			&filterPhrase{
				fieldName: "foo",
				phrase:    "23",
			},
			&filterPrefix{
				fieldName: "bar",
				prefix:    "xabc",
			},
		},
	}
	testFilterMatchForColumns(t, columns, fo, "foo", []int{9})

	// (foo:23 AND bar:"") OR (foo:foo AND bar:*)
	fo = &filterOr{
		filters: []filter{
			&filterAnd{
				filters: []filter{
					&filterPhrase{
						fieldName: "foo",
						phrase:    "23",
					},
					&filterExact{
						fieldName: "bar",
						value:     "",
					},
				},
			},
			&filterAnd{
				filters: []filter{
					&filterPhrase{
						fieldName: "foo",
						phrase:    "foo",
					},
					&filterPrefix{
						fieldName: "bar",
						prefix:    "",
					},
				},
			},
		},
	}
	testFilterMatchForColumns(t, columns, fo, "foo", []int{9})

	// (foo:23 AND bar:"") OR (foo:foo AND bar:"")
	fo = &filterOr{
		filters: []filter{
			&filterAnd{
				filters: []filter{
					&filterPhrase{
						fieldName: "foo",
						phrase:    "23",
					},
					&filterExact{
						fieldName: "bar",
						value:     "",
					},
				},
			},
			&filterAnd{
				filters: []filter{
					&filterPhrase{
						fieldName: "foo",
						phrase:    "foo",
					},
					&filterExact{
						fieldName: "bar",
						value:     "",
					},
				},
			},
		},
	}
	testFilterMatchForColumns(t, columns, fo, "foo", []int{0, 9})

	// (foo:23 AND bar:"") OR (foo:foo AND baz:"")
	fo = &filterOr{
		filters: []filter{
			&filterAnd{
				filters: []filter{
					&filterPhrase{
						fieldName: "foo",
						phrase:    "23",
					},
					&filterExact{
						fieldName: "bar",
						value:     "",
					},
				},
			},
			&filterAnd{
				filters: []filter{
					&filterPhrase{
						fieldName: "foo",
						phrase:    "foo",
					},
					&filterExact{
						fieldName: "baz",
						value:     "",
					},
				},
			},
		},
	}
	testFilterMatchForColumns(t, columns, fo, "foo", []int{0, 9})

	// (foo:23 AND bar:abc) OR (foo:foo AND bar:"")
	fo = &filterOr{
		filters: []filter{
			&filterAnd{
				filters: []filter{
					&filterPhrase{
						fieldName: "foo",
						phrase:    "23",
					},
					&filterPhrase{
						fieldName: "bar",
						phrase:    "abc",
					},
				},
			},
			&filterAnd{
				filters: []filter{
					&filterPhrase{
						fieldName: "foo",
						phrase:    "foo",
					},
					&filterExact{
						fieldName: "bar",
						value:     "",
					},
				},
			},
		},
	}
	testFilterMatchForColumns(t, columns, fo, "foo", []int{0})

	// (foo:23 AND bar:abc) OR (foo:foo AND bar:*)
	fo = &filterOr{
		filters: []filter{
			&filterAnd{
				filters: []filter{
					&filterPhrase{
						fieldName: "foo",
						phrase:    "23",
					},
					&filterPhrase{
						fieldName: "bar",
						phrase:    "abc",
					},
				},
			},
			&filterAnd{
				filters: []filter{
					&filterPhrase{
						fieldName: "foo",
						phrase:    "foo",
					},
					&filterPrefix{
						fieldName: "bar",
						prefix:    "",
					},
				},
			},
		},
	}
	testFilterMatchForColumns(t, columns, fo, "foo", nil)
}
