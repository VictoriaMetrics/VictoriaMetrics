package logstorage

import (
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
)

func TestFilterValueType(t *testing.T) {
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
				name: "other column",
				values: []string{
					"asdfdsf",
				},
			},
		}

		// match
		pv := &filterValueType{
			fieldName: "foo",
			valueType: "const",
		}
		testFilterMatchForColumns(t, columns, pv, "foo", []int{0})

		// mismatch
		pv = &filterValueType{
			fieldName: "foo",
			valueType: "dict",
		}
		testFilterMatchForColumns(t, columns, pv, "foo", nil)

		pv = &filterValueType{
			fieldName: "foo",
			valueType: "non-existing-type",
		}
		testFilterMatchForColumns(t, columns, pv, "foo", nil)

		pv = &filterValueType{
			fieldName: "bar",
			valueType: "const",
		}
		testFilterMatchForColumns(t, columns, pv, "foo", nil)

		pv = &filterValueType{
			fieldName: "",
			valueType: "const",
		}
		testFilterMatchForColumns(t, columns, pv, "foo", nil)
	})

	t.Run("const-column", func(t *testing.T) {
		columns := []column{
			{
				name: "other-column",
				values: []string{
					"x",
					"x",
					"x",
				},
			},
			{
				name: "foo",
				values: []string{
					"abc def",
					"abc def",
					"abc def",
				},
			},
			{
				name: "_msg",
				values: []string{
					"1 2 3",
					"1 2 3",
					"1 2 3",
				},
			},
		}

		// match
		pv := &filterValueType{
			fieldName: "foo",
			valueType: "const",
		}
		testFilterMatchForColumns(t, columns, pv, "foo", []int{0, 1, 2})

		pv = &filterValueType{
			fieldName: "",
			valueType: "const",
		}
		testFilterMatchForColumns(t, columns, pv, "foo", []int{0, 1, 2})

		pv = &filterValueType{
			fieldName: "_msg",
			valueType: "const",
		}
		testFilterMatchForColumns(t, columns, pv, "foo", []int{0, 1, 2})

		pv = &filterValueType{
			fieldName: "other-column",
			valueType: "const",
		}
		testFilterMatchForColumns(t, columns, pv, "foo", []int{0, 1, 2})

		// mismatch
		pv = &filterValueType{
			fieldName: "foo",
			valueType: "non-existing-type",
		}
		testFilterMatchForColumns(t, columns, pv, "foo", nil)

		pv = &filterValueType{
			fieldName: "foo",
			valueType: "dict",
		}
		testFilterMatchForColumns(t, columns, pv, "foo", nil)

		pv = &filterValueType{
			fieldName: "foo",
			valueType: "",
		}
		testFilterMatchForColumns(t, columns, pv, "foo", nil)

		pv = &filterValueType{
			fieldName: "other-column",
			valueType: "dict",
		}
		testFilterMatchForColumns(t, columns, pv, "foo", nil)

		pv = &filterValueType{
			fieldName: "",
			valueType: "dict",
		}
		testFilterMatchForColumns(t, columns, pv, "foo", nil)

		pv = &filterValueType{
			fieldName: "bar",
			valueType: "const",
		}
		testFilterMatchForColumns(t, columns, pv, "foo", nil)
	})

	t.Run("dict", func(t *testing.T) {
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
		pv := &filterValueType{
			fieldName: "foo",
			valueType: "dict",
		}
		testFilterMatchForColumns(t, columns, pv, "foo", []int{0, 1, 2, 3, 4, 5, 6})

		// mismatch
		pv = &filterValueType{
			fieldName: "foo",
			valueType: "const",
		}
		testFilterMatchForColumns(t, columns, pv, "foo", nil)
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
					"a afoobarbaz",
					"a foobar",
					"a kjlkjf dfff",
					"a ТЕСТЙЦУК НГКШ ",
					"a !!,23.(!1)",
				},
			},
		}

		// match
		pv := &filterValueType{
			fieldName: "foo",
			valueType: "string",
		}
		testFilterMatchForColumns(t, columns, pv, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9})

		// mismatch
		pv = &filterValueType{
			fieldName: "foo",
			valueType: "dict",
		}
		testFilterMatchForColumns(t, columns, pv, "foo", nil)
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
		}

		// match
		pv := &filterValueType{
			fieldName: "foo",
			valueType: "uint8",
		}
		testFilterMatchForColumns(t, columns, pv, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10})

		// mismatch
		pv = &filterValueType{
			fieldName: "foo",
			valueType: "string",
		}
		testFilterMatchForColumns(t, columns, pv, "foo", nil)
	})

	t.Run("uint16", func(t *testing.T) {
		columns := []column{
			{
				name: "foo",
				values: []string{
					"1234",
					"0",
					"3454",
					"65535",
					"1234",
					"1",
					"2",
					"3",
					"4",
					"5",
				},
			},
		}

		// match
		pv := &filterValueType{
			fieldName: "foo",
			valueType: "uint16",
		}
		testFilterMatchForColumns(t, columns, pv, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9})

		// mismatch
		pv = &filterValueType{
			fieldName: "foo",
			valueType: "uint8",
		}
		testFilterMatchForColumns(t, columns, pv, "foo", nil)
	})

	t.Run("uint32", func(t *testing.T) {
		columns := []column{
			{
				name: "foo",
				values: []string{
					"1234",
					"0",
					"3454",
					"65536",
					"1234",
					"1",
					"2",
					"3",
					"4",
					"5",
				},
			},
		}

		// match
		pv := &filterValueType{
			fieldName: "foo",
			valueType: "uint32",
		}
		testFilterMatchForColumns(t, columns, pv, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9})

		// mismatch
		pv = &filterValueType{
			fieldName: "foo",
			valueType: "uint16",
		}
		testFilterMatchForColumns(t, columns, pv, "foo", nil)
	})

	t.Run("uint64", func(t *testing.T) {
		columns := []column{
			{
				name: "foo",
				values: []string{
					"1234",
					"0",
					"3454",
					"65536",
					"12345678901",
					"1",
					"2",
					"3",
					"4",
				},
			},
		}

		// match
		pv := &filterValueType{
			fieldName: "foo",
			valueType: "uint64",
		}
		testFilterMatchForColumns(t, columns, pv, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8})

		// mismatch
		pv = &filterValueType{
			fieldName: "foo",
			valueType: "uint32",
		}
		testFilterMatchForColumns(t, columns, pv, "foo", nil)
	})

	t.Run("int64", func(t *testing.T) {
		columns := []column{
			{
				name: "foo",
				values: []string{
					"1234",
					"0",
					"3454",
					"65536",
					"12345678901",
					"1",
					"-2",
					"3",
					"4",
				},
			},
		}

		// match
		pv := &filterValueType{
			fieldName: "foo",
			valueType: "int64",
		}
		testFilterMatchForColumns(t, columns, pv, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8})

		// mismatch
		pv = &filterValueType{
			fieldName: "foo",
			valueType: "uint64",
		}
		testFilterMatchForColumns(t, columns, pv, "foo", nil)
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
		}

		// match
		pv := &filterValueType{
			fieldName: "foo",
			valueType: "float64",
		}
		testFilterMatchForColumns(t, columns, pv, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8})

		// mismatch
		pv = &filterValueType{
			fieldName: "foo",
			valueType: "uint64",
		}
		testFilterMatchForColumns(t, columns, pv, "foo", nil)
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
		}

		// match
		pv := &filterValueType{
			fieldName: "foo",
			valueType: "ipv4",
		}
		testFilterMatchForColumns(t, columns, pv, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11})

		// mismatch
		pv = &filterValueType{
			fieldName: "foo",
			valueType: "string",
		}
		testFilterMatchForColumns(t, columns, pv, "foo", nil)
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
		}

		// match
		pv := &filterValueType{
			fieldName: "_msg",
			valueType: "iso8601",
		}
		testFilterMatchForColumns(t, columns, pv, "_msg", []int{0, 1, 2, 3, 4, 5, 6, 7, 8})

		// mimatch
		pv = &filterValueType{
			fieldName: "_msg",
			valueType: "string",
		}
		testFilterMatchForColumns(t, columns, pv, "_msg", nil)
	})

	// Remove the remaining data files for the test
	fs.MustRemoveAll(t.Name())
}
