package logstorage

import (
	"testing"
)

func TestFilterNot(t *testing.T) {
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
				"a foobar",
				"a kjlkjf dfff",
				"a ТЕСТЙЦУК НГКШ ",
				"a !!,23.(!1)",
			},
		},
	}

	// match
	fn := &filterNot{
		f: &filterPhrase{
			fieldName: "foo",
			phrase:    "",
		},
	}
	testFilterMatchForColumns(t, columns, fn, "foo", []int{0, 1, 2, 3, 4, 6, 7, 8, 9})

	fn = &filterNot{
		f: &filterPhrase{
			fieldName: "foo",
			phrase:    "a",
		},
	}
	testFilterMatchForColumns(t, columns, fn, "foo", []int{5})

	fn = &filterNot{
		f: &filterPhrase{
			fieldName: "non-existing-field",
			phrase:    "foobar",
		},
	}
	testFilterMatchForColumns(t, columns, fn, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9})

	fn = &filterNot{
		f: &filterPrefix{
			fieldName: "non-existing-field",
			prefix:    "",
		},
	}
	testFilterMatchForColumns(t, columns, fn, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9})

	fn = &filterNot{
		f: &filterPrefix{
			fieldName: "foo",
			prefix:    "",
		},
	}
	testFilterMatchForColumns(t, columns, fn, "foo", []int{5})

	// mismatch
	fn = &filterNot{
		f: &filterPhrase{
			fieldName: "non-existing-field",
			phrase:    "",
		},
	}
	testFilterMatchForColumns(t, columns, fn, "foo", nil)
}
