package logstorage

import (
	"testing"
)

func TestMatchPhrase(t *testing.T) {
	t.Parallel()

	f := func(s, phrase string, resultExpected bool) {
		t.Helper()
		result := matchPhrase(s, phrase)
		if result != resultExpected {
			t.Fatalf("unexpected result; got %v; want %v", result, resultExpected)
		}
	}

	f("", "", true)
	f("foo", "", false)
	f("", "foo", false)
	f("foo", "foo", true)
	f("foo bar", "foo", true)
	f("foo bar", "bar", true)
	f("a foo bar", "foo", true)
	f("a foo bar", "fo", false)
	f("a foo bar", "oo", false)
	f("foobar", "foo", false)
	f("foobar", "bar", false)
	f("foobar", "oob", false)
	f("afoobar foo", "foo", true)
	f("раз два (три!)", "три", true)
	f("", "foo bar", false)
	f("foo bar", "foo bar", true)
	f("(foo bar)", "foo bar", true)
	f("afoo bar", "foo bar", false)
	f("afoo bar", "afoo ba", false)
	f("foo bar! baz", "foo bar!", true)
	f("a.foo bar! baz", ".foo bar! ", true)
	f("foo bar! baz", "foo bar! b", false)
	f("255.255.255.255", "5", false)
	f("255.255.255.255", "55", false)
	f("255.255.255.255", "255", true)
	f("255.255.255.255", "5.255", false)
	f("255.255.255.255", "255.25", false)
	f("255.255.255.255", "255.255", true)
}

func TestFilterPhrase(t *testing.T) {
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
		pf := &filterPhrase{
			fieldName: "foo",
			phrase:    "abc",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0})

		pf = &filterPhrase{
			fieldName: "foo",
			phrase:    "abc def",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0})

		pf = &filterPhrase{
			fieldName: "foo",
			phrase:    "def",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0})

		pf = &filterPhrase{
			fieldName: "other column",
			phrase:    "asdfdsf",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0})

		pf = &filterPhrase{
			fieldName: "non-existing-column",
			phrase:    "",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0})

		// mismatch
		pf = &filterPhrase{
			fieldName: "foo",
			phrase:    "ab",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &filterPhrase{
			fieldName: "foo",
			phrase:    "",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &filterPhrase{
			fieldName: "other column",
			phrase:    "sd",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &filterPhrase{
			fieldName: "non-existing column",
			phrase:    "abc",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)
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
		pf := &filterPhrase{
			fieldName: "foo",
			phrase:    "abc",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0, 1, 2})

		pf = &filterPhrase{
			fieldName: "foo",
			phrase:    "def",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0, 1, 2})

		pf = &filterPhrase{
			fieldName: "foo",
			phrase:    " def",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0, 1, 2})

		pf = &filterPhrase{
			fieldName: "foo",
			phrase:    "abc def",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0, 1, 2})

		pf = &filterPhrase{
			fieldName: "other-column",
			phrase:    "x",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0, 1, 2})

		pf = &filterPhrase{
			fieldName: "_msg",
			phrase:    " 2 ",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0, 1, 2})

		pf = &filterPhrase{
			fieldName: "non-existing-column",
			phrase:    "",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0, 1, 2})

		// mismatch
		pf = &filterPhrase{
			fieldName: "foo",
			phrase:    "abc def ",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &filterPhrase{
			fieldName: "foo",
			phrase:    "",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &filterPhrase{
			fieldName: "foo",
			phrase:    "x",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &filterPhrase{
			fieldName: "other-column",
			phrase:    "foo",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &filterPhrase{
			fieldName: "non-existing column",
			phrase:    "x",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &filterPhrase{
			fieldName: "_msg",
			phrase:    "foo",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)
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
		pf := &filterPhrase{
			fieldName: "foo",
			phrase:    "foobar",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{1, 3, 6})

		pf = &filterPhrase{
			fieldName: "foo",
			phrase:    "",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0})

		pf = &filterPhrase{
			fieldName: "foo",
			phrase:    "baz",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{3})

		pf = &filterPhrase{
			fieldName: "non-existing-column",
			phrase:    "",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0, 1, 2, 3, 4, 5, 6})

		// mismatch
		pf = &filterPhrase{
			fieldName: "foo",
			phrase:    "bar",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &filterPhrase{
			fieldName: "non-existing column",
			phrase:    "foobar",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)
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
		pf := &filterPhrase{
			fieldName: "foo",
			phrase:    "a",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9})

		pf = &filterPhrase{
			fieldName: "foo",
			phrase:    "НГКШ",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{8})

		pf = &filterPhrase{
			fieldName: "non-existing-column",
			phrase:    "",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9})

		pf = &filterPhrase{
			fieldName: "foo",
			phrase:    "!,",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{9})

		// mismatch
		pf = &filterPhrase{
			fieldName: "foo",
			phrase:    "aa a",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &filterPhrase{
			fieldName: "foo",
			phrase:    "",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &filterPhrase{
			fieldName: "foo",
			phrase:    "bar",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &filterPhrase{
			fieldName: "foo",
			phrase:    "@",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)
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
		pf := &filterPhrase{
			fieldName: "foo",
			phrase:    "12",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{1, 5})

		pf = &filterPhrase{
			fieldName: "foo",
			phrase:    "0",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{3, 4})

		pf = &filterPhrase{
			fieldName: "non-existing-column",
			phrase:    "",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10})

		// mismatch
		pf = &filterPhrase{
			fieldName: "foo",
			phrase:    "bar",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &filterPhrase{
			fieldName: "foo",
			phrase:    "",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &filterPhrase{
			fieldName: "foo",
			phrase:    "33",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &filterPhrase{
			fieldName: "foo",
			phrase:    "1234",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)
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
		pf := &filterPhrase{
			fieldName: "foo",
			phrase:    "1234",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0, 4})

		pf = &filterPhrase{
			fieldName: "foo",
			phrase:    "0",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{1})

		pf = &filterPhrase{
			fieldName: "non-existing-column",
			phrase:    "",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9})

		// mismatch
		pf = &filterPhrase{
			fieldName: "foo",
			phrase:    "bar",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &filterPhrase{
			fieldName: "foo",
			phrase:    "",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &filterPhrase{
			fieldName: "foo",
			phrase:    "33",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &filterPhrase{
			fieldName: "foo",
			phrase:    "123456",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)
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
		pf := &filterPhrase{
			fieldName: "foo",
			phrase:    "1234",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0, 4})

		pf = &filterPhrase{
			fieldName: "foo",
			phrase:    "65536",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{3})

		pf = &filterPhrase{
			fieldName: "non-existing-column",
			phrase:    "",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9})

		// mismatch
		pf = &filterPhrase{
			fieldName: "foo",
			phrase:    "bar",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &filterPhrase{
			fieldName: "foo",
			phrase:    "",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &filterPhrase{
			fieldName: "foo",
			phrase:    "33",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &filterPhrase{
			fieldName: "foo",
			phrase:    "12345678901",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)
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
		pf := &filterPhrase{
			fieldName: "foo",
			phrase:    "1234",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0})

		pf = &filterPhrase{
			fieldName: "foo",
			phrase:    "12345678901",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{4})

		pf = &filterPhrase{
			fieldName: "non-existing-column",
			phrase:    "",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8})

		// mismatch
		pf = &filterPhrase{
			fieldName: "foo",
			phrase:    "bar",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &filterPhrase{
			fieldName: "foo",
			phrase:    "",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &filterPhrase{
			fieldName: "foo",
			phrase:    "33",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &filterPhrase{
			fieldName: "foo",
			phrase:    "12345678901234567890",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)
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
		pf := &filterPhrase{
			fieldName: "foo",
			phrase:    "1234",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0, 4})

		pf = &filterPhrase{
			fieldName: "foo",
			phrase:    "1234.5678901",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{4})

		pf = &filterPhrase{
			fieldName: "foo",
			phrase:    "5678901",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{4})

		pf = &filterPhrase{
			fieldName: "foo",
			phrase:    "-65536",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{3})

		pf = &filterPhrase{
			fieldName: "foo",
			phrase:    "65536",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{3})

		pf = &filterPhrase{
			fieldName: "non-existing-column",
			phrase:    "",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8})

		// mismatch
		pf = &filterPhrase{
			fieldName: "foo",
			phrase:    "bar",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &filterPhrase{
			fieldName: "foo",
			phrase:    "-1234",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &filterPhrase{
			fieldName: "foo",
			phrase:    "+1234",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &filterPhrase{
			fieldName: "foo",
			phrase:    "",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &filterPhrase{
			fieldName: "foo",
			phrase:    "123",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &filterPhrase{
			fieldName: "foo",
			phrase:    "5678",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &filterPhrase{
			fieldName: "foo",
			phrase:    "33",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &filterPhrase{
			fieldName: "foo",
			phrase:    "12345678901234567890",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)
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
		pf := &filterPhrase{
			fieldName: "foo",
			phrase:    "127.0.0.1",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{2, 4, 5, 7})

		pf = &filterPhrase{
			fieldName: "foo",
			phrase:    "127",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{2, 4, 5, 6, 7, 8})

		pf = &filterPhrase{
			fieldName: "foo",
			phrase:    "127.0.0",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{2, 4, 5, 7})

		pf = &filterPhrase{
			fieldName: "foo",
			phrase:    "2.3",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0})

		pf = &filterPhrase{
			fieldName: "foo",
			phrase:    "0",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{1, 2, 4, 5, 6, 7, 8})

		pf = &filterPhrase{
			fieldName: "non-existing-column",
			phrase:    "",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11})

		// mismatch
		pf = &filterPhrase{
			fieldName: "foo",
			phrase:    "bar",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &filterPhrase{
			fieldName: "foo",
			phrase:    "",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &filterPhrase{
			fieldName: "foo",
			phrase:    "5",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &filterPhrase{
			fieldName: "foo",
			phrase:    "127.1",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &filterPhrase{
			fieldName: "foo",
			phrase:    "27.0",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &filterPhrase{
			fieldName: "foo",
			phrase:    "255.255.255.255",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)
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
		pf := &filterPhrase{
			fieldName: "_msg",
			phrase:    "2006-01-02T15:04:05.005Z",
		}
		testFilterMatchForColumns(t, columns, pf, "_msg", []int{4})

		pf = &filterPhrase{
			fieldName: "_msg",
			phrase:    "2006-01",
		}
		testFilterMatchForColumns(t, columns, pf, "_msg", []int{0, 1, 2, 3, 4, 5, 6, 7, 8})

		pf = &filterPhrase{
			fieldName: "_msg",
			phrase:    "002Z",
		}
		testFilterMatchForColumns(t, columns, pf, "_msg", []int{1})

		pf = &filterPhrase{
			fieldName: "non-existing-column",
			phrase:    "",
		}
		testFilterMatchForColumns(t, columns, pf, "_msg", []int{0, 1, 2, 3, 4, 5, 6, 7, 8})

		// mimatch
		pf = &filterPhrase{
			fieldName: "_msg",
			phrase:    "bar",
		}
		testFilterMatchForColumns(t, columns, pf, "_msg", nil)

		pf = &filterPhrase{
			fieldName: "_msg",
			phrase:    "",
		}
		testFilterMatchForColumns(t, columns, pf, "_msg", nil)

		pf = &filterPhrase{
			fieldName: "_msg",
			phrase:    "2006-03-02T15:04:05.005Z",
		}
		testFilterMatchForColumns(t, columns, pf, "_msg", nil)

		pf = &filterPhrase{
			fieldName: "_msg",
			phrase:    "06",
		}
		testFilterMatchForColumns(t, columns, pf, "_msg", nil)

		// This filter shouldn't match row=4, since it has different string representation of the timestamp
		pf = &filterPhrase{
			fieldName: "_msg",
			phrase:    "2006-01-02T16:04:05.005+01:00",
		}
		testFilterMatchForColumns(t, columns, pf, "_msg", nil)

		// This filter shouldn't match row=4, since it contains too many digits for millisecond part
		pf = &filterPhrase{
			fieldName: "_msg",
			phrase:    "2006-01-02T15:04:05.00500Z",
		}
		testFilterMatchForColumns(t, columns, pf, "_msg", nil)
	})
}
