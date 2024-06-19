package logstorage

import (
	"reflect"
	"slices"
	"testing"
)

func TestFilterIn(t *testing.T) {
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
		fi := &filterIn{
			fieldName: "foo",
			values:    []string{"abc def", "abc", "foobar"},
		}
		testFilterMatchForColumns(t, columns, fi, "foo", []int{0})

		fi = &filterIn{
			fieldName: "other column",
			values:    []string{"asdfdsf", ""},
		}
		testFilterMatchForColumns(t, columns, fi, "foo", []int{0})

		fi = &filterIn{
			fieldName: "non-existing-column",
			values:    []string{"", "foo"},
		}
		testFilterMatchForColumns(t, columns, fi, "foo", []int{0})

		// mismatch
		fi = &filterIn{
			fieldName: "foo",
			values:    []string{"abc", "def"},
		}
		testFilterMatchForColumns(t, columns, fi, "foo", nil)

		fi = &filterIn{
			fieldName: "foo",
			values:    []string{},
		}
		testFilterMatchForColumns(t, columns, fi, "foo", nil)

		fi = &filterIn{
			fieldName: "foo",
			values:    []string{"", "abc"},
		}
		testFilterMatchForColumns(t, columns, fi, "foo", nil)

		fi = &filterIn{
			fieldName: "other column",
			values:    []string{"sd"},
		}
		testFilterMatchForColumns(t, columns, fi, "foo", nil)

		fi = &filterIn{
			fieldName: "non-existing column",
			values:    []string{"abc", "def"},
		}
		testFilterMatchForColumns(t, columns, fi, "foo", nil)
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
		fi := &filterIn{
			fieldName: "foo",
			values:    []string{"aaaa", "abc def", "foobar"},
		}
		testFilterMatchForColumns(t, columns, fi, "foo", []int{0, 1, 2})

		fi = &filterIn{
			fieldName: "non-existing-column",
			values:    []string{"", "abc"},
		}
		testFilterMatchForColumns(t, columns, fi, "foo", []int{0, 1, 2})

		// mismatch
		fi = &filterIn{
			fieldName: "foo",
			values:    []string{"abc def ", "foobar"},
		}
		testFilterMatchForColumns(t, columns, fi, "foo", nil)

		fi = &filterIn{
			fieldName: "foo",
			values:    []string{},
		}
		testFilterMatchForColumns(t, columns, fi, "foo", nil)

		fi = &filterIn{
			fieldName: "foo",
			values:    []string{""},
		}
		testFilterMatchForColumns(t, columns, fi, "foo", nil)

		fi = &filterIn{
			fieldName: "non-existing column",
			values:    []string{"x"},
		}
		testFilterMatchForColumns(t, columns, fi, "foo", nil)
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
		fi := &filterIn{
			fieldName: "foo",
			values:    []string{"foobar", "aaaa", "abc", "baz"},
		}
		testFilterMatchForColumns(t, columns, fi, "foo", []int{1, 2, 6})

		fi = &filterIn{
			fieldName: "foo",
			values:    []string{"bbbb", "", "aaaa"},
		}
		testFilterMatchForColumns(t, columns, fi, "foo", []int{0})

		fi = &filterIn{
			fieldName: "non-existing-column",
			values:    []string{""},
		}
		testFilterMatchForColumns(t, columns, fi, "foo", []int{0, 1, 2, 3, 4, 5, 6})

		// mismatch
		fi = &filterIn{
			fieldName: "foo",
			values:    []string{"bar", "aaaa"},
		}
		testFilterMatchForColumns(t, columns, fi, "foo", nil)

		fi = &filterIn{
			fieldName: "foo",
			values:    []string{},
		}
		testFilterMatchForColumns(t, columns, fi, "foo", nil)

		fi = &filterIn{
			fieldName: "non-existing column",
			values:    []string{"foobar", "aaaa"},
		}
		testFilterMatchForColumns(t, columns, fi, "foo", nil)
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
		fi := &filterIn{
			fieldName: "foo",
			values:    []string{"a foobar", "aa abc a"},
		}
		testFilterMatchForColumns(t, columns, fi, "foo", []int{1, 2, 6})

		fi = &filterIn{
			fieldName: "non-existing-column",
			values:    []string{""},
		}
		testFilterMatchForColumns(t, columns, fi, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9})

		// mismatch
		fi = &filterIn{
			fieldName: "foo",
			values:    []string{"aa a"},
		}
		testFilterMatchForColumns(t, columns, fi, "foo", nil)

		fi = &filterIn{
			fieldName: "foo",
			values:    []string{},
		}
		testFilterMatchForColumns(t, columns, fi, "foo", nil)

		fi = &filterIn{
			fieldName: "foo",
			values:    []string{""},
		}
		testFilterMatchForColumns(t, columns, fi, "foo", nil)
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
		fi := &filterIn{
			fieldName: "foo",
			values:    []string{"12", "32"},
		}
		testFilterMatchForColumns(t, columns, fi, "foo", []int{1, 2, 5})

		fi = &filterIn{
			fieldName: "foo",
			values:    []string{"0"},
		}
		testFilterMatchForColumns(t, columns, fi, "foo", []int{3, 4})

		fi = &filterIn{
			fieldName: "non-existing-column",
			values:    []string{""},
		}
		testFilterMatchForColumns(t, columns, fi, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10})

		// mismatch
		fi = &filterIn{
			fieldName: "foo",
			values:    []string{"bar"},
		}
		testFilterMatchForColumns(t, columns, fi, "foo", nil)

		fi = &filterIn{
			fieldName: "foo",
			values:    []string{},
		}
		testFilterMatchForColumns(t, columns, fi, "foo", nil)

		fi = &filterIn{
			fieldName: "foo",
			values:    []string{"33"},
		}
		testFilterMatchForColumns(t, columns, fi, "foo", nil)

		fi = &filterIn{
			fieldName: "foo",
			values:    []string{"1234"},
		}
		testFilterMatchForColumns(t, columns, fi, "foo", nil)
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
		fi := &filterIn{
			fieldName: "foo",
			values:    []string{"12", "32"},
		}
		testFilterMatchForColumns(t, columns, fi, "foo", []int{1, 2, 5})

		fi = &filterIn{
			fieldName: "foo",
			values:    []string{"0"},
		}
		testFilterMatchForColumns(t, columns, fi, "foo", []int{3, 4})

		fi = &filterIn{
			fieldName: "non-existing-column",
			values:    []string{""},
		}
		testFilterMatchForColumns(t, columns, fi, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10})

		// mismatch
		fi = &filterIn{
			fieldName: "foo",
			values:    []string{"bar"},
		}
		testFilterMatchForColumns(t, columns, fi, "foo", nil)

		fi = &filterIn{
			fieldName: "foo",
			values:    []string{},
		}
		testFilterMatchForColumns(t, columns, fi, "foo", nil)

		fi = &filterIn{
			fieldName: "foo",
			values:    []string{"33"},
		}
		testFilterMatchForColumns(t, columns, fi, "foo", nil)

		fi = &filterIn{
			fieldName: "foo",
			values:    []string{"123456"},
		}
		testFilterMatchForColumns(t, columns, fi, "foo", nil)
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
		fi := &filterIn{
			fieldName: "foo",
			values:    []string{"12", "32"},
		}
		testFilterMatchForColumns(t, columns, fi, "foo", []int{1, 2, 5})

		fi = &filterIn{
			fieldName: "foo",
			values:    []string{"0"},
		}
		testFilterMatchForColumns(t, columns, fi, "foo", []int{3, 4})

		fi = &filterIn{
			fieldName: "non-existing-column",
			values:    []string{""},
		}
		testFilterMatchForColumns(t, columns, fi, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10})

		// mismatch
		fi = &filterIn{
			fieldName: "foo",
			values:    []string{"bar"},
		}
		testFilterMatchForColumns(t, columns, fi, "foo", nil)

		fi = &filterIn{
			fieldName: "foo",
			values:    []string{},
		}
		testFilterMatchForColumns(t, columns, fi, "foo", nil)

		fi = &filterIn{
			fieldName: "foo",
			values:    []string{"33"},
		}
		testFilterMatchForColumns(t, columns, fi, "foo", nil)

		fi = &filterIn{
			fieldName: "foo",
			values:    []string{"12345678901"},
		}
		testFilterMatchForColumns(t, columns, fi, "foo", nil)
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
		fi := &filterIn{
			fieldName: "foo",
			values:    []string{"12", "32"},
		}
		testFilterMatchForColumns(t, columns, fi, "foo", []int{1, 2, 5})

		fi = &filterIn{
			fieldName: "foo",
			values:    []string{"0"},
		}
		testFilterMatchForColumns(t, columns, fi, "foo", []int{3, 4})

		fi = &filterIn{
			fieldName: "non-existing-column",
			values:    []string{""},
		}
		testFilterMatchForColumns(t, columns, fi, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10})

		// mismatch
		fi = &filterIn{
			fieldName: "foo",
			values:    []string{"bar"},
		}
		testFilterMatchForColumns(t, columns, fi, "foo", nil)

		fi = &filterIn{
			fieldName: "foo",
			values:    []string{},
		}
		testFilterMatchForColumns(t, columns, fi, "foo", nil)

		fi = &filterIn{
			fieldName: "foo",
			values:    []string{"33"},
		}
		testFilterMatchForColumns(t, columns, fi, "foo", nil)
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
		fi := &filterIn{
			fieldName: "foo",
			values:    []string{"1234", "1", "foobar", "123211"},
		}
		testFilterMatchForColumns(t, columns, fi, "foo", []int{0, 5})

		fi = &filterIn{
			fieldName: "foo",
			values:    []string{"1234.5678901"},
		}
		testFilterMatchForColumns(t, columns, fi, "foo", []int{4})

		fi = &filterIn{
			fieldName: "foo",
			values:    []string{"-65536"},
		}
		testFilterMatchForColumns(t, columns, fi, "foo", []int{3})

		fi = &filterIn{
			fieldName: "non-existing-column",
			values:    []string{""},
		}
		testFilterMatchForColumns(t, columns, fi, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8})

		// mismatch
		fi = &filterIn{
			fieldName: "foo",
			values:    []string{"bar"},
		}
		testFilterMatchForColumns(t, columns, fi, "foo", nil)

		fi = &filterIn{
			fieldName: "foo",
			values:    []string{"65536"},
		}
		testFilterMatchForColumns(t, columns, fi, "foo", nil)

		fi = &filterIn{
			fieldName: "foo",
			values:    []string{},
		}
		testFilterMatchForColumns(t, columns, fi, "foo", nil)

		fi = &filterIn{
			fieldName: "foo",
			values:    []string{""},
		}
		testFilterMatchForColumns(t, columns, fi, "foo", nil)

		fi = &filterIn{
			fieldName: "foo",
			values:    []string{"123"},
		}
		testFilterMatchForColumns(t, columns, fi, "foo", nil)

		fi = &filterIn{
			fieldName: "foo",
			values:    []string{"12345678901234567890"},
		}
		testFilterMatchForColumns(t, columns, fi, "foo", nil)
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
		fi := &filterIn{
			fieldName: "foo",
			values:    []string{"127.0.0.1", "24.54.1.2", "127.0.4.2"},
		}
		testFilterMatchForColumns(t, columns, fi, "foo", []int{2, 4, 5, 6, 7})

		fi = &filterIn{
			fieldName: "non-existing-column",
			values:    []string{""},
		}
		testFilterMatchForColumns(t, columns, fi, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11})

		// mismatch
		fi = &filterIn{
			fieldName: "foo",
			values:    []string{"bar"},
		}
		testFilterMatchForColumns(t, columns, fi, "foo", nil)

		fi = &filterIn{
			fieldName: "foo",
			values:    []string{},
		}
		testFilterMatchForColumns(t, columns, fi, "foo", nil)

		fi = &filterIn{
			fieldName: "foo",
			values:    []string{""},
		}
		testFilterMatchForColumns(t, columns, fi, "foo", nil)

		fi = &filterIn{
			fieldName: "foo",
			values:    []string{"5"},
		}
		testFilterMatchForColumns(t, columns, fi, "foo", nil)

		fi = &filterIn{
			fieldName: "foo",
			values:    []string{"255.255.255.255"},
		}
		testFilterMatchForColumns(t, columns, fi, "foo", nil)
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
		fi := &filterIn{
			fieldName: "_msg",
			values:    []string{"2006-01-02T15:04:05.005Z", "foobar"},
		}
		testFilterMatchForColumns(t, columns, fi, "_msg", []int{4})

		fi = &filterIn{
			fieldName: "non-existing-column",
			values:    []string{""},
		}
		testFilterMatchForColumns(t, columns, fi, "_msg", []int{0, 1, 2, 3, 4, 5, 6, 7, 8})

		// mimatch
		fi = &filterIn{
			fieldName: "_msg",
			values:    []string{"bar"},
		}
		testFilterMatchForColumns(t, columns, fi, "_msg", nil)

		fi = &filterIn{
			fieldName: "_msg",
			values:    []string{},
		}
		testFilterMatchForColumns(t, columns, fi, "_msg", nil)

		fi = &filterIn{
			fieldName: "_msg",
			values:    []string{""},
		}
		testFilterMatchForColumns(t, columns, fi, "_msg", nil)

		fi = &filterIn{
			fieldName: "_msg",
			values:    []string{"2006-04-02T15:04:05.005Z"},
		}
		testFilterMatchForColumns(t, columns, fi, "_msg", nil)
	})
}

func TestGetCommonTokensAndTokenSets(t *testing.T) {
	f := func(values []string, commonTokensExpected []string, tokenSetsExpected [][]string) {
		t.Helper()

		commonTokens, tokenSets := getCommonTokensAndTokenSets(values)
		slices.Sort(commonTokens)

		if !reflect.DeepEqual(commonTokens, commonTokensExpected) {
			t.Fatalf("unexpected commonTokens for values=%q\ngot\n%q\nwant\n%q", values, commonTokens, commonTokensExpected)
		}

		for i, tokens := range tokenSets {
			slices.Sort(tokens)
			tokensExpected := tokenSetsExpected[i]
			if !reflect.DeepEqual(tokens, tokensExpected) {
				t.Fatalf("unexpected tokens for value=%q\ngot\n%q\nwant\n%q", values[i], tokens, tokensExpected)
			}
		}
	}

	f(nil, nil, nil)
	f([]string{"foo"}, []string{"foo"}, nil)
	f([]string{"foo", "foo"}, []string{"foo"}, nil)
	f([]string{"foo", "bar", "bar", "foo"}, nil, [][]string{{"foo"}, {"bar"}, {"bar"}, {"foo"}})
	f([]string{"foo", "foo bar", "bar foo"}, []string{"foo"}, nil)
	f([]string{"a foo bar", "bar abc foo", "foo abc a bar"}, []string{"bar", "foo"}, [][]string{{"a"}, {"abc"}, {"a", "abc"}})
	f([]string{"a xfoo bar", "xbar abc foo", "foo abc a bar"}, nil, [][]string{{"a", "bar", "xfoo"}, {"abc", "foo", "xbar"}, {"a", "abc", "bar", "foo"}})
}
