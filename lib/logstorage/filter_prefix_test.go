package logstorage

import (
	"testing"
)

func TestMatchPrefix(t *testing.T) {
	t.Parallel()

	f := func(s, prefix string, resultExpected bool) {
		t.Helper()
		result := matchPrefix(s, prefix)
		if result != resultExpected {
			t.Fatalf("unexpected result; got %v; want %v", result, resultExpected)
		}
	}

	f("", "", false)
	f("foo", "", true)
	f("", "foo", false)
	f("foo", "foo", true)
	f("foo bar", "foo", true)
	f("foo bar", "bar", true)
	f("a foo bar", "foo", true)
	f("a foo bar", "fo", true)
	f("a foo bar", "oo", false)
	f("foobar", "foo", true)
	f("foobar", "bar", false)
	f("foobar", "oob", false)
	f("afoobar foo", "foo", true)
	f("раз два (три!)", "три", true)
	f("", "foo bar", false)
	f("foo bar", "foo bar", true)
	f("(foo bar)", "foo bar", true)
	f("afoo bar", "foo bar", false)
	f("afoo bar", "afoo ba", true)
	f("foo bar! baz", "foo bar!", true)
	f("a.foo bar! baz", ".foo bar! ", true)
	f("foo bar! baz", "foo bar! b", true)
	f("255.255.255.255", "5", false)
	f("255.255.255.255", "55", false)
	f("255.255.255.255", "255", true)
	f("255.255.255.255", "5.255", false)
	f("255.255.255.255", "255.25", true)
	f("255.255.255.255", "255.255", true)
}

func TestFilterPrefix(t *testing.T) {
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
			{
				name: "other column",
				values: []string{
					"asdfdsf",
				},
			},
		}

		// match
		fp := &filterPrefix{
			fieldName: "foo",
			prefix:    "abc",
		}
		testFilterMatchForColumns(t, columns, fp, "foo", []int{0})

		fp = &filterPrefix{
			fieldName: "foo",
			prefix:    "",
		}
		testFilterMatchForColumns(t, columns, fp, "foo", []int{0})

		fp = &filterPrefix{
			fieldName: "foo",
			prefix:    "ab",
		}
		testFilterMatchForColumns(t, columns, fp, "foo", []int{0})

		fp = &filterPrefix{
			fieldName: "foo",
			prefix:    "abc def",
		}
		testFilterMatchForColumns(t, columns, fp, "foo", []int{0})

		fp = &filterPrefix{
			fieldName: "foo",
			prefix:    "def",
		}
		testFilterMatchForColumns(t, columns, fp, "foo", []int{0})

		fp = &filterPrefix{
			fieldName: "other column",
			prefix:    "asdfdsf",
		}
		testFilterMatchForColumns(t, columns, fp, "foo", []int{0})

		fp = &filterPrefix{
			fieldName: "foo",
			prefix:    "",
		}
		testFilterMatchForColumns(t, columns, fp, "foo", []int{0})

		// mismatch
		fp = &filterPrefix{
			fieldName: "foo",
			prefix:    "bc",
		}
		testFilterMatchForColumns(t, columns, fp, "foo", nil)

		fp = &filterPrefix{
			fieldName: "other column",
			prefix:    "sd",
		}
		testFilterMatchForColumns(t, columns, fp, "foo", nil)

		fp = &filterPrefix{
			fieldName: "non-existing column",
			prefix:    "abc",
		}
		testFilterMatchForColumns(t, columns, fp, "foo", nil)

		fp = &filterPrefix{
			fieldName: "non-existing column",
			prefix:    "",
		}
		testFilterMatchForColumns(t, columns, fp, "foo", nil)
	})

	t.Run("const-column", func(t *testing.T) {
		t.Parallel()

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
		fp := &filterPrefix{
			fieldName: "foo",
			prefix:    "abc",
		}
		testFilterMatchForColumns(t, columns, fp, "foo", []int{0, 1, 2})

		fp = &filterPrefix{
			fieldName: "foo",
			prefix:    "",
		}
		testFilterMatchForColumns(t, columns, fp, "foo", []int{0, 1, 2})

		fp = &filterPrefix{
			fieldName: "foo",
			prefix:    "ab",
		}
		testFilterMatchForColumns(t, columns, fp, "foo", []int{0, 1, 2})

		fp = &filterPrefix{
			fieldName: "foo",
			prefix:    "abc de",
		}
		testFilterMatchForColumns(t, columns, fp, "foo", []int{0, 1, 2})

		fp = &filterPrefix{
			fieldName: "foo",
			prefix:    " de",
		}
		testFilterMatchForColumns(t, columns, fp, "foo", []int{0, 1, 2})

		fp = &filterPrefix{
			fieldName: "foo",
			prefix:    "abc def",
		}
		testFilterMatchForColumns(t, columns, fp, "foo", []int{0, 1, 2})

		fp = &filterPrefix{
			fieldName: "other-column",
			prefix:    "x",
		}
		testFilterMatchForColumns(t, columns, fp, "foo", []int{0, 1, 2})

		fp = &filterPrefix{
			fieldName: "_msg",
			prefix:    " 2 ",
		}
		testFilterMatchForColumns(t, columns, fp, "foo", []int{0, 1, 2})

		// mismatch
		fp = &filterPrefix{
			fieldName: "foo",
			prefix:    "abc def ",
		}
		testFilterMatchForColumns(t, columns, fp, "foo", nil)

		fp = &filterPrefix{
			fieldName: "foo",
			prefix:    "x",
		}
		testFilterMatchForColumns(t, columns, fp, "foo", nil)

		fp = &filterPrefix{
			fieldName: "other-column",
			prefix:    "foo",
		}
		testFilterMatchForColumns(t, columns, fp, "foo", nil)

		fp = &filterPrefix{
			fieldName: "non-existing column",
			prefix:    "x",
		}
		testFilterMatchForColumns(t, columns, fp, "foo", nil)

		fp = &filterPrefix{
			fieldName: "non-existing column",
			prefix:    "",
		}
		testFilterMatchForColumns(t, columns, fp, "foo", nil)

		fp = &filterPrefix{
			fieldName: "_msg",
			prefix:    "foo",
		}
		testFilterMatchForColumns(t, columns, fp, "foo", nil)
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
		fp := &filterPrefix{
			fieldName: "foo",
			prefix:    "foobar",
		}
		testFilterMatchForColumns(t, columns, fp, "foo", []int{1, 3, 4, 6})

		fp = &filterPrefix{
			fieldName: "foo",
			prefix:    "",
		}
		testFilterMatchForColumns(t, columns, fp, "foo", []int{1, 2, 3, 4, 5, 6})

		fp = &filterPrefix{
			fieldName: "foo",
			prefix:    "ba",
		}
		testFilterMatchForColumns(t, columns, fp, "foo", []int{3})

		// mismatch
		fp = &filterPrefix{
			fieldName: "foo",
			prefix:    "bar",
		}
		testFilterMatchForColumns(t, columns, fp, "foo", nil)

		fp = &filterPrefix{
			fieldName: "non-existing column",
			prefix:    "foobar",
		}
		testFilterMatchForColumns(t, columns, fp, "foo", nil)

		fp = &filterPrefix{
			fieldName: "non-existing column",
			prefix:    "",
		}
		testFilterMatchForColumns(t, columns, fp, "foo", nil)
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
					"a foobar",
					"a kjlkjf dfff",
					"a ТЕСТЙЦУК НГКШ ",
					"a !!,23.(!1)",
				},
			},
		}

		// match
		fp := &filterPrefix{
			fieldName: "foo",
			prefix:    "",
		}
		testFilterMatchForColumns(t, columns, fp, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9})

		fp = &filterPrefix{
			fieldName: "foo",
			prefix:    "a",
		}
		testFilterMatchForColumns(t, columns, fp, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9})

		fp = &filterPrefix{
			fieldName: "foo",
			prefix:    "НГК",
		}
		testFilterMatchForColumns(t, columns, fp, "foo", []int{8})

		fp = &filterPrefix{
			fieldName: "foo",
			prefix:    "aa a",
		}
		testFilterMatchForColumns(t, columns, fp, "foo", []int{2})

		fp = &filterPrefix{
			fieldName: "foo",
			prefix:    "!,",
		}
		testFilterMatchForColumns(t, columns, fp, "foo", []int{9})

		// mismatch
		fp = &filterPrefix{
			fieldName: "foo",
			prefix:    "aa ax",
		}
		testFilterMatchForColumns(t, columns, fp, "foo", nil)

		fp = &filterPrefix{
			fieldName: "foo",
			prefix:    "qwe rty abc",
		}
		testFilterMatchForColumns(t, columns, fp, "foo", nil)

		fp = &filterPrefix{
			fieldName: "foo",
			prefix:    "bar",
		}
		testFilterMatchForColumns(t, columns, fp, "foo", nil)

		fp = &filterPrefix{
			fieldName: "non-existing-column",
			prefix:    "",
		}
		testFilterMatchForColumns(t, columns, fp, "foo", nil)

		fp = &filterPrefix{
			fieldName: "foo",
			prefix:    "@",
		}
		testFilterMatchForColumns(t, columns, fp, "foo", nil)
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
		fp := &filterPrefix{
			fieldName: "foo",
			prefix:    "12",
		}
		testFilterMatchForColumns(t, columns, fp, "foo", []int{0, 1, 5})

		fp = &filterPrefix{
			fieldName: "foo",
			prefix:    "",
		}
		testFilterMatchForColumns(t, columns, fp, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10})

		fp = &filterPrefix{
			fieldName: "foo",
			prefix:    "0",
		}
		testFilterMatchForColumns(t, columns, fp, "foo", []int{3, 4})

		// mismatch
		fp = &filterPrefix{
			fieldName: "foo",
			prefix:    "bar",
		}
		testFilterMatchForColumns(t, columns, fp, "foo", nil)

		fp = &filterPrefix{
			fieldName: "foo",
			prefix:    "33",
		}
		testFilterMatchForColumns(t, columns, fp, "foo", nil)

		fp = &filterPrefix{
			fieldName: "foo",
			prefix:    "1234",
		}
		testFilterMatchForColumns(t, columns, fp, "foo", nil)

		fp = &filterPrefix{
			fieldName: "non-existing-column",
			prefix:    "",
		}
		testFilterMatchForColumns(t, columns, fp, "foo", nil)
	})

	t.Run("uint16", func(t *testing.T) {
		t.Parallel()

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
		fp := &filterPrefix{
			fieldName: "foo",
			prefix:    "123",
		}
		testFilterMatchForColumns(t, columns, fp, "foo", []int{0, 4})

		fp = &filterPrefix{
			fieldName: "foo",
			prefix:    "",
		}
		testFilterMatchForColumns(t, columns, fp, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9})

		fp = &filterPrefix{
			fieldName: "foo",
			prefix:    "0",
		}
		testFilterMatchForColumns(t, columns, fp, "foo", []int{1})

		// mismatch
		fp = &filterPrefix{
			fieldName: "foo",
			prefix:    "bar",
		}
		testFilterMatchForColumns(t, columns, fp, "foo", nil)

		fp = &filterPrefix{
			fieldName: "foo",
			prefix:    "33",
		}
		testFilterMatchForColumns(t, columns, fp, "foo", nil)

		fp = &filterPrefix{
			fieldName: "foo",
			prefix:    "123456",
		}
		testFilterMatchForColumns(t, columns, fp, "foo", nil)

		fp = &filterPrefix{
			fieldName: "non-existing-column",
			prefix:    "",
		}
		testFilterMatchForColumns(t, columns, fp, "foo", nil)
	})

	t.Run("uint32", func(t *testing.T) {
		t.Parallel()

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
		fp := &filterPrefix{
			fieldName: "foo",
			prefix:    "123",
		}
		testFilterMatchForColumns(t, columns, fp, "foo", []int{0, 4})

		fp = &filterPrefix{
			fieldName: "foo",
			prefix:    "",
		}
		testFilterMatchForColumns(t, columns, fp, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9})

		fp = &filterPrefix{
			fieldName: "foo",
			prefix:    "65536",
		}
		testFilterMatchForColumns(t, columns, fp, "foo", []int{3})

		// mismatch
		fp = &filterPrefix{
			fieldName: "foo",
			prefix:    "bar",
		}
		testFilterMatchForColumns(t, columns, fp, "foo", nil)

		fp = &filterPrefix{
			fieldName: "foo",
			prefix:    "33",
		}
		testFilterMatchForColumns(t, columns, fp, "foo", nil)

		fp = &filterPrefix{
			fieldName: "foo",
			prefix:    "12345678901",
		}
		testFilterMatchForColumns(t, columns, fp, "foo", nil)

		fp = &filterPrefix{
			fieldName: "non-existing-column",
			prefix:    "",
		}
		testFilterMatchForColumns(t, columns, fp, "foo", nil)
	})

	t.Run("uint64", func(t *testing.T) {
		t.Parallel()

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
		fp := &filterPrefix{
			fieldName: "foo",
			prefix:    "1234",
		}
		testFilterMatchForColumns(t, columns, fp, "foo", []int{0, 4})

		fp = &filterPrefix{
			fieldName: "foo",
			prefix:    "",
		}
		testFilterMatchForColumns(t, columns, fp, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8})

		fp = &filterPrefix{
			fieldName: "foo",
			prefix:    "12345678901",
		}
		testFilterMatchForColumns(t, columns, fp, "foo", []int{4})

		// mismatch
		fp = &filterPrefix{
			fieldName: "foo",
			prefix:    "bar",
		}
		testFilterMatchForColumns(t, columns, fp, "foo", nil)

		fp = &filterPrefix{
			fieldName: "foo",
			prefix:    "33",
		}
		testFilterMatchForColumns(t, columns, fp, "foo", nil)

		fp = &filterPrefix{
			fieldName: "foo",
			prefix:    "12345678901234567890",
		}
		testFilterMatchForColumns(t, columns, fp, "foo", nil)

		fp = &filterPrefix{
			fieldName: "non-existing-column",
			prefix:    "",
		}
		testFilterMatchForColumns(t, columns, fp, "foo", nil)
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
		fp := &filterPrefix{
			fieldName: "foo",
			prefix:    "123",
		}
		testFilterMatchForColumns(t, columns, fp, "foo", []int{0, 4})

		fp = &filterPrefix{
			fieldName: "foo",
			prefix:    "",
		}
		testFilterMatchForColumns(t, columns, fp, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8})

		fp = &filterPrefix{
			fieldName: "foo",
			prefix:    "1234.5678901",
		}
		testFilterMatchForColumns(t, columns, fp, "foo", []int{4})

		fp = &filterPrefix{
			fieldName: "foo",
			prefix:    "56789",
		}
		testFilterMatchForColumns(t, columns, fp, "foo", []int{4})

		fp = &filterPrefix{
			fieldName: "foo",
			prefix:    "-6553",
		}
		testFilterMatchForColumns(t, columns, fp, "foo", []int{3})

		fp = &filterPrefix{
			fieldName: "foo",
			prefix:    "65536",
		}
		testFilterMatchForColumns(t, columns, fp, "foo", []int{3})

		// mismatch
		fp = &filterPrefix{
			fieldName: "foo",
			prefix:    "bar",
		}
		testFilterMatchForColumns(t, columns, fp, "foo", nil)

		fp = &filterPrefix{
			fieldName: "foo",
			prefix:    "7344.8943",
		}
		testFilterMatchForColumns(t, columns, fp, "foo", nil)

		fp = &filterPrefix{
			fieldName: "foo",
			prefix:    "-1234",
		}
		testFilterMatchForColumns(t, columns, fp, "foo", nil)

		fp = &filterPrefix{
			fieldName: "foo",
			prefix:    "+1234",
		}
		testFilterMatchForColumns(t, columns, fp, "foo", nil)

		fp = &filterPrefix{
			fieldName: "foo",
			prefix:    "23",
		}
		testFilterMatchForColumns(t, columns, fp, "foo", nil)

		fp = &filterPrefix{
			fieldName: "foo",
			prefix:    "678",
		}
		testFilterMatchForColumns(t, columns, fp, "foo", nil)

		fp = &filterPrefix{
			fieldName: "foo",
			prefix:    "33",
		}
		testFilterMatchForColumns(t, columns, fp, "foo", nil)

		fp = &filterPrefix{
			fieldName: "foo",
			prefix:    "12345678901234567890",
		}
		testFilterMatchForColumns(t, columns, fp, "foo", nil)

		fp = &filterPrefix{
			fieldName: "non-existing-column",
			prefix:    "",
		}
		testFilterMatchForColumns(t, columns, fp, "foo", nil)
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
		fp := &filterPrefix{
			fieldName: "foo",
			prefix:    "127.0.0.1",
		}
		testFilterMatchForColumns(t, columns, fp, "foo", []int{2, 4, 5, 7})

		fp = &filterPrefix{
			fieldName: "foo",
			prefix:    "",
		}
		testFilterMatchForColumns(t, columns, fp, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11})

		fp = &filterPrefix{
			fieldName: "foo",
			prefix:    "12",
		}
		testFilterMatchForColumns(t, columns, fp, "foo", []int{2, 4, 5, 6, 7, 8, 9})

		fp = &filterPrefix{
			fieldName: "foo",
			prefix:    "127.0.0",
		}
		testFilterMatchForColumns(t, columns, fp, "foo", []int{2, 4, 5, 7})

		fp = &filterPrefix{
			fieldName: "foo",
			prefix:    "2.3.",
		}
		testFilterMatchForColumns(t, columns, fp, "foo", []int{0})

		fp = &filterPrefix{
			fieldName: "foo",
			prefix:    "0",
		}
		testFilterMatchForColumns(t, columns, fp, "foo", []int{1, 2, 4, 5, 6, 7, 8})

		// mismatch
		fp = &filterPrefix{
			fieldName: "foo",
			prefix:    "bar",
		}
		testFilterMatchForColumns(t, columns, fp, "foo", nil)

		fp = &filterPrefix{
			fieldName: "foo",
			prefix:    "8",
		}
		testFilterMatchForColumns(t, columns, fp, "foo", nil)

		fp = &filterPrefix{
			fieldName: "foo",
			prefix:    "127.1",
		}
		testFilterMatchForColumns(t, columns, fp, "foo", nil)

		fp = &filterPrefix{
			fieldName: "foo",
			prefix:    "27.0",
		}
		testFilterMatchForColumns(t, columns, fp, "foo", nil)

		fp = &filterPrefix{
			fieldName: "foo",
			prefix:    "255.255.255.255",
		}
		testFilterMatchForColumns(t, columns, fp, "foo", nil)

		fp = &filterPrefix{
			fieldName: "non-existing-column",
			prefix:    "",
		}
		testFilterMatchForColumns(t, columns, fp, "foo", nil)
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
		fp := &filterPrefix{
			fieldName: "_msg",
			prefix:    "2006-01-02T15:04:05.005Z",
		}
		testFilterMatchForColumns(t, columns, fp, "_msg", []int{4})

		fp = &filterPrefix{
			fieldName: "_msg",
			prefix:    "",
		}
		testFilterMatchForColumns(t, columns, fp, "_msg", []int{0, 1, 2, 3, 4, 5, 6, 7, 8})

		fp = &filterPrefix{
			fieldName: "_msg",
			prefix:    "2006-01-0",
		}
		testFilterMatchForColumns(t, columns, fp, "_msg", []int{0, 1, 2, 3, 4, 5, 6, 7, 8})

		fp = &filterPrefix{
			fieldName: "_msg",
			prefix:    "002",
		}
		testFilterMatchForColumns(t, columns, fp, "_msg", []int{1})

		// mimatch
		fp = &filterPrefix{
			fieldName: "_msg",
			prefix:    "bar",
		}
		testFilterMatchForColumns(t, columns, fp, "_msg", nil)

		fp = &filterPrefix{
			fieldName: "_msg",
			prefix:    "2006-03-02T15:04:05.005Z",
		}
		testFilterMatchForColumns(t, columns, fp, "_msg", nil)

		fp = &filterPrefix{
			fieldName: "_msg",
			prefix:    "06",
		}
		testFilterMatchForColumns(t, columns, fp, "_msg", nil)

		// This filter shouldn't match row=4, since it has different string representation of the timestamp
		fp = &filterPrefix{
			fieldName: "_msg",
			prefix:    "2006-01-02T16:04:05.005+01:00",
		}
		testFilterMatchForColumns(t, columns, fp, "_msg", nil)

		// This filter shouldn't match row=4, since it contains too many digits for millisecond part
		fp = &filterPrefix{
			fieldName: "_msg",
			prefix:    "2006-01-02T15:04:05.00500Z",
		}
		testFilterMatchForColumns(t, columns, fp, "_msg", nil)

		fp = &filterPrefix{
			fieldName: "non-existing-column",
			prefix:    "",
		}
		testFilterMatchForColumns(t, columns, fp, "_msg", nil)
	})
}
