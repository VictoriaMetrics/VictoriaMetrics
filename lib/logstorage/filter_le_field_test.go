package logstorage

import (
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
)

func TestFilterLeField(t *testing.T) {
	t.Parallel()

	t.Run("single-row", func(t *testing.T) {
		columns := []column{
			{
				name: "foo",
				values: []string{
					"abc def",
				},
			},
			{
				name: "bar",
				values: []string{
					"abc def",
				},
			},
			{
				name: "baz",
				values: []string{
					"qwerty",
				},
			},
		}

		// match
		fe := &filterLeField{
			fieldName:      "foo",
			otherFieldName: "foo",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{0})

		fe = &filterLeField{
			fieldName:      "foo",
			otherFieldName: "bar",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{0})

		fe = &filterLeField{
			fieldName:      "bar",
			otherFieldName: "foo",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{0})

		fe = &filterLeField{
			fieldName:      "foo",
			otherFieldName: "baz",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{0})

		fe = &filterLeField{
			fieldName:      "non-existing-column",
			otherFieldName: "other-non-existing-column",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{0})

		fe = &filterLeField{
			fieldName:      "non-existing-column",
			otherFieldName: "foo",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{0})

		// mismatch
		fe = &filterLeField{
			fieldName:      "foo",
			otherFieldName: "non-existing-column",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", nil)

		fe = &filterLeField{
			fieldName:          "foo",
			otherFieldName:     "foo",
			excludeEqualValues: true,
		}
		testFilterMatchForColumns(t, columns, fe, "foo", nil)

		fe = &filterLeField{
			fieldName:          "foo",
			otherFieldName:     "bar",
			excludeEqualValues: true,
		}
		testFilterMatchForColumns(t, columns, fe, "foo", nil)

		fe = &filterLeField{
			fieldName:      "baz",
			otherFieldName: "foo",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", nil)

		fe = &filterLeField{
			fieldName:          "bar",
			otherFieldName:     "foo",
			excludeEqualValues: true,
		}
		testFilterMatchForColumns(t, columns, fe, "foo", nil)

		fe = &filterLeField{
			fieldName:          "non-existing-column",
			otherFieldName:     "non-existing-column",
			excludeEqualValues: true,
		}
		testFilterMatchForColumns(t, columns, fe, "foo", nil)
	})

	t.Run("const-column", func(t *testing.T) {
		columns := []column{
			{
				name: "foo",
				values: []string{
					"abc def",
					"abc def",
					"abc def",
				},
			},
			{
				name: "bar",
				values: []string{
					"abc def",
					"abc def",
					"abc def",
				},
			},
			{
				name: "baz",
				values: []string{
					"qwerty",
					"qwerty",
					"qwerty",
				},
			},
		}

		// match
		fe := &filterLeField{
			fieldName:      "foo",
			otherFieldName: "foo",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{0, 1, 2})

		fe = &filterLeField{
			fieldName:      "foo",
			otherFieldName: "bar",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{0, 1, 2})

		fe = &filterLeField{
			fieldName:      "foo",
			otherFieldName: "baz",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{0, 1, 2})

		fe = &filterLeField{
			fieldName:      "non-existing-column",
			otherFieldName: "non-existing-column",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{0, 1, 2})

		fe = &filterLeField{
			fieldName:      "non-existing-column",
			otherFieldName: "foo",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{0, 1, 2})

		// mismatch
		fe = &filterLeField{
			fieldName:      "foo",
			otherFieldName: "non-existing-column",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", nil)

		fe = &filterLeField{
			fieldName:          "foo",
			otherFieldName:     "foo",
			excludeEqualValues: true,
		}
		testFilterMatchForColumns(t, columns, fe, "foo", nil)

		fe = &filterLeField{
			fieldName:          "foo",
			otherFieldName:     "bar",
			excludeEqualValues: true,
		}
		testFilterMatchForColumns(t, columns, fe, "foo", nil)

		fe = &filterLeField{
			fieldName:      "baz",
			otherFieldName: "foo",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", nil)

		fe = &filterLeField{
			fieldName:          "non-existing-column",
			otherFieldName:     "other-non-existing-column",
			excludeEqualValues: true,
		}
		testFilterMatchForColumns(t, columns, fe, "foo", nil)
	})

	t.Run("dict", func(t *testing.T) {
		columns := []column{
			{
				name: "foo",
				values: []string{
					"abc",
					"foobar",
					"",
					"afdf foobar baz",
					"fddf foobarbaz",
					"afoobarbaz",
					"foobar",
				},
			},
			{
				name: "bar",
				values: []string{
					"xabc",
					"foobar",
					"x",
					"afdf foobar baz",
					"xfddf foobarbaz",
					"afoobarbaz",
					"xfoobar",
				},
			},
			{
				name: "baz",
				values: []string{
					"xabc",
					"xfoobar",
					"x",
					"xafdf foobar baz",
					"xfddf foobarbaz",
					"xafoobarbaz",
					"xfoobar",
				},
			},
		}

		// match
		fe := &filterLeField{
			fieldName:      "foo",
			otherFieldName: "foo",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{0, 1, 2, 3, 4, 5, 6})

		fe = &filterLeField{
			fieldName:      "foo",
			otherFieldName: "bar",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{0, 1, 2, 3, 4, 5, 6})

		fe = &filterLeField{
			fieldName:          "foo",
			otherFieldName:     "bar",
			excludeEqualValues: true,
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{0, 2, 4, 6})

		fe = &filterLeField{
			fieldName:      "bar",
			otherFieldName: "foo",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{1, 3, 5})

		fe = &filterLeField{
			fieldName:      "non-existing-column",
			otherFieldName: "other-non-existing-column",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{0, 1, 2, 3, 4, 5, 6})

		fe = &filterLeField{
			fieldName:      "foo",
			otherFieldName: "non-existing-column",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{2})

		fe = &filterLeField{
			fieldName:      "non-existing-column",
			otherFieldName: "foo",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{0, 1, 2, 3, 4, 5, 6})

		fe = &filterLeField{
			fieldName:          "non-existing-column",
			otherFieldName:     "foo",
			excludeEqualValues: true,
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{0, 1, 3, 4, 5, 6})

		// mismatch
		fe = &filterLeField{
			fieldName:          "bar",
			otherFieldName:     "foo",
			excludeEqualValues: true,
		}
		testFilterMatchForColumns(t, columns, fe, "foo", nil)

		fe = &filterLeField{
			fieldName:      "baz",
			otherFieldName: "foo",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", nil)

		fe = &filterLeField{
			fieldName:          "foo",
			otherFieldName:     "non-existing-column",
			excludeEqualValues: true,
		}
		testFilterMatchForColumns(t, columns, fe, "foo", nil)

		fe = &filterLeField{
			fieldName:          "non-existing-column",
			otherFieldName:     "non-existing-column",
			excludeEqualValues: true,
		}
		testFilterMatchForColumns(t, columns, fe, "foo", nil)
	})

	t.Run("strings", func(t *testing.T) {
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
					"a foobar baz",
					"a kjlkjf dfff",
					"a ТЕСТЙЦУК НГКШ ",
					"a !!,23.(!1)",
				},
			},
			{
				name: "bar",
				values: []string{
					"za foo",
					"a foobar",
					"zaa abc a",
					"ca afdf a,foobar baz",
					"za fddf foobarbaz",
					"",
					"za foobar baz",
					"a kjlkjf dfff",
					"za ТЕСТЙЦУК НГКШ ",
					"a !!,23.(!1)",
				},
			},
			{
				name: "baz",
				values: []string{
					"xa foo",
					"xa foobar",
					"xaa abc a",
					"xca afdf a,foobar baz",
					"xa fddf foobarbaz",
					"x",
					"xa foobar baz",
					"xa kjlkjf dfff",
					"xa ТЕСТЙЦУК НГКШ ",
					"xa !!,23.(!1)",
				},
			},
		}

		// match
		fe := &filterLeField{
			fieldName:      "foo",
			otherFieldName: "foo",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9})

		fe = &filterLeField{
			fieldName:      "foo",
			otherFieldName: "bar",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9})

		fe = &filterLeField{
			fieldName:          "foo",
			otherFieldName:     "bar",
			excludeEqualValues: true,
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{0, 2, 4, 6, 8})

		fe = &filterLeField{
			fieldName:      "bar",
			otherFieldName: "foo",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{1, 3, 5, 7, 9})

		fe = &filterLeField{
			fieldName:      "non-existing-column",
			otherFieldName: "non-existing-column",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9})

		fe = &filterLeField{
			fieldName:      "foo",
			otherFieldName: "non-existing-column",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{5})

		fe = &filterLeField{
			fieldName:      "non-existing-column",
			otherFieldName: "foo",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9})

		fe = &filterLeField{
			fieldName:          "non-existing-column",
			otherFieldName:     "foo",
			excludeEqualValues: true,
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{0, 1, 2, 3, 4, 6, 7, 8, 9})

		// mismatch
		fe = &filterLeField{
			fieldName:          "bar",
			otherFieldName:     "foo",
			excludeEqualValues: true,
		}
		testFilterMatchForColumns(t, columns, fe, "foo", nil)

		fe = &filterLeField{
			fieldName:      "baz",
			otherFieldName: "foo",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", nil)

		fe = &filterLeField{
			fieldName:          "foo",
			otherFieldName:     "non-existing-column",
			excludeEqualValues: true,
		}
		testFilterMatchForColumns(t, columns, fe, "foo", nil)

		fe = &filterLeField{
			fieldName:          "non-existing-column",
			otherFieldName:     "other-non-existing-column",
			excludeEqualValues: true,
		}
		testFilterMatchForColumns(t, columns, fe, "foo", nil)
	})

	t.Run("uint8", func(t *testing.T) {
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
			{
				name: "bar",
				values: []string{
					"223",
					"12",
					"232",
					"0",
					"20",
					"12",
					"21",
					"2",
					"23",
					"4",
					"25",
				},
			},
			{
				name: "baz",
				values: []string{
					"223",
					"212",
					"232",
					"20",
					"20",
					"212",
					"21",
					"22",
					"23",
					"24",
					"25",
				},
			},
		}

		// match
		fe := &filterLeField{
			fieldName:      "foo",
			otherFieldName: "foo",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10})

		fe = &filterLeField{
			fieldName:      "foo",
			otherFieldName: "bar",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10})

		fe = &filterLeField{
			fieldName:          "foo",
			otherFieldName:     "bar",
			excludeEqualValues: true,
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{0, 2, 4, 6, 8, 10})

		fe = &filterLeField{
			fieldName:      "bar",
			otherFieldName: "foo",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{1, 3, 5, 7, 9})

		fe = &filterLeField{
			fieldName:      "non-existing-column",
			otherFieldName: "other-non-existing-column",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10})

		fe = &filterLeField{
			fieldName:      "non-existing-column",
			otherFieldName: "foo",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10})

		// mismatch
		fe = &filterLeField{
			fieldName:          "bar",
			otherFieldName:     "foo",
			excludeEqualValues: true,
		}
		testFilterMatchForColumns(t, columns, fe, "foo", nil)

		fe = &filterLeField{
			fieldName:          "baz",
			otherFieldName:     "foo",
			excludeEqualValues: true,
		}
		testFilterMatchForColumns(t, columns, fe, "foo", nil)

		fe = &filterLeField{
			fieldName:      "foo",
			otherFieldName: "non-exsiting-column",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", nil)

		fe = &filterLeField{
			fieldName:          "foo",
			otherFieldName:     "non-exsiting-column",
			excludeEqualValues: true,
		}
		testFilterMatchForColumns(t, columns, fe, "foo", nil)

		fe = &filterLeField{
			fieldName:          "non-existing-column",
			otherFieldName:     "other-non-existing-column",
			excludeEqualValues: true,
		}
		testFilterMatchForColumns(t, columns, fe, "foo", nil)
	})

	t.Run("uint16", func(t *testing.T) {
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
					"256",
					"2",
					"3",
					"4",
					"5",
				},
			},
			{
				name: "bar",
				values: []string{
					"1123",
					"12",
					"132",
					"0",
					"10",
					"12",
					"1256",
					"2",
					"13",
					"4",
					"15",
				},
			},
			{
				name: "baz",
				values: []string{
					"2123",
					"212",
					"232",
					"20",
					"20",
					"212",
					"2256",
					"22",
					"23",
					"24",
					"25",
				},
			},
		}

		// match
		fe := &filterLeField{
			fieldName:      "foo",
			otherFieldName: "foo",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10})

		fe = &filterLeField{
			fieldName:      "foo",
			otherFieldName: "bar",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10})

		fe = &filterLeField{
			fieldName:          "foo",
			otherFieldName:     "bar",
			excludeEqualValues: true,
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{0, 2, 4, 6, 8, 10})

		fe = &filterLeField{
			fieldName:      "bar",
			otherFieldName: "foo",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{1, 3, 5, 7, 9})

		fe = &filterLeField{
			fieldName:      "non-existing-column",
			otherFieldName: "non-existing-column",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10})

		fe = &filterLeField{
			fieldName:      "non-existing-column",
			otherFieldName: "foo",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10})

		// mismatch
		fe = &filterLeField{
			fieldName:          "bar",
			otherFieldName:     "foo",
			excludeEqualValues: true,
		}
		testFilterMatchForColumns(t, columns, fe, "foo", nil)

		fe = &filterLeField{
			fieldName:      "baz",
			otherFieldName: "foo",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", nil)

		fe = &filterLeField{
			fieldName:      "foo",
			otherFieldName: "non-existing-column",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", nil)

		fe = &filterLeField{
			fieldName:          "foo",
			otherFieldName:     "non-existing-column",
			excludeEqualValues: true,
		}
		testFilterMatchForColumns(t, columns, fe, "foo", nil)

		fe = &filterLeField{
			fieldName:          "non-existing-column",
			otherFieldName:     "other-non-existing-column",
			excludeEqualValues: true,
		}
		testFilterMatchForColumns(t, columns, fe, "foo", nil)
	})

	t.Run("uint32", func(t *testing.T) {
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
					"65536",
					"2",
					"3",
					"4",
					"5",
				},
			},
			{
				name: "bar",
				values: []string{
					"1123",
					"12",
					"132",
					"0",
					"10",
					"12",
					"165536",
					"2",
					"13",
					"4",
					"15",
				},
			},
			{
				name: "baz",
				values: []string{
					"2123",
					"212",
					"232",
					"20",
					"20",
					"212",
					"265536",
					"22",
					"23",
					"24",
					"25",
				},
			},
		}

		// match
		fe := &filterLeField{
			fieldName:      "foo",
			otherFieldName: "foo",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10})

		fe = &filterLeField{
			fieldName:      "foo",
			otherFieldName: "bar",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10})

		fe = &filterLeField{
			fieldName:          "foo",
			otherFieldName:     "bar",
			excludeEqualValues: true,
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{0, 2, 4, 6, 8, 10})

		fe = &filterLeField{
			fieldName:      "bar",
			otherFieldName: "foo",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{1, 3, 5, 7, 9})

		fe = &filterLeField{
			fieldName:      "non-existing-column",
			otherFieldName: "non-existing-column",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10})

		fe = &filterLeField{
			fieldName:      "non-existing-colun",
			otherFieldName: "foo",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10})

		// mismatch
		fe = &filterLeField{
			fieldName:          "bar",
			otherFieldName:     "foo",
			excludeEqualValues: true,
		}
		testFilterMatchForColumns(t, columns, fe, "foo", nil)

		fe = &filterLeField{
			fieldName:      "baz",
			otherFieldName: "foo",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", nil)

		fe = &filterLeField{
			fieldName:      "foo",
			otherFieldName: "non-existing-column",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", nil)

		fe = &filterLeField{
			fieldName:          "non-existing-colun",
			otherFieldName:     "other-non-existing-column",
			excludeEqualValues: true,
		}
		testFilterMatchForColumns(t, columns, fe, "foo", nil)
	})

	t.Run("uint64", func(t *testing.T) {
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
					"12345678901",
					"2",
					"3",
					"4",
					"5",
				},
			},
			{
				name: "bar",
				values: []string{
					"1123",
					"12",
					"132",
					"0",
					"10",
					"12",
					"112345678901",
					"2",
					"13",
					"4",
					"15",
				},
			},
			{
				name: "baz",
				values: []string{
					"2123",
					"212",
					"232",
					"20",
					"20",
					"212",
					"212345678901",
					"22",
					"23",
					"24",
					"25",
				},
			},
		}

		// match
		fe := &filterLeField{
			fieldName:      "foo",
			otherFieldName: "foo",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10})

		fe = &filterLeField{
			fieldName:      "foo",
			otherFieldName: "bar",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10})

		fe = &filterLeField{
			fieldName:          "foo",
			otherFieldName:     "bar",
			excludeEqualValues: true,
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{0, 2, 4, 6, 8, 10})

		fe = &filterLeField{
			fieldName:      "bar",
			otherFieldName: "foo",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{1, 3, 5, 7, 9})

		fe = &filterLeField{
			fieldName:      "non-existing-column",
			otherFieldName: "non-existing-column",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10})

		fe = &filterLeField{
			fieldName:      "non-existing-column",
			otherFieldName: "foo",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10})

		// mismatch
		fe = &filterLeField{
			fieldName:          "bar",
			otherFieldName:     "foo",
			excludeEqualValues: true,
		}
		testFilterMatchForColumns(t, columns, fe, "foo", nil)

		fe = &filterLeField{
			fieldName:      "baz",
			otherFieldName: "foo",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", nil)

		fe = &filterLeField{
			fieldName:      "foo",
			otherFieldName: "non-existing-column",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", nil)

		fe = &filterLeField{
			fieldName:          "non-existing-column",
			otherFieldName:     "non-existing-column",
			excludeEqualValues: true,
		}
		testFilterMatchForColumns(t, columns, fe, "foo", nil)
	})

	t.Run("int64", func(t *testing.T) {
		columns := []column{
			{
				name: "foo",
				values: []string{
					"123",
					"12",
					"32",
					"0",
					"0",
					"-12",
					"12345678901",
					"2",
					"3",
					"4",
					"5",
				},
			},
			{
				name: "bar",
				values: []string{
					"1123",
					"12",
					"132",
					"0",
					"10",
					"-12",
					"112345678901",
					"2",
					"13",
					"4",
					"15",
				},
			},
			{
				name: "baz",
				values: []string{
					"2123",
					"212",
					"232",
					"20",
					"20",
					"-2",
					"212345678901",
					"22",
					"23",
					"24",
					"25",
				},
			},
		}

		// match
		fe := &filterLeField{
			fieldName:      "foo",
			otherFieldName: "foo",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10})

		fe = &filterLeField{
			fieldName:      "foo",
			otherFieldName: "bar",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10})

		fe = &filterLeField{
			fieldName:          "foo",
			otherFieldName:     "bar",
			excludeEqualValues: true,
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{0, 2, 4, 6, 8, 10})

		fe = &filterLeField{
			fieldName:      "bar",
			otherFieldName: "foo",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{1, 3, 5, 7, 9})

		fe = &filterLeField{
			fieldName:      "non-existing-column",
			otherFieldName: "non-existing-column",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10})

		fe = &filterLeField{
			fieldName:      "non-existing-column",
			otherFieldName: "foo",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10})

		// mismatch
		fe = &filterLeField{
			fieldName:          "bar",
			otherFieldName:     "foo",
			excludeEqualValues: true,
		}
		testFilterMatchForColumns(t, columns, fe, "foo", nil)

		fe = &filterLeField{
			fieldName:      "baz",
			otherFieldName: "foo",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", nil)

		fe = &filterLeField{
			fieldName:      "foo",
			otherFieldName: "non-existing-column",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", nil)

		fe = &filterLeField{
			fieldName:          "non-existing-column",
			otherFieldName:     "non-existing-column",
			excludeEqualValues: true,
		}
		testFilterMatchForColumns(t, columns, fe, "foo", nil)
	})

	t.Run("float64", func(t *testing.T) {
		columns := []column{
			{
				name: "foo",
				values: []string{
					"1234",
					"0",
					"3454",
					"-65536",
					"1234.5678901",
					"1",
					"2",
					"3",
					"4",
				},
			},
			{
				name: "bar",
				values: []string{
					"11234",
					"0",
					"13454",
					"-65536",
					"11234.5678901",
					"1",
					"12",
					"3",
					"14",
				},
			},
			{
				name: "baz",
				values: []string{
					"21234",
					"20",
					"23454",
					"-5536",
					"21234.5678901",
					"21",
					"22",
					"23",
					"24",
				},
			},
		}

		// match
		fe := &filterLeField{
			fieldName:      "foo",
			otherFieldName: "foo",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8})

		fe = &filterLeField{
			fieldName:      "foo",
			otherFieldName: "bar",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8})

		fe = &filterLeField{
			fieldName:          "foo",
			otherFieldName:     "bar",
			excludeEqualValues: true,
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{0, 2, 4, 6, 8})

		fe = &filterLeField{
			fieldName:      "bar",
			otherFieldName: "foo",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{1, 3, 5, 7})

		fe = &filterLeField{
			fieldName:      "non-existing-column",
			otherFieldName: "other-non-existing-column",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8})

		fe = &filterLeField{
			fieldName:      "non-existing-column",
			otherFieldName: "foo",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8})

		// mismatch
		fe = &filterLeField{
			fieldName:          "bar",
			otherFieldName:     "foo",
			excludeEqualValues: true,
		}
		testFilterMatchForColumns(t, columns, fe, "foo", nil)

		fe = &filterLeField{
			fieldName:      "baz",
			otherFieldName: "foo",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", nil)

		fe = &filterLeField{
			fieldName:      "foo",
			otherFieldName: "non-existing-column",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", nil)

		fe = &filterLeField{
			fieldName:          "non-existing-column",
			otherFieldName:     "non-existing-column",
			excludeEqualValues: true,
		}
		testFilterMatchForColumns(t, columns, fe, "foo", nil)
	})

	t.Run("ipv4", func(t *testing.T) {
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
					"55.55.55.55",
					"66.66.66.66",
					"7.7.7.7",
				},
			},
			{
				name: "bar",
				values: []string{
					"21.2.3.4",
					"0.0.0.0",
					"227.0.0.1",
					"254.255.255.255",
					"227.0.0.1",
					"127.0.0.1",
					"227.0.4.2",
					"127.0.0.1",
					"212.0.127.6",
					"55.55.55.55",
					"76.66.66.66",
					"7.7.7.7",
				},
			},
			{
				name: "baz",
				values: []string{
					"21.2.3.4",
					"20.0.0.0",
					"227.0.0.1",
					"255.255.255.255",
					"227.0.0.1",
					"227.0.0.1",
					"227.0.4.2",
					"227.0.0.1",
					"212.0.127.6",
					"255.55.55.55",
					"76.66.66.66",
					"27.7.7.7",
				},
			},
		}

		// match
		fe := &filterLeField{
			fieldName:      "foo",
			otherFieldName: "foo",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11})

		fe = &filterLeField{
			fieldName:      "foo",
			otherFieldName: "bar",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11})

		fe = &filterLeField{
			fieldName:          "foo",
			otherFieldName:     "bar",
			excludeEqualValues: true,
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{0, 2, 4, 6, 8, 10})

		fe = &filterLeField{
			fieldName:      "bar",
			otherFieldName: "foo",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{1, 3, 5, 7, 9, 11})

		fe = &filterLeField{
			fieldName:      "non-existing-column",
			otherFieldName: "non-existing-column",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11})

		fe = &filterLeField{
			fieldName:      "non-existing-column",
			otherFieldName: "foo",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11})

		// mismatch
		fe = &filterLeField{
			fieldName:          "bar",
			otherFieldName:     "foo",
			excludeEqualValues: true,
		}
		testFilterMatchForColumns(t, columns, fe, "foo", nil)

		fe = &filterLeField{
			fieldName:      "baz",
			otherFieldName: "foo",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", nil)

		fe = &filterLeField{
			fieldName:      "foo",
			otherFieldName: "non-existing-column",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", nil)

		fe = &filterLeField{
			fieldName:          "non-existing-column",
			otherFieldName:     "non-existing-column",
			excludeEqualValues: true,
		}
		testFilterMatchForColumns(t, columns, fe, "foo", nil)
	})

	t.Run("timestamp-iso8601", func(t *testing.T) {
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
			{
				name: "bar",
				values: []string{
					"2007-01-02T15:04:05.001Z",
					"2006-01-02T15:04:05.002Z",
					"2007-01-02T15:04:05.003Z",
					"2006-01-02T15:04:05.004Z",
					"2007-01-02T15:04:05.005Z",
					"2006-01-02T15:04:05.006Z",
					"2007-01-02T15:04:05.007Z",
					"2006-01-02T15:04:05.008Z",
					"2007-01-02T15:04:05.009Z",
				},
			},
			{
				name: "baz",
				values: []string{
					"2007-01-02T15:04:05.001Z",
					"2007-01-02T15:04:05.002Z",
					"2007-01-02T15:04:05.003Z",
					"2007-01-02T15:04:05.004Z",
					"2007-01-02T15:04:05.005Z",
					"2007-01-02T15:04:05.006Z",
					"2007-01-02T15:04:05.007Z",
					"2007-01-02T15:04:05.008Z",
					"2007-01-02T15:04:05.009Z",
				},
			},
		}

		// match
		fe := &filterLeField{
			fieldName:      "_msg",
			otherFieldName: "_msg",
		}
		testFilterMatchForColumns(t, columns, fe, "_msg", []int{0, 1, 2, 3, 4, 5, 6, 7, 8})

		fe = &filterLeField{
			fieldName:      "_msg",
			otherFieldName: "bar",
		}
		testFilterMatchForColumns(t, columns, fe, "_msg", []int{0, 1, 2, 3, 4, 5, 6, 7, 8})

		fe = &filterLeField{
			fieldName:          "_msg",
			otherFieldName:     "bar",
			excludeEqualValues: true,
		}
		testFilterMatchForColumns(t, columns, fe, "_msg", []int{0, 2, 4, 6, 8})

		fe = &filterLeField{
			fieldName:      "bar",
			otherFieldName: "_msg",
		}
		testFilterMatchForColumns(t, columns, fe, "_msg", []int{1, 3, 5, 7})

		fe = &filterLeField{
			fieldName:      "non-existing-column",
			otherFieldName: "non-exsiting-column",
		}
		testFilterMatchForColumns(t, columns, fe, "_msg", []int{0, 1, 2, 3, 4, 5, 6, 7, 8})

		fe = &filterLeField{
			fieldName:      "non-existing-column",
			otherFieldName: "_msg",
		}
		testFilterMatchForColumns(t, columns, fe, "_msg", []int{0, 1, 2, 3, 4, 5, 6, 7, 8})

		// mismatch
		fe = &filterLeField{
			fieldName:          "bar",
			otherFieldName:     "_msg",
			excludeEqualValues: true,
		}
		testFilterMatchForColumns(t, columns, fe, "_msg", nil)

		fe = &filterLeField{
			fieldName:      "baz",
			otherFieldName: "_msg",
		}
		testFilterMatchForColumns(t, columns, fe, "_msg", nil)

		fe = &filterLeField{
			fieldName:      "_msg",
			otherFieldName: "non-existing-column",
		}
		testFilterMatchForColumns(t, columns, fe, "_msg", nil)

		fe = &filterLeField{
			fieldName:          "non-existing-column",
			otherFieldName:     "non-existing-column",
			excludeEqualValues: true,
		}
		testFilterMatchForColumns(t, columns, fe, "_msg", nil)
	})

	t.Run("mixed-columns", func(t *testing.T) {
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
				},
			},
			{
				name: "bar",
				values: []string{
					"1.2.3.4",
					"1.0.0.0",
					"",
					"254.255.255.255",
					"foobar",
					"127.0.0.1",
				},
			},
		}

		// match
		fe := &filterLeField{
			fieldName:      "foo",
			otherFieldName: "bar",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{0, 1, 3, 4, 5})

		fe = &filterLeField{
			fieldName:          "foo",
			otherFieldName:     "bar",
			excludeEqualValues: true,
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{1, 4})

		fe = &filterLeField{
			fieldName:      "bar",
			otherFieldName: "foo",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{0, 2, 3, 5})

		fe = &filterLeField{
			fieldName:          "bar",
			otherFieldName:     "foo",
			excludeEqualValues: true,
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{2})

		fe = &filterLeField{
			fieldName:      "non-existing-column",
			otherFieldName: "non-existing-column",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{0, 1, 2, 3, 4, 5})

		fe = &filterLeField{
			fieldName:      "non-existing-column",
			otherFieldName: "bar",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{0, 1, 2, 3, 4, 5})

		fe = &filterLeField{
			fieldName:          "non-existing-column",
			otherFieldName:     "bar",
			excludeEqualValues: true,
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{0, 1, 3, 4, 5})

		fe = &filterLeField{
			fieldName:      "bar",
			otherFieldName: "non-existing-column",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{2})

		fe = &filterLeField{
			fieldName:      "non-existing-column",
			otherFieldName: "foo",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{0, 1, 2, 3, 4, 5})

		// mismatch
		fe = &filterLeField{
			fieldName:      "foo",
			otherFieldName: "non-existing-column",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", nil)

		fe = &filterLeField{
			fieldName:          "bar",
			otherFieldName:     "non-existing-column",
			excludeEqualValues: true,
		}
		testFilterMatchForColumns(t, columns, fe, "foo", nil)
	})

	// Remove the remaining data files for the test
	fs.MustRemoveAll(t.Name())
}
