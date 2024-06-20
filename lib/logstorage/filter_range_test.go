package logstorage

import (
	"testing"
)

func TestFilterRange(t *testing.T) {
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
		fr := &filterRange{
			fieldName: "foo",
			minValue:  -10,
			maxValue:  20,
		}
		testFilterMatchForColumns(t, columns, fr, "foo", []int{0, 1, 2})

		fr = &filterRange{
			fieldName: "foo",
			minValue:  10,
			maxValue:  10,
		}
		testFilterMatchForColumns(t, columns, fr, "foo", []int{0, 1, 2})

		fr = &filterRange{
			fieldName: "foo",
			minValue:  10,
			maxValue:  20,
		}
		testFilterMatchForColumns(t, columns, fr, "foo", []int{0, 1, 2})

		// mismatch
		fr = &filterRange{
			fieldName: "foo",
			minValue:  -10,
			maxValue:  9.99,
		}
		testFilterMatchForColumns(t, columns, fr, "foo", nil)

		fr = &filterRange{
			fieldName: "foo",
			minValue:  20,
			maxValue:  -10,
		}
		testFilterMatchForColumns(t, columns, fr, "foo", nil)

		fr = &filterRange{
			fieldName: "foo",
			minValue:  10.1,
			maxValue:  20,
		}
		testFilterMatchForColumns(t, columns, fr, "foo", nil)

		fr = &filterRange{
			fieldName: "non-existing-column",
			minValue:  10,
			maxValue:  20,
		}
		testFilterMatchForColumns(t, columns, fr, "foo", nil)

		fr = &filterRange{
			fieldName: "foo",
			minValue:  11,
			maxValue:  10,
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
		fr := &filterRange{
			fieldName: "foo",
			minValue:  -10,
			maxValue:  20,
		}
		testFilterMatchForColumns(t, columns, fr, "foo", []int{1, 3, 4})

		fr = &filterRange{
			fieldName: "foo",
			minValue:  10,
			maxValue:  20,
		}
		testFilterMatchForColumns(t, columns, fr, "foo", []int{1, 3, 4})

		fr = &filterRange{
			fieldName: "foo",
			minValue:  10.1,
			maxValue:  19.9,
		}
		testFilterMatchForColumns(t, columns, fr, "foo", []int{4})

		// mismatch
		fr = &filterRange{
			fieldName: "foo",
			minValue:  -11,
			maxValue:  0,
		}
		testFilterMatchForColumns(t, columns, fr, "foo", nil)

		fr = &filterRange{
			fieldName: "foo",
			minValue:  11,
			maxValue:  19,
		}
		testFilterMatchForColumns(t, columns, fr, "foo", nil)

		fr = &filterRange{
			fieldName: "foo",
			minValue:  20.1,
			maxValue:  100,
		}
		testFilterMatchForColumns(t, columns, fr, "foo", nil)

		fr = &filterRange{
			fieldName: "foo",
			minValue:  20,
			maxValue:  10,
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
		fr := &filterRange{
			fieldName: "foo",
			minValue:  -100,
			maxValue:  100,
		}
		testFilterMatchForColumns(t, columns, fr, "foo", []int{2, 3, 4, 5})

		fr = &filterRange{
			fieldName: "foo",
			minValue:  10,
			maxValue:  20,
		}
		testFilterMatchForColumns(t, columns, fr, "foo", []int{2, 3, 4})

		fr = &filterRange{
			fieldName: "foo",
			minValue:  -5,
			maxValue:  -5,
		}
		testFilterMatchForColumns(t, columns, fr, "foo", []int{5})

		// mismatch
		fr = &filterRange{
			fieldName: "foo",
			minValue:  -10,
			maxValue:  -5.1,
		}
		testFilterMatchForColumns(t, columns, fr, "foo", nil)

		fr = &filterRange{
			fieldName: "foo",
			minValue:  20.1,
			maxValue:  100,
		}
		testFilterMatchForColumns(t, columns, fr, "foo", nil)

		fr = &filterRange{
			fieldName: "foo",
			minValue:  20,
			maxValue:  10,
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
		fr := &filterRange{
			fieldName: "foo",
			minValue:  0,
			maxValue:  3,
		}
		testFilterMatchForColumns(t, columns, fr, "foo", []int{3, 4, 6, 7, 8})

		fr = &filterRange{
			fieldName: "foo",
			minValue:  0.1,
			maxValue:  2.9,
		}
		testFilterMatchForColumns(t, columns, fr, "foo", []int{6, 7})

		fr = &filterRange{
			fieldName: "foo",
			minValue:  -1e18,
			maxValue:  2.9,
		}
		testFilterMatchForColumns(t, columns, fr, "foo", []int{3, 4, 6, 7})

		// mismatch
		fr = &filterRange{
			fieldName: "foo",
			minValue:  -1e18,
			maxValue:  -0.1,
		}
		testFilterMatchForColumns(t, columns, fr, "foo", nil)

		fr = &filterRange{
			fieldName: "foo",
			minValue:  0.1,
			maxValue:  0.9,
		}
		testFilterMatchForColumns(t, columns, fr, "foo", nil)

		fr = &filterRange{
			fieldName: "foo",
			minValue:  2.9,
			maxValue:  0.1,
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
		fr := &filterRange{
			fieldName: "foo",
			minValue:  0,
			maxValue:  3,
		}
		testFilterMatchForColumns(t, columns, fr, "foo", []int{3, 4, 6, 7, 8})

		fr = &filterRange{
			fieldName: "foo",
			minValue:  0.1,
			maxValue:  2.9,
		}
		testFilterMatchForColumns(t, columns, fr, "foo", []int{6, 7})

		fr = &filterRange{
			fieldName: "foo",
			minValue:  -1e18,
			maxValue:  2.9,
		}
		testFilterMatchForColumns(t, columns, fr, "foo", []int{3, 4, 6, 7})

		// mismatch
		fr = &filterRange{
			fieldName: "foo",
			minValue:  -1e18,
			maxValue:  -0.1,
		}
		testFilterMatchForColumns(t, columns, fr, "foo", nil)

		fr = &filterRange{
			fieldName: "foo",
			minValue:  0.1,
			maxValue:  0.9,
		}
		testFilterMatchForColumns(t, columns, fr, "foo", nil)

		fr = &filterRange{
			fieldName: "foo",
			minValue:  2.9,
			maxValue:  0.1,
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
		fr := &filterRange{
			fieldName: "foo",
			minValue:  0,
			maxValue:  3,
		}
		testFilterMatchForColumns(t, columns, fr, "foo", []int{3, 4, 6, 7, 8})

		fr = &filterRange{
			fieldName: "foo",
			minValue:  0.1,
			maxValue:  2.9,
		}
		testFilterMatchForColumns(t, columns, fr, "foo", []int{6, 7})

		fr = &filterRange{
			fieldName: "foo",
			minValue:  -1e18,
			maxValue:  2.9,
		}
		testFilterMatchForColumns(t, columns, fr, "foo", []int{3, 4, 6, 7})

		// mismatch
		fr = &filterRange{
			fieldName: "foo",
			minValue:  -1e18,
			maxValue:  -0.1,
		}
		testFilterMatchForColumns(t, columns, fr, "foo", nil)

		fr = &filterRange{
			fieldName: "foo",
			minValue:  0.1,
			maxValue:  0.9,
		}
		testFilterMatchForColumns(t, columns, fr, "foo", nil)

		fr = &filterRange{
			fieldName: "foo",
			minValue:  2.9,
			maxValue:  0.1,
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
		fr := &filterRange{
			fieldName: "foo",
			minValue:  -inf,
			maxValue:  3,
		}
		testFilterMatchForColumns(t, columns, fr, "foo", []int{3, 4, 6, 7, 8})

		fr = &filterRange{
			fieldName: "foo",
			minValue:  0.1,
			maxValue:  2.9,
		}
		testFilterMatchForColumns(t, columns, fr, "foo", []int{6, 7})

		fr = &filterRange{
			fieldName: "foo",
			minValue:  -1e18,
			maxValue:  2.9,
		}
		testFilterMatchForColumns(t, columns, fr, "foo", []int{3, 4, 6, 7})

		fr = &filterRange{
			fieldName: "foo",
			minValue:  1000,
			maxValue:  inf,
		}
		testFilterMatchForColumns(t, columns, fr, "foo", []int{5})

		// mismatch
		fr = &filterRange{
			fieldName: "foo",
			minValue:  -1e18,
			maxValue:  -0.1,
		}
		testFilterMatchForColumns(t, columns, fr, "foo", nil)

		fr = &filterRange{
			fieldName: "foo",
			minValue:  0.1,
			maxValue:  0.9,
		}
		testFilterMatchForColumns(t, columns, fr, "foo", nil)

		fr = &filterRange{
			fieldName: "foo",
			minValue:  2.9,
			maxValue:  0.1,
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
		fr := &filterRange{
			fieldName: "foo",
			minValue:  -inf,
			maxValue:  3,
		}
		testFilterMatchForColumns(t, columns, fr, "foo", []int{3, 4, 6, 7, 8})

		fr = &filterRange{
			fieldName: "foo",
			minValue:  0.1,
			maxValue:  2.9,
		}
		testFilterMatchForColumns(t, columns, fr, "foo", []int{7})

		fr = &filterRange{
			fieldName: "foo",
			minValue:  -1e18,
			maxValue:  1.9,
		}
		testFilterMatchForColumns(t, columns, fr, "foo", []int{3, 4, 6, 8})

		fr = &filterRange{
			fieldName: "foo",
			minValue:  1000,
			maxValue:  inf,
		}
		testFilterMatchForColumns(t, columns, fr, "foo", []int{5})

		// mismatch
		fr = &filterRange{
			fieldName: "foo",
			minValue:  -1e18,
			maxValue:  -334.1,
		}
		testFilterMatchForColumns(t, columns, fr, "foo", nil)

		fr = &filterRange{
			fieldName: "foo",
			minValue:  0.1,
			maxValue:  0.9,
		}
		testFilterMatchForColumns(t, columns, fr, "foo", nil)

		fr = &filterRange{
			fieldName: "foo",
			minValue:  2.9,
			maxValue:  0.1,
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

		fr := &filterRange{
			fieldName: "foo",
			minValue:  -100,
			maxValue:  100,
		}
		testFilterMatchForColumns(t, columns, fr, "foo", []int{1})
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

		// range filter always mismatches timestmap
		fr := &filterRange{
			fieldName: "_msg",
			minValue:  -100,
			maxValue:  100,
		}
		testFilterMatchForColumns(t, columns, fr, "_msg", nil)
	})
}
