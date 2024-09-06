package logstorage

import (
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

	// non-empty intersection
	// foo:a AND foo:abc*
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
	// foo:abc* AND foo:a
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
	// foo:bc* AND foo:a
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
	// foo:abc AND foo:foo*
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
	// foo:foo AND foo:abc*
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
	// foo:abc* AND foo:foo
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

	// empty value
	// foo:"" AND bar:""
	fa = &filterAnd{
		filters: []filter{
			&filterExact{
				fieldName: "foo",
				value:     "",
			},
			&filterExact{
				fieldName: "bar",
				value:     "",
			},
		},
	}
	testFilterMatchForColumns(t, columns, fa, "foo", []int{5})

	// non-existing field with empty value
	// foo:foo* AND bar:""
	fa = &filterAnd{
		filters: []filter{
			&filterPrefix{
				fieldName: "foo",
				prefix:    "foo",
			},
			&filterExact{
				fieldName: "bar",
				value:     "",
			},
		},
	}
	testFilterMatchForColumns(t, columns, fa, "foo", []int{0, 1, 3, 4, 6})

	// reverse non-existing field with empty value
	// bar:"" AND foo:foo*
	fa = &filterAnd{
		filters: []filter{
			&filterExact{
				fieldName: "bar",
				value:     "",
			},
			&filterPrefix{
				fieldName: "foo",
				prefix:    "foo",
			},
		},
	}
	testFilterMatchForColumns(t, columns, fa, "foo", []int{0, 1, 3, 4, 6})

	// non-existing field with non-empty value
	// foo:foo* AND bar:*
	fa = &filterAnd{
		filters: []filter{
			&filterPrefix{
				fieldName: "foo",
				prefix:    "foo",
			},
			&filterPrefix{
				fieldName: "bar",
				prefix:    "",
			},
		},
	}
	testFilterMatchForColumns(t, columns, fa, "foo", nil)

	// reverse non-existing field with non-empty value
	// bar:* AND foo:foo*
	fa = &filterAnd{
		filters: []filter{
			&filterPrefix{
				fieldName: "bar",
				prefix:    "",
			},
			&filterPrefix{
				fieldName: "foo",
				prefix:    "foo",
			},
		},
	}
	testFilterMatchForColumns(t, columns, fa, "foo", nil)

	// https://github.com/VictoriaMetrics/VictoriaMetrics/issues/6554
	// foo:"a foo"* AND (foo:="a foobar" OR boo:bbbbbbb)
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

	// foo:"a foo"* AND (foo:"abcd foobar" OR foo:foobar)
	fa = &filterAnd{
		filters: []filter{
			&filterPrefix{
				fieldName: "foo",
				prefix:    "a foo",
			},
			&filterOr{
				filters: []filter{
					&filterPhrase{
						fieldName: "foo",
						phrase:    "abcd foobar",
					},
					&filterPhrase{
						fieldName: "foo",
						phrase:    "foobar",
					},
				},
			},
		},
	}
	testFilterMatchForColumns(t, columns, fa, "foo", []int{1, 6})

	// (foo:foo* OR bar:baz) AND (bar:x OR foo:a)
	fa = &filterAnd{
		filters: []filter{
			&filterOr{
				filters: []filter{
					&filterPrefix{
						fieldName: "foo",
						prefix:    "foo",
					},
					&filterPhrase{
						fieldName: "bar",
						phrase:    "baz",
					},
				},
			},
			&filterOr{
				filters: []filter{
					&filterPhrase{
						fieldName: "bar",
						phrase:    "x",
					},
					&filterPhrase{
						fieldName: "foo",
						phrase:    "a",
					},
				},
			},
		},
	}
	testFilterMatchForColumns(t, columns, fa, "foo", []int{0, 1, 3, 4, 6})

	// (foo:foo* OR bar:baz) AND (bar:x OR foo:xyz)
	fa = &filterAnd{
		filters: []filter{
			&filterOr{
				filters: []filter{
					&filterPrefix{
						fieldName: "foo",
						prefix:    "foo",
					},
					&filterPhrase{
						fieldName: "bar",
						phrase:    "baz",
					},
				},
			},
			&filterOr{
				filters: []filter{
					&filterPhrase{
						fieldName: "bar",
						phrase:    "x",
					},
					&filterPhrase{
						fieldName: "foo",
						phrase:    "xyz",
					},
				},
			},
		},
	}
	testFilterMatchForColumns(t, columns, fa, "foo", nil)

	// (foo:foo* OR bar:baz) AND (bar:* OR foo:xyz)
	fa = &filterAnd{
		filters: []filter{
			&filterOr{
				filters: []filter{
					&filterPrefix{
						fieldName: "foo",
						prefix:    "foo",
					},
					&filterPhrase{
						fieldName: "bar",
						phrase:    "baz",
					},
				},
			},
			&filterOr{
				filters: []filter{
					&filterPrefix{
						fieldName: "bar",
						prefix:    "",
					},
					&filterPhrase{
						fieldName: "foo",
						phrase:    "xyz",
					},
				},
			},
		},
	}
	testFilterMatchForColumns(t, columns, fa, "foo", nil)

	// (foo:foo* OR bar:baz) AND (bar:"" OR foo:xyz)
	fa = &filterAnd{
		filters: []filter{
			&filterOr{
				filters: []filter{
					&filterPrefix{
						fieldName: "foo",
						prefix:    "foo",
					},
					&filterPhrase{
						fieldName: "bar",
						phrase:    "baz",
					},
				},
			},
			&filterOr{
				filters: []filter{
					&filterExact{
						fieldName: "bar",
						value:     "",
					},
					&filterPhrase{
						fieldName: "foo",
						phrase:    "xyz",
					},
				},
			},
		},
	}
	testFilterMatchForColumns(t, columns, fa, "foo", []int{0, 1, 3, 4, 6})
}
