package logstorage

import (
	"testing"
)

func TestMatchStringRange(t *testing.T) {
	t.Parallel()

	f := func(s, minValue, maxValue string, resultExpected bool) {
		t.Helper()
		result := matchStringRange(s, minValue, maxValue)
		if result != resultExpected {
			t.Fatalf("unexpected result; got %v; want %v", result, resultExpected)
		}
	}

	f("foo", "a", "b", false)
	f("foo", "a", "foa", false)
	f("foo", "a", "foz", true)
	f("foo", "foo", "foo", false)
	f("foo", "foo", "fooa", true)
	f("foo", "fooa", "foo", false)
}

func TestFilterStringRange(t *testing.T) {
	t.Parallel()

	t.Run("const-column", func(t *testing.T) {
		t.Parallel()

		columns := []column{
			{
				name: "foo",
				values: []string{
					"127.0.0.1",
					"127.0.0.1",
					"127.0.0.1",
				},
			},
		}

		// match
		fr := &filterStringRange{
			fieldName: "foo",
			minValue:  "127.0.0.1",
			maxValue:  "255.",
		}
		testFilterMatchForColumns(t, columns, fr, "foo", []int{0, 1, 2})

		fr = &filterStringRange{
			fieldName: "foo",
			minValue:  "127.0.0.1",
			maxValue:  "127.0.0.2",
		}
		testFilterMatchForColumns(t, columns, fr, "foo", []int{0, 1, 2})

		// mismatch
		fr = &filterStringRange{
			fieldName: "foo",
			minValue:  "127.0.0.1",
			maxValue:  "127.0.0.1",
		}
		testFilterMatchForColumns(t, columns, fr, "foo", nil)

		fr = &filterStringRange{
			fieldName: "foo",
			minValue:  "",
			maxValue:  "127.0.0.0",
		}
		testFilterMatchForColumns(t, columns, fr, "foo", nil)

		fr = &filterStringRange{
			fieldName: "non-existing-column",
			minValue:  "1",
			maxValue:  "2",
		}
		testFilterMatchForColumns(t, columns, fr, "foo", nil)

		fr = &filterStringRange{
			fieldName: "foo",
			minValue:  "127.0.0.2",
			maxValue:  "",
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
					"127.0.0.1",
					"Abc",
					"127.255.255.255",
					"10.4",
					"foo 127.0.0.1",
					"127.0.0.1 bar",
					"127.0.0.1",
				},
			},
		}

		// match
		fr := &filterStringRange{
			fieldName: "foo",
			minValue:  "127.0.0.0",
			maxValue:  "128.0.0.0",
		}
		testFilterMatchForColumns(t, columns, fr, "foo", []int{1, 3, 6, 7})

		fr = &filterStringRange{
			fieldName: "foo",
			minValue:  "127",
			maxValue:  "127.0.0.2",
		}
		testFilterMatchForColumns(t, columns, fr, "foo", []int{1, 6, 7})

		// mismatch
		fr = &filterStringRange{
			fieldName: "foo",
			minValue:  "0",
			maxValue:  "10",
		}
		testFilterMatchForColumns(t, columns, fr, "foo", nil)

		fr = &filterStringRange{
			fieldName: "foo",
			minValue:  "127.0.0.2",
			maxValue:  "127.127.0.0",
		}
		testFilterMatchForColumns(t, columns, fr, "foo", nil)

		fr = &filterStringRange{
			fieldName: "foo",
			minValue:  "128.0.0.0",
			maxValue:  "127.0.0.0",
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
					"127.0.0.1",
					"200",
					"155.5",
					"-5",
					"a fooBaR",
					"a 127.0.0.1 dfff",
					"a ТЕСТЙЦУК НГКШ ",
					"a !!,23.(!1)",
				},
			},
		}

		// match
		fr := &filterStringRange{
			fieldName: "foo",
			minValue:  "127.0.0.1",
			maxValue:  "255.255.255.255",
		}
		testFilterMatchForColumns(t, columns, fr, "foo", []int{2, 3, 4})

		// mismatch
		fr = &filterStringRange{
			fieldName: "foo",
			minValue:  "0",
			maxValue:  "10",
		}
		testFilterMatchForColumns(t, columns, fr, "foo", nil)

		fr = &filterStringRange{
			fieldName: "foo",
			minValue:  "255.255.255.255",
			maxValue:  "127.0.0.1",
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
		fr := &filterStringRange{
			fieldName: "foo",
			minValue:  "33",
			maxValue:  "500",
		}
		testFilterMatchForColumns(t, columns, fr, "foo", []int{0})

		// mismatch
		fr = &filterStringRange{
			fieldName: "foo",
			minValue:  "a",
			maxValue:  "b",
		}
		testFilterMatchForColumns(t, columns, fr, "foo", nil)

		fr = &filterStringRange{
			fieldName: "foo",
			minValue:  "100",
			maxValue:  "101",
		}
		testFilterMatchForColumns(t, columns, fr, "foo", nil)

		fr = &filterStringRange{
			fieldName: "foo",
			minValue:  "5",
			maxValue:  "33",
		}
		testFilterMatchForColumns(t, columns, fr, "foo", nil)
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
					"65535",
					"1",
					"2",
					"3",
					"4",
					"5",
				},
			},
		}

		// match
		fr := &filterStringRange{
			fieldName: "foo",
			minValue:  "33",
			maxValue:  "555",
		}
		testFilterMatchForColumns(t, columns, fr, "foo", []int{0})

		// mismatch
		fr = &filterStringRange{
			fieldName: "foo",
			minValue:  "a",
			maxValue:  "b",
		}
		testFilterMatchForColumns(t, columns, fr, "foo", nil)

		fr = &filterStringRange{
			fieldName: "foo",
			minValue:  "100",
			maxValue:  "101",
		}
		testFilterMatchForColumns(t, columns, fr, "foo", nil)

		fr = &filterStringRange{
			fieldName: "foo",
			minValue:  "5",
			maxValue:  "33",
		}
		testFilterMatchForColumns(t, columns, fr, "foo", nil)
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
					"65536",
					"1",
					"2",
					"3",
					"4",
					"5",
				},
			},
		}

		// match
		fr := &filterStringRange{
			fieldName: "foo",
			minValue:  "33",
			maxValue:  "555",
		}
		testFilterMatchForColumns(t, columns, fr, "foo", []int{0})

		// mismatch
		fr = &filterStringRange{
			fieldName: "foo",
			minValue:  "a",
			maxValue:  "b",
		}
		testFilterMatchForColumns(t, columns, fr, "foo", nil)

		fr = &filterStringRange{
			fieldName: "foo",
			minValue:  "100",
			maxValue:  "101",
		}
		testFilterMatchForColumns(t, columns, fr, "foo", nil)

		fr = &filterStringRange{
			fieldName: "foo",
			minValue:  "5",
			maxValue:  "33",
		}
		testFilterMatchForColumns(t, columns, fr, "foo", nil)
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
					"12345678901",
					"1",
					"2",
					"3",
					"4",
					"5",
				},
			},
		}

		// match
		fr := &filterStringRange{
			fieldName: "foo",
			minValue:  "33",
			maxValue:  "5555",
		}
		testFilterMatchForColumns(t, columns, fr, "foo", []int{0})

		// mismatch
		fr = &filterStringRange{
			fieldName: "foo",
			minValue:  "a",
			maxValue:  "b",
		}
		testFilterMatchForColumns(t, columns, fr, "foo", nil)

		fr = &filterStringRange{
			fieldName: "foo",
			minValue:  "100",
			maxValue:  "101",
		}
		testFilterMatchForColumns(t, columns, fr, "foo", nil)

		fr = &filterStringRange{
			fieldName: "foo",
			minValue:  "5",
			maxValue:  "33",
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
		fr := &filterStringRange{
			fieldName: "foo",
			minValue:  "33",
			maxValue:  "555",
		}
		testFilterMatchForColumns(t, columns, fr, "foo", []int{0})

		// mismatch
		fr = &filterStringRange{
			fieldName: "foo",
			minValue:  "a",
			maxValue:  "b",
		}
		testFilterMatchForColumns(t, columns, fr, "foo", nil)

		fr = &filterStringRange{
			fieldName: "foo",
			minValue:  "100",
			maxValue:  "101",
		}
		testFilterMatchForColumns(t, columns, fr, "foo", nil)

		fr = &filterStringRange{
			fieldName: "foo",
			minValue:  "5",
			maxValue:  "33",
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
		fr := &filterStringRange{
			fieldName: "foo",
			minValue:  "127.0.0",
			maxValue:  "128.0.0.0",
		}
		testFilterMatchForColumns(t, columns, fr, "foo", []int{2, 4, 5, 6, 7})

		// mismatch
		fr = &filterStringRange{
			fieldName: "foo",
			minValue:  "a",
			maxValue:  "b",
		}
		testFilterMatchForColumns(t, columns, fr, "foo", nil)

		fr = &filterStringRange{
			fieldName: "foo",
			minValue:  "128.0.0.0",
			maxValue:  "129.0.0.0",
		}
		testFilterMatchForColumns(t, columns, fr, "foo", nil)

		fr = &filterStringRange{
			fieldName: "foo",
			minValue:  "255.0.0.0",
			maxValue:  "255.255.255.255",
		}
		testFilterMatchForColumns(t, columns, fr, "foo", nil)

		fr = &filterStringRange{
			fieldName: "foo",
			minValue:  "128.0.0.0",
			maxValue:  "",
		}
		testFilterMatchForColumns(t, columns, fr, "foo", nil)
	})

	t.Run("timestamp-iso8601", func(t *testing.T) {
		t.Parallel()

		columns := []column{
			{
				name: "_msg",
				values: []string{
					"2005-01-02T15:04:05.001Z",
					"2006-02-02T15:04:05.002Z",
					"2006-01-02T15:04:05.003Z",
					"2006-01-02T15:04:05.004Z",
					"2026-01-02T15:04:05.005Z",
					"2026-01-02T15:04:05.006Z",
					"2026-01-02T15:04:05.007Z",
					"2026-01-02T15:04:05.008Z",
					"2026-01-02T15:04:05.009Z",
				},
			},
		}

		// match
		fr := &filterStringRange{
			fieldName: "_msg",
			minValue:  "2006-01-02",
			maxValue:  "2006-01-03",
		}
		testFilterMatchForColumns(t, columns, fr, "_msg", []int{2, 3})

		fr = &filterStringRange{
			fieldName: "_msg",
			minValue:  "",
			maxValue:  "2006",
		}
		testFilterMatchForColumns(t, columns, fr, "_msg", []int{0})

		// mismatch
		fr = &filterStringRange{
			fieldName: "_msg",
			minValue:  "3",
			maxValue:  "4",
		}
		testFilterMatchForColumns(t, columns, fr, "_msg", nil)

		fr = &filterStringRange{
			fieldName: "_msg",
			minValue:  "a",
			maxValue:  "b",
		}
		testFilterMatchForColumns(t, columns, fr, "_msg", nil)

		fr = &filterStringRange{
			fieldName: "_msg",
			minValue:  "2006-01-03",
			maxValue:  "2006-01-02",
		}
		testFilterMatchForColumns(t, columns, fr, "_msg", nil)
	})
}
