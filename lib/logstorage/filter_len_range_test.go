package logstorage

import (
	"testing"
)

func TestMatchLenRange(t *testing.T) {
	t.Parallel()

	f := func(s string, minLen, maxLen uint64, resultExpected bool) {
		t.Helper()
		result := matchLenRange(s, minLen, maxLen)
		if result != resultExpected {
			t.Fatalf("unexpected result; got %v; want %v", result, resultExpected)
		}
	}

	f("", 0, 0, true)
	f("", 0, 1, true)
	f("", 1, 1, false)

	f("abc", 0, 2, false)
	f("abc", 0, 3, true)
	f("abc", 0, 4, true)
	f("abc", 3, 4, true)
	f("abc", 4, 4, false)
	f("abc", 4, 2, false)

	f("ФЫВА", 3, 3, false)
	f("ФЫВА", 4, 4, true)
	f("ФЫВА", 5, 5, false)
	f("ФЫВА", 0, 10, true)
}

func TestFilterLenRange(t *testing.T) {
	t.Parallel()

	t.Run("const-column", func(t *testing.T) {
		t.Parallel()

		columns := []column{
			{
				name: "foo",
				values: []string{
					"10",
					"10",
					"10",
				},
			},
		}

		// match
		fr := &filterLenRange{
			fieldName: "foo",
			minLen:    2,
			maxLen:    20,
		}
		testFilterMatchForColumns(t, columns, fr, "foo", []int{0, 1, 2})

		fr = &filterLenRange{
			fieldName: "non-existing-column",
			minLen:    0,
			maxLen:    10,
		}
		testFilterMatchForColumns(t, columns, fr, "foo", []int{0, 1, 2})

		// mismatch
		fr = &filterLenRange{
			fieldName: "foo",
			minLen:    3,
			maxLen:    20,
		}
		testFilterMatchForColumns(t, columns, fr, "foo", nil)

		fr = &filterLenRange{
			fieldName: "non-existing-column",
			minLen:    10,
			maxLen:    20,
		}
		testFilterMatchForColumns(t, columns, fr, "foo", nil)
	})

	t.Run("dict", func(t *testing.T) {
		t.Parallel()

		columns := []column{
			{
				name: "foo",
				values: []string{
					"",
					"10",
					"Abc",
					"20",
					"10.5",
					"10 AFoobarbaz",
					"foobar",
				},
			},
		}

		// match
		fr := &filterLenRange{
			fieldName: "foo",
			minLen:    2,
			maxLen:    3,
		}
		testFilterMatchForColumns(t, columns, fr, "foo", []int{1, 2, 3})

		fr = &filterLenRange{
			fieldName: "foo",
			minLen:    0,
			maxLen:    1,
		}
		testFilterMatchForColumns(t, columns, fr, "foo", []int{0})

		// mismatch
		fr = &filterLenRange{
			fieldName: "foo",
			minLen:    20,
			maxLen:    30,
		}
		testFilterMatchForColumns(t, columns, fr, "foo", nil)
	})

	t.Run("strings", func(t *testing.T) {
		t.Parallel()

		columns := []column{
			{
				name: "foo",
				values: []string{
					"A FOO",
					"a 10",
					"10",
					"20",
					"15.5",
					"-5",
					"a fooBaR",
					"a kjlkjf dfff",
					"a ТЕСТЙЦУК НГКШ ",
					"a !!,23.(!1)",
				},
			},
		}

		// match
		fr := &filterLenRange{
			fieldName: "foo",
			minLen:    2,
			maxLen:    3,
		}
		testFilterMatchForColumns(t, columns, fr, "foo", []int{2, 3, 5})

		// mismatch
		fr = &filterLenRange{
			fieldName: "foo",
			minLen:    100,
			maxLen:    200,
		}
		testFilterMatchForColumns(t, columns, fr, "foo", nil)
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
		fr := &filterLenRange{
			fieldName: "foo",
			minLen:    2,
			maxLen:    2,
		}
		testFilterMatchForColumns(t, columns, fr, "foo", []int{1, 2, 5})

		// mismatch
		fr = &filterLenRange{
			fieldName: "foo",
			minLen:    0,
			maxLen:    0,
		}
		testFilterMatchForColumns(t, columns, fr, "foo", nil)

		fr = &filterLenRange{
			fieldName: "foo",
			minLen:    10,
			maxLen:    10,
		}
		testFilterMatchForColumns(t, columns, fr, "foo", nil)
	})

	t.Run("uint16", func(t *testing.T) {
		t.Parallel()

		columns := []column{
			{
				name: "foo",
				values: []string{
					"256",
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
		fr := &filterLenRange{
			fieldName: "foo",
			minLen:    2,
			maxLen:    2,
		}
		testFilterMatchForColumns(t, columns, fr, "foo", []int{1, 2, 5})

		// mismatch
		fr = &filterLenRange{
			fieldName: "foo",
			minLen:    0,
			maxLen:    0,
		}
		testFilterMatchForColumns(t, columns, fr, "foo", nil)

		fr = &filterLenRange{
			fieldName: "foo",
			minLen:    10,
			maxLen:    10,
		}
		testFilterMatchForColumns(t, columns, fr, "foo", nil)
	})

	t.Run("uint32", func(t *testing.T) {
		t.Parallel()

		columns := []column{
			{
				name: "foo",
				values: []string{
					"65536",
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
		fr := &filterLenRange{
			fieldName: "foo",
			minLen:    2,
			maxLen:    2,
		}
		testFilterMatchForColumns(t, columns, fr, "foo", []int{1, 2, 5})

		// mismatch
		fr = &filterLenRange{
			fieldName: "foo",
			minLen:    0,
			maxLen:    0,
		}
		testFilterMatchForColumns(t, columns, fr, "foo", nil)

		fr = &filterLenRange{
			fieldName: "foo",
			minLen:    10,
			maxLen:    10,
		}
		testFilterMatchForColumns(t, columns, fr, "foo", nil)
	})

	t.Run("uint64", func(t *testing.T) {
		t.Parallel()

		columns := []column{
			{
				name: "foo",
				values: []string{
					"123456789012",
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
		fr := &filterLenRange{
			fieldName: "foo",
			minLen:    2,
			maxLen:    2,
		}
		testFilterMatchForColumns(t, columns, fr, "foo", []int{1, 2, 5})

		// mismatch
		fr = &filterLenRange{
			fieldName: "foo",
			minLen:    0,
			maxLen:    0,
		}
		testFilterMatchForColumns(t, columns, fr, "foo", nil)

		fr = &filterLenRange{
			fieldName: "foo",
			minLen:    20,
			maxLen:    20,
		}
		testFilterMatchForColumns(t, columns, fr, "foo", nil)
	})

	t.Run("float64", func(t *testing.T) {
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
					"123456.78901",
					"-0.2",
					"2",
					"-334",
					"4",
					"5",
				},
			},
		}

		// match
		fr := &filterLenRange{
			fieldName: "foo",
			minLen:    2,
			maxLen:    2,
		}
		testFilterMatchForColumns(t, columns, fr, "foo", []int{1, 2})

		// mismatch
		fr = &filterLenRange{
			fieldName: "foo",
			minLen:    100,
			maxLen:    200,
		}
		testFilterMatchForColumns(t, columns, fr, "foo", nil)
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
					"55.55.12.55",
					"66.66.66.66",
					"7.7.7.7",
				},
			},
		}

		// match
		fr := &filterLenRange{
			fieldName: "foo",
			minLen:    3,
			maxLen:    7,
		}
		testFilterMatchForColumns(t, columns, fr, "foo", []int{0, 1, 11})

		// mismatch
		fr = &filterLenRange{
			fieldName: "foo",
			minLen:    20,
			maxLen:    30,
		}
		testFilterMatchForColumns(t, columns, fr, "foo", nil)
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
		fr := &filterLenRange{
			fieldName: "_msg",
			minLen:    10,
			maxLen:    30,
		}
		testFilterMatchForColumns(t, columns, fr, "_msg", []int{0, 1, 2, 3, 4, 5, 6, 7, 8})

		// mismatch
		fr = &filterLenRange{
			fieldName: "_msg",
			minLen:    10,
			maxLen:    11,
		}
		testFilterMatchForColumns(t, columns, fr, "_msg", nil)
	})
}
