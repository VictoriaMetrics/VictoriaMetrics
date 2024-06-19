package logstorage

import (
	"testing"
)

func TestFilterExact(t *testing.T) {
	t.Parallel()

	t.Run("single-row", func(t *testing.T) {
		t.Parallel()

		columns := []column{
			{
				name: "foo",
				values: []string{
					"abc def",
				},
			},
		}

		// match
		fe := &filterExact{
			fieldName: "foo",
			value:     "abc def",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{0})

		fe = &filterExact{
			fieldName: "non-existing-column",
			value:     "",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{0})

		// mismatch
		fe = &filterExact{
			fieldName: "foo",
			value:     "abc",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", nil)

		fe = &filterExact{
			fieldName: "non-existing column",
			value:     "abc",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", nil)
	})

	t.Run("const-column", func(t *testing.T) {
		t.Parallel()

		columns := []column{
			{
				name: "foo",
				values: []string{
					"abc def",
					"abc def",
					"abc def",
				},
			},
		}

		// match
		fe := &filterExact{
			fieldName: "foo",
			value:     "abc def",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{0, 1, 2})

		fe = &filterExact{
			fieldName: "non-existing-column",
			value:     "",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{0, 1, 2})

		// mismatch
		fe = &filterExact{
			fieldName: "foo",
			value:     "foobar",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", nil)

		fe = &filterExact{
			fieldName: "foo",
			value:     "",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", nil)

		fe = &filterExact{
			fieldName: "non-existing column",
			value:     "x",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", nil)
	})

	t.Run("dict", func(t *testing.T) {
		t.Parallel()

		columns := []column{
			{
				name: "foo",
				values: []string{
					"",
					"foobar",
					"abc",
					"afdf foobar baz",
					"fddf foobarbaz",
					"afoobarbaz",
					"foobar",
				},
			},
		}

		// match
		fe := &filterExact{
			fieldName: "foo",
			value:     "foobar",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{1, 6})

		fe = &filterExact{
			fieldName: "foo",
			value:     "",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{0})

		// mismatch
		fe = &filterExact{
			fieldName: "foo",
			value:     "baz",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", nil)

		fe = &filterExact{
			fieldName: "non-existing column",
			value:     "foobar",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", nil)
	})

	t.Run("strings", func(t *testing.T) {
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
					"a afoobarbaz",
					"a foobar baz",
					"a kjlkjf dfff",
					"a ТЕСТЙЦУК НГКШ ",
					"a !!,23.(!1)",
				},
			},
		}

		// match
		fe := &filterExact{
			fieldName: "foo",
			value:     "aa abc a",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{2})

		fe = &filterExact{
			fieldName: "non-existing-column",
			value:     "",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9})

		// mismatch
		fe = &filterExact{
			fieldName: "foo",
			value:     "aa a",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", nil)

		fe = &filterExact{
			fieldName: "foo",
			value:     "fooaaazz a",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", nil)

		fe = &filterExact{
			fieldName: "foo",
			value:     "",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", nil)
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
		fe := &filterExact{
			fieldName: "foo",
			value:     "12",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{1, 5})

		fe = &filterExact{
			fieldName: "non-existing-column",
			value:     "",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10})

		// mismatch
		fe = &filterExact{
			fieldName: "foo",
			value:     "bar",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", nil)

		fe = &filterExact{
			fieldName: "foo",
			value:     "",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", nil)

		fe = &filterExact{
			fieldName: "foo",
			value:     "33",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", nil)
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
					"12",
					"256",
					"2",
					"3",
					"4",
					"5",
				},
			},
		}

		// match
		fe := &filterExact{
			fieldName: "foo",
			value:     "12",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{1, 5})

		fe = &filterExact{
			fieldName: "non-existing-column",
			value:     "",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10})

		// mismatch
		fe = &filterExact{
			fieldName: "foo",
			value:     "bar",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", nil)

		fe = &filterExact{
			fieldName: "foo",
			value:     "",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", nil)

		fe = &filterExact{
			fieldName: "foo",
			value:     "33",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", nil)
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
					"12",
					"65536",
					"2",
					"3",
					"4",
					"5",
				},
			},
		}

		// match
		fe := &filterExact{
			fieldName: "foo",
			value:     "12",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{1, 5})

		fe = &filterExact{
			fieldName: "non-existing-column",
			value:     "",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10})

		// mismatch
		fe = &filterExact{
			fieldName: "foo",
			value:     "bar",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", nil)

		fe = &filterExact{
			fieldName: "foo",
			value:     "",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", nil)

		fe = &filterExact{
			fieldName: "foo",
			value:     "33",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", nil)
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
					"12",
					"12345678901",
					"2",
					"3",
					"4",
					"5",
				},
			},
		}

		// match
		fe := &filterExact{
			fieldName: "foo",
			value:     "12",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{1, 5})

		fe = &filterExact{
			fieldName: "non-existing-column",
			value:     "",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10})

		// mismatch
		fe = &filterExact{
			fieldName: "foo",
			value:     "bar",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", nil)

		fe = &filterExact{
			fieldName: "foo",
			value:     "",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", nil)

		fe = &filterExact{
			fieldName: "foo",
			value:     "33",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", nil)
	})

	t.Run("float64", func(t *testing.T) {
		t.Parallel()

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
		}

		// match
		fe := &filterExact{
			fieldName: "foo",
			value:     "1234",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{0})

		fe = &filterExact{
			fieldName: "foo",
			value:     "1234.5678901",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{4})

		fe = &filterExact{
			fieldName: "foo",
			value:     "-65536",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{3})

		fe = &filterExact{
			fieldName: "non-existing-column",
			value:     "",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8})

		// mismatch
		fe = &filterExact{
			fieldName: "foo",
			value:     "bar",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", nil)

		fe = &filterExact{
			fieldName: "foo",
			value:     "65536",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", nil)

		fe = &filterExact{
			fieldName: "foo",
			value:     "",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", nil)

		fe = &filterExact{
			fieldName: "foo",
			value:     "123",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", nil)

		fe = &filterExact{
			fieldName: "foo",
			value:     "12345678901234567890",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", nil)
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
					"55.55.55.55",
					"66.66.66.66",
					"7.7.7.7",
				},
			},
		}

		// match
		fe := &filterExact{
			fieldName: "foo",
			value:     "127.0.0.1",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{2, 4, 5, 7})

		fe = &filterExact{
			fieldName: "non-existing-column",
			value:     "",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11})

		// mismatch
		fe = &filterExact{
			fieldName: "foo",
			value:     "bar",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", nil)

		fe = &filterExact{
			fieldName: "foo",
			value:     "",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", nil)

		fe = &filterExact{
			fieldName: "foo",
			value:     "127.0",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", nil)

		fe = &filterExact{
			fieldName: "foo",
			value:     "255.255.255.255",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", nil)
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
		fe := &filterExact{
			fieldName: "_msg",
			value:     "2006-01-02T15:04:05.005Z",
		}
		testFilterMatchForColumns(t, columns, fe, "_msg", []int{4})

		fe = &filterExact{
			fieldName: "non-existing-column",
			value:     "",
		}
		testFilterMatchForColumns(t, columns, fe, "_msg", []int{0, 1, 2, 3, 4, 5, 6, 7, 8})

		// mimatch
		fe = &filterExact{
			fieldName: "_msg",
			value:     "bar",
		}
		testFilterMatchForColumns(t, columns, fe, "_msg", nil)

		fe = &filterExact{
			fieldName: "_msg",
			value:     "",
		}
		testFilterMatchForColumns(t, columns, fe, "_msg", nil)

		fe = &filterExact{
			fieldName: "_msg",
			value:     "2006-03-02T15:04:05.005Z",
		}
		testFilterMatchForColumns(t, columns, fe, "_msg", nil)
	})
}
