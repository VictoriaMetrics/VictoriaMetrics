package logstorage

import (
	"testing"
)

func TestMatchIPv4Range(t *testing.T) {
	t.Parallel()

	f := func(s string, minValue, maxValue uint32, resultExpected bool) {
		t.Helper()
		result := matchIPv4Range(s, minValue, maxValue)
		if result != resultExpected {
			t.Fatalf("unexpected result; got %v; want %v", result, resultExpected)
		}
	}

	// Invalid IP
	f("", 0, 1000, false)
	f("123", 0, 1000, false)

	// range mismatch
	f("0.0.0.1", 2, 100, false)
	f("127.0.0.1", 0x6f000000, 0x7f000000, false)

	// range match
	f("0.0.0.1", 1, 1, true)
	f("0.0.0.1", 0, 100, true)
	f("127.0.0.1", 0x7f000000, 0x7f000001, true)
}

func TestFilterIPv4Range(t *testing.T) {
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
		fr := &filterIPv4Range{
			fieldName: "foo",
			minValue:  0,
			maxValue:  0x80000000,
		}
		testFilterMatchForColumns(t, columns, fr, "foo", []int{0, 1, 2})

		fr = &filterIPv4Range{
			fieldName: "foo",
			minValue:  0x7f000001,
			maxValue:  0x7f000001,
		}
		testFilterMatchForColumns(t, columns, fr, "foo", []int{0, 1, 2})

		// mismatch
		fr = &filterIPv4Range{
			fieldName: "foo",
			minValue:  0,
			maxValue:  0x7f000000,
		}
		testFilterMatchForColumns(t, columns, fr, "foo", nil)

		fr = &filterIPv4Range{
			fieldName: "non-existing-column",
			minValue:  0,
			maxValue:  20000,
		}
		testFilterMatchForColumns(t, columns, fr, "foo", nil)

		fr = &filterIPv4Range{
			fieldName: "foo",
			minValue:  0x80000000,
			maxValue:  0,
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
		fr := &filterIPv4Range{
			fieldName: "foo",
			minValue:  0x7f000000,
			maxValue:  0x80000000,
		}
		testFilterMatchForColumns(t, columns, fr, "foo", []int{1, 3, 7})

		fr = &filterIPv4Range{
			fieldName: "foo",
			minValue:  0,
			maxValue:  0x7f000001,
		}
		testFilterMatchForColumns(t, columns, fr, "foo", []int{1, 7})

		// mismatch
		fr = &filterIPv4Range{
			fieldName: "foo",
			minValue:  0,
			maxValue:  1000,
		}
		testFilterMatchForColumns(t, columns, fr, "foo", nil)

		fr = &filterIPv4Range{
			fieldName: "foo",
			minValue:  0x7f000002,
			maxValue:  0x7f7f0000,
		}
		testFilterMatchForColumns(t, columns, fr, "foo", nil)

		fr = &filterIPv4Range{
			fieldName: "foo",
			minValue:  0x80000000,
			maxValue:  0x7f000000,
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
					"20",
					"15.5",
					"-5",
					"a fooBaR",
					"a 127.0.0.1 dfff",
					"a ТЕСТЙЦУК НГКШ ",
					"a !!,23.(!1)",
				},
			},
		}

		// match
		fr := &filterIPv4Range{
			fieldName: "foo",
			minValue:  0x7f000000,
			maxValue:  0xffffffff,
		}
		testFilterMatchForColumns(t, columns, fr, "foo", []int{2})

		// mismatch
		fr = &filterIPv4Range{
			fieldName: "foo",
			minValue:  0,
			maxValue:  10000,
		}
		testFilterMatchForColumns(t, columns, fr, "foo", nil)

		fr = &filterIPv4Range{
			fieldName: "foo",
			minValue:  0xffffffff,
			maxValue:  0x7f000000,
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

		// mismatch
		fr := &filterIPv4Range{
			fieldName: "foo",
			minValue:  0,
			maxValue:  0xffffffff,
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

		// mismatch
		fr := &filterIPv4Range{
			fieldName: "foo",
			minValue:  0,
			maxValue:  0xffffffff,
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

		// mismatch
		fr := &filterIPv4Range{
			fieldName: "foo",
			minValue:  0,
			maxValue:  0xffffffff,
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

		// mismatch
		fr := &filterIPv4Range{
			fieldName: "foo",
			minValue:  0,
			maxValue:  0xffffffff,
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

		// mismatch
		fr := &filterIPv4Range{
			fieldName: "foo",
			minValue:  0,
			maxValue:  0xffffffff,
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
		fr := &filterIPv4Range{
			fieldName: "foo",
			minValue:  0,
			maxValue:  0x08000000,
		}
		testFilterMatchForColumns(t, columns, fr, "foo", []int{0, 1, 11})

		// mismatch
		fr = &filterIPv4Range{
			fieldName: "foo",
			minValue:  0x80000000,
			maxValue:  0x90000000,
		}
		testFilterMatchForColumns(t, columns, fr, "foo", nil)

		fr = &filterIPv4Range{
			fieldName: "foo",
			minValue:  0xff000000,
			maxValue:  0xffffffff,
		}
		testFilterMatchForColumns(t, columns, fr, "foo", nil)

		fr = &filterIPv4Range{
			fieldName: "foo",
			minValue:  0x08000000,
			maxValue:  0,
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

		// mismatch
		fr := &filterIPv4Range{
			fieldName: "_msg",
			minValue:  0,
			maxValue:  0xffffffff,
		}
		testFilterMatchForColumns(t, columns, fr, "_msg", nil)
	})
}
