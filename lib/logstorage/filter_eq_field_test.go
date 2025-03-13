package logstorage

import (
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
)

func TestFilterEqField(t *testing.T) {
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
		fe := &filterEqField{
			fieldName:      "foo",
			otherFieldName: "foo",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{0})

		fe = &filterEqField{
			fieldName:      "foo",
			otherFieldName: "bar",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{0})

		fe = &filterEqField{
			fieldName:      "non-existing-column",
			otherFieldName: "other-non-existing-column",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{0})

		// mismatch
		fe = &filterEqField{
			fieldName:      "foo",
			otherFieldName: "baz",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", nil)

		fe = &filterEqField{
			fieldName:      "foo",
			otherFieldName: "non-existing-column",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", nil)

		fe = &filterEqField{
			fieldName:      "non-existing column",
			otherFieldName: "foo",
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
		fe := &filterEqField{
			fieldName:      "foo",
			otherFieldName: "foo",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{0, 1, 2})

		fe = &filterEqField{
			fieldName:      "foo",
			otherFieldName: "bar",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{0, 1, 2})

		fe = &filterEqField{
			fieldName:      "non-existing-column",
			otherFieldName: "non-existing-column",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{0, 1, 2})

		// mismatch
		fe = &filterEqField{
			fieldName:      "foo",
			otherFieldName: "baz",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", nil)

		fe = &filterEqField{
			fieldName:      "foo",
			otherFieldName: "non-existing-column",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", nil)

		fe = &filterEqField{
			fieldName:      "non-existing-column",
			otherFieldName: "foo",
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
					"xfoobar",
					"",
					"",
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
					"x",
					"xfddf foobarbaz",
					"xafoobarbaz",
					"xfoobar",
				},
			},
		}

		// match
		fe := &filterEqField{
			fieldName:      "foo",
			otherFieldName: "foo",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{0, 1, 2, 3, 4, 5, 6})

		fe = &filterEqField{
			fieldName:      "foo",
			otherFieldName: "bar",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{2, 5})

		fe = &filterEqField{
			fieldName:      "non-existing-column",
			otherFieldName: "other-non-existing-column",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{0, 1, 2, 3, 4, 5, 6})

		fe = &filterEqField{
			fieldName:      "foo",
			otherFieldName: "non-existing-column",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{2})

		fe = &filterEqField{
			fieldName:      "non-existing-column",
			otherFieldName: "foo",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{2})

		// mismatch
		fe = &filterEqField{
			fieldName:      "foo",
			otherFieldName: "baz",
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
					"a foo",
					"xa foobar",
					"aa abc a",
					"",
					"xa fddf foobarbaz",
					"",
					"xa foobar baz",
					"a kjlkjf dfff",
					"a ТЕСТЙЦУК НГКШ ",
					"xa !!,23.(!1)",
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
		fe := &filterEqField{
			fieldName:      "foo",
			otherFieldName: "foo",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9})

		fe = &filterEqField{
			fieldName:      "foo",
			otherFieldName: "bar",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{0, 2, 5, 7, 8})

		fe = &filterEqField{
			fieldName:      "non-existing-column",
			otherFieldName: "non-existing-column",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9})

		fe = &filterEqField{
			fieldName:      "foo",
			otherFieldName: "non-existing-column",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{5})

		fe = &filterEqField{
			fieldName:      "non-existing-column",
			otherFieldName: "foo",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{5})

		// mismatch
		fe = &filterEqField{
			fieldName:      "foo",
			otherFieldName: "baz",
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
					"23",
					"12",
					"42",
					"0",
					"10",
					"12",
					"10",
					"2",
					"30",
					"4",
					"50",
				},
			},
			{
				name: "baz",
				values: []string{
					"230",
					"120",
					"20",
					"10",
					"20",
					"120",
					"10",
					"20",
					"30",
					"40",
					"50",
				},
			},
		}

		// match
		fe := &filterEqField{
			fieldName:      "foo",
			otherFieldName: "foo",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10})

		fe = &filterEqField{
			fieldName:      "foo",
			otherFieldName: "bar",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{1, 3, 5, 7, 9})

		fe = &filterEqField{
			fieldName:      "non-existing-column",
			otherFieldName: "other-non-existing-column",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10})

		// mismatch
		fe = &filterEqField{
			fieldName:      "foo",
			otherFieldName: "baz",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", nil)

		fe = &filterEqField{
			fieldName:      "foo",
			otherFieldName: "non-exsiting-column",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", nil)

		fe = &filterEqField{
			fieldName:      "non-existing-column",
			otherFieldName: "foo",
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
					"23",
					"12",
					"2",
					"0",
					"10",
					"12",
					"560",
					"2",
					"43",
					"4",
					"50",
				},
			},
			{
				name: "baz",
				values: []string{
					"1123",
					"112",
					"132",
					"10",
					"10",
					"112",
					"1256",
					"12",
					"13",
					"14",
					"15",
				},
			},
		}

		// match
		fe := &filterEqField{
			fieldName:      "foo",
			otherFieldName: "foo",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10})

		fe = &filterEqField{
			fieldName:      "foo",
			otherFieldName: "bar",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{1, 3, 5, 7, 9})

		fe = &filterEqField{
			fieldName:      "non-existing-column",
			otherFieldName: "non-existing-column",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10})

		// mismatch
		fe = &filterEqField{
			fieldName:      "foo",
			otherFieldName: "baz",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", nil)

		fe = &filterEqField{
			fieldName:      "foo",
			otherFieldName: "non-existing-column",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", nil)

		fe = &filterEqField{
			fieldName:      "non-existing-column",
			otherFieldName: "foo",
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
		fe := &filterEqField{
			fieldName:      "foo",
			otherFieldName: "foo",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10})

		fe = &filterEqField{
			fieldName:      "foo",
			otherFieldName: "bar",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{1, 3, 5, 7, 9})

		fe = &filterEqField{
			fieldName:      "non-existing-column",
			otherFieldName: "non-existing-column",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10})

		// mismatch
		fe = &filterEqField{
			fieldName:      "foo",
			otherFieldName: "baz",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", nil)

		fe = &filterEqField{
			fieldName:      "foo",
			otherFieldName: "non-existing-column",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", nil)

		fe = &filterEqField{
			fieldName:      "non-existing-colun",
			otherFieldName: "foo",
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
		fe := &filterEqField{
			fieldName:      "foo",
			otherFieldName: "foo",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10})

		fe = &filterEqField{
			fieldName:      "foo",
			otherFieldName: "bar",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{1, 3, 5, 7, 9})

		fe = &filterEqField{
			fieldName:      "non-existing-column",
			otherFieldName: "non-existing-column",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10})

		// mismatch
		fe = &filterEqField{
			fieldName:      "foo",
			otherFieldName: "baz",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", nil)

		fe = &filterEqField{
			fieldName:      "foo",
			otherFieldName: "non-existing-column",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", nil)

		fe = &filterEqField{
			fieldName:      "non-existing-column",
			otherFieldName: "foo",
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
					"3123",
					"12",
					"332",
					"0",
					"30",
					"-12",
					"312345678901",
					"2",
					"33",
					"4",
					"35",
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
					"-212",
					"212345678901",
					"22",
					"23",
					"24",
					"25",
				},
			},
		}

		// match
		fe := &filterEqField{
			fieldName:      "foo",
			otherFieldName: "foo",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10})

		fe = &filterEqField{
			fieldName:      "foo",
			otherFieldName: "bar",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{1, 3, 5, 7, 9})

		fe = &filterEqField{
			fieldName:      "non-existing-column",
			otherFieldName: "non-existing-column",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10})

		// mismatch
		fe = &filterEqField{
			fieldName:      "foo",
			otherFieldName: "baz",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", nil)

		fe = &filterEqField{
			fieldName:      "foo",
			otherFieldName: "non-existing-column",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", nil)

		fe = &filterEqField{
			fieldName:      "non-existing-column",
			otherFieldName: "foo",
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
					"23454",
					"-65536",
					"21234.5678901",
					"1",
					"22",
					"3",
					"24",
				},
			},
			{
				name: "baz",
				values: []string{
					"21234",
					"20",
					"23454",
					"-265536",
					"21234.5678901",
					"21",
					"22",
					"23",
					"24",
				},
			},
		}

		// match
		fe := &filterEqField{
			fieldName:      "foo",
			otherFieldName: "foo",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8})

		fe = &filterEqField{
			fieldName:      "foo",
			otherFieldName: "bar",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{1, 3, 5, 7})

		fe = &filterEqField{
			fieldName:      "non-existing-column",
			otherFieldName: "other-non-existing-column",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8})

		// mismatch
		fe = &filterEqField{
			fieldName:      "foo",
			otherFieldName: "baz",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", nil)

		fe = &filterEqField{
			fieldName:      "foo",
			otherFieldName: "non-existing-column",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", nil)

		fe = &filterEqField{
			fieldName:      "non-existing-column",
			otherFieldName: "foo",
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
					"26.66.66.66",
					"7.7.7.7",
				},
			},
			{
				name: "baz",
				values: []string{
					"31.2.3.4",
					"30.0.0.0",
					"37.0.0.1",
					"34.255.255.255",
					"37.0.0.1",
					"37.0.0.1",
					"37.0.4.2",
					"37.0.0.1",
					"32.0.127.6",
					"35.55.55.55",
					"36.66.66.66",
					"37.7.7.7",
				},
			},
		}

		// match
		fe := &filterEqField{
			fieldName:      "foo",
			otherFieldName: "foo",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11})

		fe = &filterEqField{
			fieldName:      "foo",
			otherFieldName: "bar",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{1, 3, 5, 7, 9, 11})

		fe = &filterEqField{
			fieldName:      "non-existing-column",
			otherFieldName: "non-existing-column",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11})

		// mismatch
		fe = &filterEqField{
			fieldName:      "foo",
			otherFieldName: "baz",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", nil)

		fe = &filterEqField{
			fieldName:      "foo",
			otherFieldName: "non-existing-column",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", nil)

		fe = &filterEqField{
			fieldName:      "non-existing-column",
			otherFieldName: "foo",
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
					"2009-01-02T15:04:05.001Z",
					"2009-01-02T15:04:05.002Z",
					"2009-01-02T15:04:05.003Z",
					"2009-01-02T15:04:05.004Z",
					"2009-01-02T15:04:05.005Z",
					"2009-01-02T15:04:05.006Z",
					"2009-01-02T15:04:05.007Z",
					"2009-01-02T15:04:05.008Z",
					"2009-01-02T15:04:05.009Z",
				},
			},
		}

		// match
		fe := &filterEqField{
			fieldName:      "_msg",
			otherFieldName: "_msg",
		}
		testFilterMatchForColumns(t, columns, fe, "_msg", []int{0, 1, 2, 3, 4, 5, 6, 7, 8})

		fe = &filterEqField{
			fieldName:      "_msg",
			otherFieldName: "bar",
		}
		testFilterMatchForColumns(t, columns, fe, "_msg", []int{1, 3, 5, 7})

		fe = &filterEqField{
			fieldName:      "non-existing-column",
			otherFieldName: "non-exsiting-column",
		}
		testFilterMatchForColumns(t, columns, fe, "_msg", []int{0, 1, 2, 3, 4, 5, 6, 7, 8})

		// mismatch
		fe = &filterEqField{
			fieldName:      "_msg",
			otherFieldName: "baz",
		}
		testFilterMatchForColumns(t, columns, fe, "_msg", nil)

		fe = &filterEqField{
			fieldName:      "_msg",
			otherFieldName: "non-existing-column",
		}
		testFilterMatchForColumns(t, columns, fe, "_msg", nil)

		fe = &filterEqField{
			fieldName:      "non-existing-column",
			otherFieldName: "_msg",
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
		fe := &filterEqField{
			fieldName:      "foo",
			otherFieldName: "bar",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{0, 3, 5})

		fe = &filterEqField{
			fieldName:      "non-existing-column",
			otherFieldName: "non-existing-column",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{0, 1, 2, 3, 4, 5})

		fe = &filterEqField{
			fieldName:      "non-existing-column",
			otherFieldName: "bar",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{2})

		fe = &filterEqField{
			fieldName:      "bar",
			otherFieldName: "non-existing-column",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", []int{2})

		// mismatch
		fe = &filterEqField{
			fieldName:      "foo",
			otherFieldName: "non-existing-column",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", nil)

		fe = &filterEqField{
			fieldName:      "non-existing-column",
			otherFieldName: "foo",
		}
		testFilterMatchForColumns(t, columns, fe, "foo", nil)
	})

	// Remove the remaining data files for the test
	fs.MustRemoveAll(t.Name())
}
