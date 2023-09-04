package logstorage

import (
	"fmt"
	"math"
	"reflect"
	"regexp"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
)

func TestMatchLenRange(t *testing.T) {
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

func TestMatchPhrase(t *testing.T) {
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

func TestMatchPrefix(t *testing.T) {
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

func TestMatchSequence(t *testing.T) {
	f := func(s string, phrases []string, resultExpected bool) {
		t.Helper()
		result := matchSequence(s, phrases)
		if result != resultExpected {
			t.Fatalf("unexpected result; got %v; want %v", result, resultExpected)
		}
	}

	f("", []string{""}, true)
	f("foo", []string{""}, true)
	f("", []string{"foo"}, false)
	f("foo", []string{"foo"}, true)
	f("foo bar", []string{"foo"}, true)
	f("foo bar", []string{"bar"}, true)
	f("foo bar", []string{"foo bar"}, true)
	f("foo bar", []string{"foo", "bar"}, true)
	f("foo bar", []string{"foo", " bar"}, true)
	f("foo bar", []string{"foo ", "bar"}, true)
	f("foo bar", []string{"foo ", " bar"}, false)
	f("foo bar", []string{"bar", "foo"}, false)
}

func TestMatchStringRange(t *testing.T) {
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

func TestMatchIPv4Range(t *testing.T) {
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

func TestFilterBitmap(t *testing.T) {
	for i := 0; i < 100; i++ {
		bm := getFilterBitmap(i)
		if bm.bitsLen != i {
			t.Fatalf("unexpected bits length: %d; want %d", bm.bitsLen, i)
		}

		// Make sure that all the bits are set.
		nextIdx := 0
		bm.forEachSetBit(func(idx int) bool {
			if idx >= i {
				t.Fatalf("index must be smaller than %d", i)
			}
			if idx != nextIdx {
				t.Fatalf("unexpected idx; got %d; want %d", idx, nextIdx)
			}
			nextIdx++
			return true
		})

		// Clear a part of bits
		bm.forEachSetBit(func(idx int) bool {
			return idx%2 != 0
		})
		nextIdx = 1
		bm.forEachSetBit(func(idx int) bool {
			if idx != nextIdx {
				t.Fatalf("unexpected idx; got %d; want %d", idx, nextIdx)
			}
			nextIdx += 2
			return true
		})

		// Clear all the bits
		bm.forEachSetBit(func(idx int) bool {
			return false
		})
		bitsCount := 0
		bm.forEachSetBit(func(idx int) bool {
			bitsCount++
			return true
		})
		if bitsCount != 0 {
			t.Fatalf("unexpected non-zero number of set bits remained: %d", bitsCount)
		}

		putFilterBitmap(bm)
	}
}

func TestComplexFilters(t *testing.T) {
	columns := []column{
		{
			name: "foo",
			values: []string{
				"a foo",
				"a foobar",
				"aa abc a",
				"ca afdf a,foobar baz",
				"a fddf foobarbaz",
				"a",
				"a foobar abcdef",
				"a kjlkjf dfff",
				"a ТЕСТЙЦУК НГКШ ",
				"a !!,23.(!1)",
			},
		},
	}

	// (foobar AND NOT baz AND (abcdef OR xyz))
	f := &andFilter{
		filters: []filter{
			&phraseFilter{
				fieldName: "foo",
				phrase:    "foobar",
			},
			&notFilter{
				f: &phraseFilter{
					fieldName: "foo",
					phrase:    "baz",
				},
			},
			&orFilter{
				filters: []filter{
					&phraseFilter{
						fieldName: "foo",
						phrase:    "abcdef",
					},
					&phraseFilter{
						fieldName: "foo",
						phrase:    "xyz",
					},
				},
			},
		},
	}
	testFilterMatchForColumns(t, columns, f, "foo", []int{6})

	// (foobaz AND NOT baz AND (abcdef OR xyz))
	f = &andFilter{
		filters: []filter{
			&phraseFilter{
				fieldName: "foo",
				phrase:    "foobaz",
			},
			&notFilter{
				f: &phraseFilter{
					fieldName: "foo",
					phrase:    "baz",
				},
			},
			&orFilter{
				filters: []filter{
					&phraseFilter{
						fieldName: "foo",
						phrase:    "abcdef",
					},
					&phraseFilter{
						fieldName: "foo",
						phrase:    "xyz",
					},
				},
			},
		},
	}
	testFilterMatchForColumns(t, columns, f, "foo", nil)

	// (foobaz AND NOT baz AND (abcdef OR xyz OR a))
	f = &andFilter{
		filters: []filter{
			&phraseFilter{
				fieldName: "foo",
				phrase:    "foobar",
			},
			&notFilter{
				f: &phraseFilter{
					fieldName: "foo",
					phrase:    "baz",
				},
			},
			&orFilter{
				filters: []filter{
					&phraseFilter{
						fieldName: "foo",
						phrase:    "abcdef",
					},
					&phraseFilter{
						fieldName: "foo",
						phrase:    "xyz",
					},
					&phraseFilter{
						fieldName: "foo",
						phrase:    "a",
					},
				},
			},
		},
	}
	testFilterMatchForColumns(t, columns, f, "foo", []int{1, 6})

	// (foobaz AND NOT qwert AND (abcdef OR xyz OR a))
	f = &andFilter{
		filters: []filter{
			&phraseFilter{
				fieldName: "foo",
				phrase:    "foobar",
			},
			&notFilter{
				f: &phraseFilter{
					fieldName: "foo",
					phrase:    "qwert",
				},
			},
			&orFilter{
				filters: []filter{
					&phraseFilter{
						fieldName: "foo",
						phrase:    "abcdef",
					},
					&phraseFilter{
						fieldName: "foo",
						phrase:    "xyz",
					},
					&phraseFilter{
						fieldName: "foo",
						phrase:    "a",
					},
				},
			},
		},
	}
	testFilterMatchForColumns(t, columns, f, "foo", []int{1, 3, 6})
}

func TestOrFilter(t *testing.T) {
	columns := []column{
		{
			name: "foo",
			values: []string{
				"a foo",
				"a foobar",
				"aa abc a",
				"ca afdf a,foobar baz",
				"a fddf foobarbaz",
				"a",
				"a foobar abcdef",
				"a kjlkjf dfff",
				"a ТЕСТЙЦУК НГКШ ",
				"a !!,23.(!1)",
			},
		},
	}

	// non-empty union
	of := &orFilter{
		filters: []filter{
			&phraseFilter{
				fieldName: "foo",
				phrase:    "23",
			},
			&prefixFilter{
				fieldName: "foo",
				prefix:    "abc",
			},
		},
	}
	testFilterMatchForColumns(t, columns, of, "foo", []int{2, 6, 9})

	// reverse non-empty union
	of = &orFilter{
		filters: []filter{
			&prefixFilter{
				fieldName: "foo",
				prefix:    "abc",
			},
			&phraseFilter{
				fieldName: "foo",
				phrase:    "23",
			},
		},
	}
	testFilterMatchForColumns(t, columns, of, "foo", []int{2, 6, 9})

	// first empty result, second non-empty result
	of = &orFilter{
		filters: []filter{
			&prefixFilter{
				fieldName: "foo",
				prefix:    "xabc",
			},
			&phraseFilter{
				fieldName: "foo",
				phrase:    "23",
			},
		},
	}
	testFilterMatchForColumns(t, columns, of, "foo", []int{9})

	// first non-empty result, second empty result
	of = &orFilter{
		filters: []filter{
			&phraseFilter{
				fieldName: "foo",
				phrase:    "23",
			},
			&prefixFilter{
				fieldName: "foo",
				prefix:    "xabc",
			},
		},
	}
	testFilterMatchForColumns(t, columns, of, "foo", []int{9})

	// first match all
	of = &orFilter{
		filters: []filter{
			&phraseFilter{
				fieldName: "foo",
				phrase:    "a",
			},
			&prefixFilter{
				fieldName: "foo",
				prefix:    "23",
			},
		},
	}
	testFilterMatchForColumns(t, columns, of, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9})

	// second match all
	of = &orFilter{
		filters: []filter{
			&prefixFilter{
				fieldName: "foo",
				prefix:    "23",
			},
			&phraseFilter{
				fieldName: "foo",
				phrase:    "a",
			},
		},
	}
	testFilterMatchForColumns(t, columns, of, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9})

	// both empty results
	of = &orFilter{
		filters: []filter{
			&phraseFilter{
				fieldName: "foo",
				phrase:    "x23",
			},
			&prefixFilter{
				fieldName: "foo",
				prefix:    "xabc",
			},
		},
	}
	testFilterMatchForColumns(t, columns, of, "foo", nil)
}

func TestAndFilter(t *testing.T) {
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
				"a foobar abcdef",
				"a kjlkjf dfff",
				"a ТЕСТЙЦУК НГКШ ",
				"a !!,23.(!1)",
			},
		},
	}

	// non-empty intersection
	af := &andFilter{
		filters: []filter{
			&phraseFilter{
				fieldName: "foo",
				phrase:    "a",
			},
			&prefixFilter{
				fieldName: "foo",
				prefix:    "abc",
			},
		},
	}
	testFilterMatchForColumns(t, columns, af, "foo", []int{2, 6})

	// reverse non-empty intersection
	af = &andFilter{
		filters: []filter{
			&prefixFilter{
				fieldName: "foo",
				prefix:    "abc",
			},
			&phraseFilter{
				fieldName: "foo",
				phrase:    "a",
			},
		},
	}
	testFilterMatchForColumns(t, columns, af, "foo", []int{2, 6})

	// the first filter mismatch
	af = &andFilter{
		filters: []filter{
			&prefixFilter{
				fieldName: "foo",
				prefix:    "bc",
			},
			&phraseFilter{
				fieldName: "foo",
				phrase:    "a",
			},
		},
	}
	testFilterMatchForColumns(t, columns, af, "foo", nil)

	// the last filter mismatch
	af = &andFilter{
		filters: []filter{
			&phraseFilter{
				fieldName: "foo",
				phrase:    "abc",
			},
			&prefixFilter{
				fieldName: "foo",
				prefix:    "foo",
			},
		},
	}
	testFilterMatchForColumns(t, columns, af, "foo", nil)

	// empty intersection
	af = &andFilter{
		filters: []filter{
			&phraseFilter{
				fieldName: "foo",
				phrase:    "foo",
			},
			&prefixFilter{
				fieldName: "foo",
				prefix:    "abc",
			},
		},
	}
	testFilterMatchForColumns(t, columns, af, "foo", nil)

	// reverse empty intersection
	af = &andFilter{
		filters: []filter{
			&prefixFilter{
				fieldName: "foo",
				prefix:    "abc",
			},
			&phraseFilter{
				fieldName: "foo",
				phrase:    "foo",
			},
		},
	}
	testFilterMatchForColumns(t, columns, af, "foo", nil)
}

func TestNotFilter(t *testing.T) {
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
				"a foobar",
				"a kjlkjf dfff",
				"a ТЕСТЙЦУК НГКШ ",
				"a !!,23.(!1)",
			},
		},
	}

	// match
	nf := &notFilter{
		f: &phraseFilter{
			fieldName: "foo",
			phrase:    "",
		},
	}
	testFilterMatchForColumns(t, columns, nf, "foo", []int{0, 1, 2, 3, 4, 6, 7, 8, 9})

	nf = &notFilter{
		f: &phraseFilter{
			fieldName: "foo",
			phrase:    "a",
		},
	}
	testFilterMatchForColumns(t, columns, nf, "foo", []int{5})

	nf = &notFilter{
		f: &phraseFilter{
			fieldName: "non-existing-field",
			phrase:    "foobar",
		},
	}
	testFilterMatchForColumns(t, columns, nf, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9})

	nf = &notFilter{
		f: &prefixFilter{
			fieldName: "non-existing-field",
			prefix:    "",
		},
	}
	testFilterMatchForColumns(t, columns, nf, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9})

	nf = &notFilter{
		f: &prefixFilter{
			fieldName: "foo",
			prefix:    "",
		},
	}
	testFilterMatchForColumns(t, columns, nf, "foo", []int{5})

	// mismatch
	nf = &notFilter{
		f: &phraseFilter{
			fieldName: "non-existing-field",
			phrase:    "",
		},
	}
	testFilterMatchForColumns(t, columns, nf, "foo", nil)
}

func TestTimeFilter(t *testing.T) {
	timestamps := []int64{
		1,
		9,
		123,
		456,
		789,
	}

	// match
	tf := &timeFilter{
		minTimestamp: -10,
		maxTimestamp: 1,
	}
	testFilterMatchForTimestamps(t, timestamps, tf, []int{0})

	tf = &timeFilter{
		minTimestamp: -10,
		maxTimestamp: 10,
	}
	testFilterMatchForTimestamps(t, timestamps, tf, []int{0, 1})

	tf = &timeFilter{
		minTimestamp: 1,
		maxTimestamp: 1,
	}
	testFilterMatchForTimestamps(t, timestamps, tf, []int{0})

	tf = &timeFilter{
		minTimestamp: 2,
		maxTimestamp: 456,
	}
	testFilterMatchForTimestamps(t, timestamps, tf, []int{1, 2, 3})

	tf = &timeFilter{
		minTimestamp: 2,
		maxTimestamp: 457,
	}
	testFilterMatchForTimestamps(t, timestamps, tf, []int{1, 2, 3})

	tf = &timeFilter{
		minTimestamp: 120,
		maxTimestamp: 788,
	}
	testFilterMatchForTimestamps(t, timestamps, tf, []int{2, 3})

	tf = &timeFilter{
		minTimestamp: 120,
		maxTimestamp: 789,
	}
	testFilterMatchForTimestamps(t, timestamps, tf, []int{2, 3, 4})

	tf = &timeFilter{
		minTimestamp: 120,
		maxTimestamp: 10000,
	}
	testFilterMatchForTimestamps(t, timestamps, tf, []int{2, 3, 4})

	tf = &timeFilter{
		minTimestamp: 789,
		maxTimestamp: 1000,
	}
	testFilterMatchForTimestamps(t, timestamps, tf, []int{4})

	// mismatch
	tf = &timeFilter{
		minTimestamp: -1000,
		maxTimestamp: 0,
	}
	testFilterMatchForTimestamps(t, timestamps, tf, nil)

	tf = &timeFilter{
		minTimestamp: 790,
		maxTimestamp: 1000,
	}
	testFilterMatchForTimestamps(t, timestamps, tf, nil)
}

func TestStreamFilter(t *testing.T) {
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
				"a foobar",
				"a kjlkjf dfff",
				"a ТЕСТЙЦУК НГКШ ",
				"a !!,23.(!1)",
			},
		},
	}

	// Match
	f := &exactFilter{
		fieldName: "job",
		value:     "foobar",
	}
	testFilterMatchForColumns(t, columns, f, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9})

	// Mismatch
	f = &exactFilter{
		fieldName: "job",
		value:     "abc",
	}
	testFilterMatchForColumns(t, columns, f, "foo", nil)
}

func TestSequenceFilter(t *testing.T) {
	t.Run("single-row", func(t *testing.T) {
		columns := []column{
			{
				name: "foo",
				values: []string{
					"abc def",
				},
			},
		}

		// match
		sf := &sequenceFilter{
			fieldName: "foo",
			phrases:   []string{"abc"},
		}
		testFilterMatchForColumns(t, columns, sf, "foo", []int{0})

		sf = &sequenceFilter{
			fieldName: "foo",
			phrases:   []string{"def"},
		}
		testFilterMatchForColumns(t, columns, sf, "foo", []int{0})

		sf = &sequenceFilter{
			fieldName: "foo",
			phrases:   []string{"abc def"},
		}
		testFilterMatchForColumns(t, columns, sf, "foo", []int{0})

		sf = &sequenceFilter{
			fieldName: "foo",
			phrases:   []string{"abc ", "", "def", ""},
		}
		testFilterMatchForColumns(t, columns, sf, "foo", []int{0})

		sf = &sequenceFilter{
			fieldName: "foo",
			phrases:   []string{},
		}
		testFilterMatchForColumns(t, columns, sf, "foo", []int{0})

		sf = &sequenceFilter{
			fieldName: "foo",
			phrases:   []string{""},
		}
		testFilterMatchForColumns(t, columns, sf, "foo", []int{0})

		sf = &sequenceFilter{
			fieldName: "non-existing-column",
			phrases:   []string{""},
		}
		testFilterMatchForColumns(t, columns, sf, "foo", []int{0})

		// mismatch
		sf = &sequenceFilter{
			fieldName: "foo",
			phrases:   []string{"ab"},
		}
		testFilterMatchForColumns(t, columns, sf, "foo", nil)

		sf = &sequenceFilter{
			fieldName: "foo",
			phrases:   []string{"abc", "abc"},
		}
		testFilterMatchForColumns(t, columns, sf, "foo", nil)

		sf = &sequenceFilter{
			fieldName: "foo",
			phrases:   []string{"abc", "def", "foo"},
		}
		testFilterMatchForColumns(t, columns, sf, "foo", nil)
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
		}

		// match
		sf := &sequenceFilter{
			fieldName: "foo",
			phrases:   []string{"abc", " def"},
		}
		testFilterMatchForColumns(t, columns, sf, "foo", []int{0, 1, 2})

		sf = &sequenceFilter{
			fieldName: "foo",
			phrases:   []string{"abc ", ""},
		}
		testFilterMatchForColumns(t, columns, sf, "foo", []int{0, 1, 2})

		sf = &sequenceFilter{
			fieldName: "non-existing-column",
			phrases:   []string{"", ""},
		}
		testFilterMatchForColumns(t, columns, sf, "foo", []int{0, 1, 2})

		sf = &sequenceFilter{
			fieldName: "foo",
			phrases:   []string{},
		}
		testFilterMatchForColumns(t, columns, sf, "foo", []int{0, 1, 2})

		// mismatch
		sf = &sequenceFilter{
			fieldName: "foo",
			phrases:   []string{"abc def ", "foobar"},
		}
		testFilterMatchForColumns(t, columns, sf, "foo", nil)

		sf = &sequenceFilter{
			fieldName: "non-existing column",
			phrases:   []string{"x", "yz"},
		}
		testFilterMatchForColumns(t, columns, sf, "foo", nil)
	})

	t.Run("dict", func(t *testing.T) {
		columns := []column{
			{
				name: "foo",
				values: []string{
					"",
					"baz foobar",
					"abc",
					"afdf foobar baz",
					"fddf foobarbaz",
					"afoobarbaz",
					"foobar",
				},
			},
		}

		// match
		sf := &sequenceFilter{
			fieldName: "foo",
			phrases:   []string{"foobar", "baz"},
		}
		testFilterMatchForColumns(t, columns, sf, "foo", []int{3})

		sf = &sequenceFilter{
			fieldName: "foo",
			phrases:   []string{""},
		}
		testFilterMatchForColumns(t, columns, sf, "foo", []int{0, 1, 2, 3, 4, 5, 6})

		sf = &sequenceFilter{
			fieldName: "non-existing-column",
			phrases:   []string{""},
		}
		testFilterMatchForColumns(t, columns, sf, "foo", []int{0, 1, 2, 3, 4, 5, 6})

		sf = &sequenceFilter{
			fieldName: "foo",
			phrases:   []string{},
		}
		testFilterMatchForColumns(t, columns, sf, "foo", []int{0, 1, 2, 3, 4, 5, 6})

		// mismatch
		sf = &sequenceFilter{
			fieldName: "foo",
			phrases:   []string{"baz", "aaaa"},
		}
		testFilterMatchForColumns(t, columns, sf, "foo", nil)

		sf = &sequenceFilter{
			fieldName: "non-existing column",
			phrases:   []string{"foobar", "aaaa"},
		}
		testFilterMatchForColumns(t, columns, sf, "foo", nil)
	})

	t.Run("strings", func(t *testing.T) {
		columns := []column{
			{
				name: "foo",
				values: []string{
					"a bb foo",
					"bb a foobar",
					"aa abc a",
					"ca afdf a,foobar baz",
					"a fddf foobarbaz",
					"a afoobarbaz",
					"a foobar bb",
					"a kjlkjf dfff",
					"a ТЕСТЙЦУК НГКШ ",
					"a !!,23.(!1)",
				},
			},
		}

		// match
		sf := &sequenceFilter{
			fieldName: "foo",
			phrases:   []string{"a", "bb"},
		}
		testFilterMatchForColumns(t, columns, sf, "foo", []int{0, 6})

		sf = &sequenceFilter{
			fieldName: "foo",
			phrases:   []string{"НГКШ", " "},
		}
		testFilterMatchForColumns(t, columns, sf, "foo", []int{8})

		sf = &sequenceFilter{
			fieldName: "foo",
			phrases:   []string{},
		}
		testFilterMatchForColumns(t, columns, sf, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9})

		sf = &sequenceFilter{
			fieldName: "foo",
			phrases:   []string{""},
		}
		testFilterMatchForColumns(t, columns, sf, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9})

		sf = &sequenceFilter{
			fieldName: "non-existing-column",
			phrases:   []string{""},
		}
		testFilterMatchForColumns(t, columns, sf, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9})

		sf = &sequenceFilter{
			fieldName: "foo",
			phrases:   []string{"!,", "(!1)"},
		}
		testFilterMatchForColumns(t, columns, sf, "foo", []int{9})

		// mismatch
		sf = &sequenceFilter{
			fieldName: "foo",
			phrases:   []string{"aa a", "bcdasqq"},
		}
		testFilterMatchForColumns(t, columns, sf, "foo", nil)

		sf = &sequenceFilter{
			fieldName: "foo",
			phrases:   []string{"@", "!!!!"},
		}
		testFilterMatchForColumns(t, columns, sf, "foo", nil)
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
		sf := &sequenceFilter{
			fieldName: "foo",
			phrases:   []string{"12"},
		}
		testFilterMatchForColumns(t, columns, sf, "foo", []int{1, 5})

		sf = &sequenceFilter{
			fieldName: "foo",
			phrases:   []string{},
		}
		testFilterMatchForColumns(t, columns, sf, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10})

		sf = &sequenceFilter{
			fieldName: "foo",
			phrases:   []string{""},
		}
		testFilterMatchForColumns(t, columns, sf, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10})

		sf = &sequenceFilter{
			fieldName: "non-existing-column",
			phrases:   []string{""},
		}
		testFilterMatchForColumns(t, columns, sf, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10})

		// mismatch
		sf = &sequenceFilter{
			fieldName: "foo",
			phrases:   []string{"bar"},
		}
		testFilterMatchForColumns(t, columns, sf, "foo", nil)

		sf = &sequenceFilter{
			fieldName: "foo",
			phrases:   []string{"", "bar"},
		}
		testFilterMatchForColumns(t, columns, sf, "foo", nil)

		sf = &sequenceFilter{
			fieldName: "foo",
			phrases:   []string{"1234"},
		}
		testFilterMatchForColumns(t, columns, sf, "foo", nil)

		sf = &sequenceFilter{
			fieldName: "foo",
			phrases:   []string{"1234", "567"},
		}
		testFilterMatchForColumns(t, columns, sf, "foo", nil)
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
		}

		// match
		sf := &sequenceFilter{
			fieldName: "foo",
			phrases:   []string{"12"},
		}
		testFilterMatchForColumns(t, columns, sf, "foo", []int{1, 5})

		sf = &sequenceFilter{
			fieldName: "foo",
			phrases:   []string{},
		}
		testFilterMatchForColumns(t, columns, sf, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10})

		sf = &sequenceFilter{
			fieldName: "foo",
			phrases:   []string{""},
		}
		testFilterMatchForColumns(t, columns, sf, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10})

		sf = &sequenceFilter{
			fieldName: "non-existing-column",
			phrases:   []string{""},
		}
		testFilterMatchForColumns(t, columns, sf, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10})

		// mismatch
		sf = &sequenceFilter{
			fieldName: "foo",
			phrases:   []string{"bar"},
		}
		testFilterMatchForColumns(t, columns, sf, "foo", nil)

		sf = &sequenceFilter{
			fieldName: "foo",
			phrases:   []string{"", "bar"},
		}
		testFilterMatchForColumns(t, columns, sf, "foo", nil)

		sf = &sequenceFilter{
			fieldName: "foo",
			phrases:   []string{"1234"},
		}
		testFilterMatchForColumns(t, columns, sf, "foo", nil)

		sf = &sequenceFilter{
			fieldName: "foo",
			phrases:   []string{"1234", "567"},
		}
		testFilterMatchForColumns(t, columns, sf, "foo", nil)
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
		}

		// match
		sf := &sequenceFilter{
			fieldName: "foo",
			phrases:   []string{"12"},
		}
		testFilterMatchForColumns(t, columns, sf, "foo", []int{1, 5})

		sf = &sequenceFilter{
			fieldName: "foo",
			phrases:   []string{},
		}
		testFilterMatchForColumns(t, columns, sf, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10})

		sf = &sequenceFilter{
			fieldName: "foo",
			phrases:   []string{""},
		}
		testFilterMatchForColumns(t, columns, sf, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10})

		sf = &sequenceFilter{
			fieldName: "non-existing-column",
			phrases:   []string{""},
		}
		testFilterMatchForColumns(t, columns, sf, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10})

		// mismatch
		sf = &sequenceFilter{
			fieldName: "foo",
			phrases:   []string{"bar"},
		}
		testFilterMatchForColumns(t, columns, sf, "foo", nil)

		sf = &sequenceFilter{
			fieldName: "foo",
			phrases:   []string{"", "bar"},
		}
		testFilterMatchForColumns(t, columns, sf, "foo", nil)

		sf = &sequenceFilter{
			fieldName: "foo",
			phrases:   []string{"1234"},
		}
		testFilterMatchForColumns(t, columns, sf, "foo", nil)

		sf = &sequenceFilter{
			fieldName: "foo",
			phrases:   []string{"1234", "567"},
		}
		testFilterMatchForColumns(t, columns, sf, "foo", nil)
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
		}

		// match
		sf := &sequenceFilter{
			fieldName: "foo",
			phrases:   []string{"12"},
		}
		testFilterMatchForColumns(t, columns, sf, "foo", []int{1, 5})

		sf = &sequenceFilter{
			fieldName: "foo",
			phrases:   []string{},
		}
		testFilterMatchForColumns(t, columns, sf, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10})

		sf = &sequenceFilter{
			fieldName: "foo",
			phrases:   []string{""},
		}
		testFilterMatchForColumns(t, columns, sf, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10})

		sf = &sequenceFilter{
			fieldName: "non-existing-column",
			phrases:   []string{""},
		}
		testFilterMatchForColumns(t, columns, sf, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10})

		// mismatch
		sf = &sequenceFilter{
			fieldName: "foo",
			phrases:   []string{"bar"},
		}
		testFilterMatchForColumns(t, columns, sf, "foo", nil)

		sf = &sequenceFilter{
			fieldName: "foo",
			phrases:   []string{"", "bar"},
		}
		testFilterMatchForColumns(t, columns, sf, "foo", nil)

		sf = &sequenceFilter{
			fieldName: "foo",
			phrases:   []string{"1234"},
		}
		testFilterMatchForColumns(t, columns, sf, "foo", nil)

		sf = &sequenceFilter{
			fieldName: "foo",
			phrases:   []string{"1234", "567"},
		}
		testFilterMatchForColumns(t, columns, sf, "foo", nil)
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
		sf := &sequenceFilter{
			fieldName: "foo",
			phrases:   []string{"-", "65536"},
		}
		testFilterMatchForColumns(t, columns, sf, "foo", []int{3})

		sf = &sequenceFilter{
			fieldName: "foo",
			phrases:   []string{"1234.", "5678901"},
		}
		testFilterMatchForColumns(t, columns, sf, "foo", []int{4})

		sf = &sequenceFilter{
			fieldName: "foo",
			phrases:   []string{"", "5678901"},
		}
		testFilterMatchForColumns(t, columns, sf, "foo", []int{4})

		sf = &sequenceFilter{
			fieldName: "foo",
			phrases:   []string{},
		}
		testFilterMatchForColumns(t, columns, sf, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8})

		sf = &sequenceFilter{
			fieldName: "foo",
			phrases:   []string{"", ""},
		}
		testFilterMatchForColumns(t, columns, sf, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8})

		sf = &sequenceFilter{
			fieldName: "non-existing-column",
			phrases:   []string{""},
		}
		testFilterMatchForColumns(t, columns, sf, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8})

		// mismatch
		sf = &sequenceFilter{
			fieldName: "foo",
			phrases:   []string{"bar"},
		}
		testFilterMatchForColumns(t, columns, sf, "foo", nil)

		sf = &sequenceFilter{
			fieldName: "foo",
			phrases:   []string{"65536", "-"},
		}
		testFilterMatchForColumns(t, columns, sf, "foo", nil)

		sf = &sequenceFilter{
			fieldName: "foo",
			phrases:   []string{"5678901", "1234"},
		}
		testFilterMatchForColumns(t, columns, sf, "foo", nil)

		sf = &sequenceFilter{
			fieldName: "foo",
			phrases:   []string{"12345678901234567890"},
		}
		testFilterMatchForColumns(t, columns, sf, "foo", nil)
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
					"1.0.127.6",
					"55.55.55.55",
					"66.66.66.66",
					"7.7.7.7",
				},
			},
		}

		// match
		sf := &sequenceFilter{
			fieldName: "foo",
			phrases:   []string{"127.0.0.1"},
		}
		testFilterMatchForColumns(t, columns, sf, "foo", []int{2, 4, 5, 7})

		sf = &sequenceFilter{
			fieldName: "foo",
			phrases:   []string{"127", "1"},
		}
		testFilterMatchForColumns(t, columns, sf, "foo", []int{2, 4, 5, 7})

		sf = &sequenceFilter{
			fieldName: "foo",
			phrases:   []string{"127.0.0"},
		}
		testFilterMatchForColumns(t, columns, sf, "foo", []int{2, 4, 5, 7})

		sf = &sequenceFilter{
			fieldName: "foo",
			phrases:   []string{"2.3", ".4"},
		}
		testFilterMatchForColumns(t, columns, sf, "foo", []int{0})

		sf = &sequenceFilter{
			fieldName: "foo",
			phrases:   []string{},
		}
		testFilterMatchForColumns(t, columns, sf, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11})

		sf = &sequenceFilter{
			fieldName: "foo",
			phrases:   []string{""},
		}
		testFilterMatchForColumns(t, columns, sf, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11})

		sf = &sequenceFilter{
			fieldName: "non-existing-column",
			phrases:   []string{""},
		}
		testFilterMatchForColumns(t, columns, sf, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11})

		// mismatch
		sf = &sequenceFilter{
			fieldName: "foo",
			phrases:   []string{"bar"},
		}
		testFilterMatchForColumns(t, columns, sf, "foo", nil)

		sf = &sequenceFilter{
			fieldName: "foo",
			phrases:   []string{"5"},
		}
		testFilterMatchForColumns(t, columns, sf, "foo", nil)

		sf = &sequenceFilter{
			fieldName: "foo",
			phrases:   []string{"127.", "1", "1", "345"},
		}
		testFilterMatchForColumns(t, columns, sf, "foo", nil)

		sf = &sequenceFilter{
			fieldName: "foo",
			phrases:   []string{"27.0"},
		}
		testFilterMatchForColumns(t, columns, sf, "foo", nil)

		sf = &sequenceFilter{
			fieldName: "foo",
			phrases:   []string{"255.255.255.255"},
		}
		testFilterMatchForColumns(t, columns, sf, "foo", nil)
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
		sf := &sequenceFilter{
			fieldName: "_msg",
			phrases:   []string{"2006-01-02T15:04:05.005Z"},
		}
		testFilterMatchForColumns(t, columns, sf, "_msg", []int{4})

		sf = &sequenceFilter{
			fieldName: "_msg",
			phrases:   []string{"2006-01", "04:05."},
		}
		testFilterMatchForColumns(t, columns, sf, "_msg", []int{0, 1, 2, 3, 4, 5, 6, 7, 8})

		sf = &sequenceFilter{
			fieldName: "_msg",
			phrases:   []string{"2006", "002Z"},
		}
		testFilterMatchForColumns(t, columns, sf, "_msg", []int{1})

		sf = &sequenceFilter{
			fieldName: "_msg",
			phrases:   []string{},
		}
		testFilterMatchForColumns(t, columns, sf, "_msg", []int{0, 1, 2, 3, 4, 5, 6, 7, 8})

		sf = &sequenceFilter{
			fieldName: "_msg",
			phrases:   []string{""},
		}
		testFilterMatchForColumns(t, columns, sf, "_msg", []int{0, 1, 2, 3, 4, 5, 6, 7, 8})

		sf = &sequenceFilter{
			fieldName: "non-existing-column",
			phrases:   []string{""},
		}
		testFilterMatchForColumns(t, columns, sf, "_msg", []int{0, 1, 2, 3, 4, 5, 6, 7, 8})

		// mimatch
		sf = &sequenceFilter{
			fieldName: "_msg",
			phrases:   []string{"bar"},
		}
		testFilterMatchForColumns(t, columns, sf, "_msg", nil)

		sf = &sequenceFilter{
			fieldName: "_msg",
			phrases:   []string{"002Z", "2006"},
		}
		testFilterMatchForColumns(t, columns, sf, "_msg", nil)

		sf = &sequenceFilter{
			fieldName: "_msg",
			phrases:   []string{"2006-04-02T15:04:05.005Z", "2023"},
		}
		testFilterMatchForColumns(t, columns, sf, "_msg", nil)

		sf = &sequenceFilter{
			fieldName: "_msg",
			phrases:   []string{"06"},
		}
		testFilterMatchForColumns(t, columns, sf, "_msg", nil)
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
					"foobarbaz",
					"foobar",
				},
			},
		}

		// match
		ef := &exactPrefixFilter{
			fieldName: "foo",
			prefix:    "foobar",
		}
		testFilterMatchForColumns(t, columns, ef, "foo", []int{1, 5, 6})

		ef = &exactPrefixFilter{
			fieldName: "foo",
			prefix:    "",
		}
		testFilterMatchForColumns(t, columns, ef, "foo", []int{0, 1, 2, 3, 4, 5, 6})

		// mismatch
		ef = &exactPrefixFilter{
			fieldName: "foo",
			prefix:    "baz",
		}
		testFilterMatchForColumns(t, columns, ef, "foo", nil)

		ef = &exactPrefixFilter{
			fieldName: "non-existing column",
			prefix:    "foobar",
		}
		testFilterMatchForColumns(t, columns, ef, "foo", nil)
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
					"aa fddf foobarbaz",
					"a afoobarbaz",
					"a foobar baz",
					"a kjlkjf dfff",
					"a ТЕСТЙЦУК НГКШ ",
					"a !!,23.(!1)",
				},
			},
		}

		// match
		ef := &exactPrefixFilter{
			fieldName: "foo",
			prefix:    "aa ",
		}
		testFilterMatchForColumns(t, columns, ef, "foo", []int{2, 4})

		ef = &exactPrefixFilter{
			fieldName: "non-existing-column",
			prefix:    "",
		}
		testFilterMatchForColumns(t, columns, ef, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9})

		// mismatch
		ef = &exactPrefixFilter{
			fieldName: "foo",
			prefix:    "aa b",
		}
		testFilterMatchForColumns(t, columns, ef, "foo", nil)

		ef = &exactPrefixFilter{
			fieldName: "foo",
			prefix:    "fobar",
		}
		testFilterMatchForColumns(t, columns, ef, "foo", nil)

		ef = &exactPrefixFilter{
			fieldName: "non-existing-column",
			prefix:    "aa",
		}
		testFilterMatchForColumns(t, columns, ef, "foo", nil)
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
		ef := &exactPrefixFilter{
			fieldName: "foo",
			prefix:    "12",
		}
		testFilterMatchForColumns(t, columns, ef, "foo", []int{0, 1, 5})

		ef = &exactPrefixFilter{
			fieldName: "foo",
			prefix:    "",
		}
		testFilterMatchForColumns(t, columns, ef, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10})

		// mismatch
		ef = &exactPrefixFilter{
			fieldName: "foo",
			prefix:    "bar",
		}
		testFilterMatchForColumns(t, columns, ef, "foo", nil)

		ef = &exactPrefixFilter{
			fieldName: "foo",
			prefix:    "999",
		}
		testFilterMatchForColumns(t, columns, ef, "foo", nil)

		ef = &exactPrefixFilter{
			fieldName: "foo",
			prefix:    "7",
		}
		testFilterMatchForColumns(t, columns, ef, "foo", nil)
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
					"1",
					"2",
					"3",
					"467",
					"5",
				},
			},
		}

		// match
		ef := &exactPrefixFilter{
			fieldName: "foo",
			prefix:    "12",
		}
		testFilterMatchForColumns(t, columns, ef, "foo", []int{0, 1, 5})

		ef = &exactPrefixFilter{
			fieldName: "foo",
			prefix:    "",
		}
		testFilterMatchForColumns(t, columns, ef, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10})

		// mismatch
		ef = &exactPrefixFilter{
			fieldName: "foo",
			prefix:    "bar",
		}
		testFilterMatchForColumns(t, columns, ef, "foo", nil)

		ef = &exactPrefixFilter{
			fieldName: "foo",
			prefix:    "999",
		}
		testFilterMatchForColumns(t, columns, ef, "foo", nil)

		ef = &exactPrefixFilter{
			fieldName: "foo",
			prefix:    "7",
		}
		testFilterMatchForColumns(t, columns, ef, "foo", nil)
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
					"1",
					"2",
					"3",
					"65536",
					"5",
				},
			},
		}

		// match
		ef := &exactPrefixFilter{
			fieldName: "foo",
			prefix:    "12",
		}
		testFilterMatchForColumns(t, columns, ef, "foo", []int{0, 1, 5})

		ef = &exactPrefixFilter{
			fieldName: "foo",
			prefix:    "",
		}
		testFilterMatchForColumns(t, columns, ef, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10})

		// mismatch
		ef = &exactPrefixFilter{
			fieldName: "foo",
			prefix:    "bar",
		}
		testFilterMatchForColumns(t, columns, ef, "foo", nil)

		ef = &exactPrefixFilter{
			fieldName: "foo",
			prefix:    "99999",
		}
		testFilterMatchForColumns(t, columns, ef, "foo", nil)

		ef = &exactPrefixFilter{
			fieldName: "foo",
			prefix:    "7",
		}
		testFilterMatchForColumns(t, columns, ef, "foo", nil)
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
					"1",
					"2",
					"3",
					"123456789012",
					"5",
				},
			},
		}

		// match
		ef := &exactPrefixFilter{
			fieldName: "foo",
			prefix:    "12",
		}
		testFilterMatchForColumns(t, columns, ef, "foo", []int{0, 1, 5})

		ef = &exactPrefixFilter{
			fieldName: "foo",
			prefix:    "",
		}
		testFilterMatchForColumns(t, columns, ef, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10})

		// mismatch
		ef = &exactPrefixFilter{
			fieldName: "foo",
			prefix:    "bar",
		}
		testFilterMatchForColumns(t, columns, ef, "foo", nil)

		ef = &exactPrefixFilter{
			fieldName: "foo",
			prefix:    "1234567890123",
		}
		testFilterMatchForColumns(t, columns, ef, "foo", nil)

		ef = &exactPrefixFilter{
			fieldName: "foo",
			prefix:    "7",
		}
		testFilterMatchForColumns(t, columns, ef, "foo", nil)
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
		ef := &exactPrefixFilter{
			fieldName: "foo",
			prefix:    "123",
		}
		testFilterMatchForColumns(t, columns, ef, "foo", []int{0, 4})

		ef = &exactPrefixFilter{
			fieldName: "foo",
			prefix:    "1234.567",
		}
		testFilterMatchForColumns(t, columns, ef, "foo", []int{4})

		ef = &exactPrefixFilter{
			fieldName: "foo",
			prefix:    "-65536",
		}
		testFilterMatchForColumns(t, columns, ef, "foo", []int{3})

		ef = &exactPrefixFilter{
			fieldName: "foo",
			prefix:    "",
		}
		testFilterMatchForColumns(t, columns, ef, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8})

		// mismatch
		ef = &exactPrefixFilter{
			fieldName: "foo",
			prefix:    "bar",
		}
		testFilterMatchForColumns(t, columns, ef, "foo", nil)

		ef = &exactPrefixFilter{
			fieldName: "foo",
			prefix:    "6511",
		}
		testFilterMatchForColumns(t, columns, ef, "foo", nil)
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
					"127.0.0.2",
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
		ef := &exactPrefixFilter{
			fieldName: "foo",
			prefix:    "127.0.",
		}
		testFilterMatchForColumns(t, columns, ef, "foo", []int{2, 4, 5, 7})

		ef = &exactPrefixFilter{
			fieldName: "foo",
			prefix:    "",
		}
		testFilterMatchForColumns(t, columns, ef, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11})

		// mismatch
		ef = &exactPrefixFilter{
			fieldName: "foo",
			prefix:    "bar",
		}
		testFilterMatchForColumns(t, columns, ef, "foo", nil)

		ef = &exactPrefixFilter{
			fieldName: "foo",
			prefix:    "255",
		}
		testFilterMatchForColumns(t, columns, ef, "foo", nil)
	})

	t.Run("timestamp-iso8601", func(t *testing.T) {
		columns := []column{
			{
				name: "_msg",
				values: []string{
					"2006-01-02T15:04:05.001Z",
					"2006-01-02T15:04:05.002Z",
					"2006-01-02T15:04:05.003Z",
					"2006-01-02T15:04:06.004Z",
					"2006-01-02T15:04:06.005Z",
					"2006-01-02T15:04:07.006Z",
					"2006-01-02T15:04:10.007Z",
					"2006-01-02T15:04:12.008Z",
					"2006-01-02T15:04:15.009Z",
				},
			},
		}

		// match
		ef := &exactPrefixFilter{
			fieldName: "_msg",
			prefix:    "2006-01-02T15:04:05",
		}
		testFilterMatchForColumns(t, columns, ef, "_msg", []int{0, 1, 2})

		ef = &exactPrefixFilter{
			fieldName: "_msg",
			prefix:    "",
		}
		testFilterMatchForColumns(t, columns, ef, "_msg", []int{0, 1, 2, 3, 4, 5, 6, 7, 8})

		// mimatch
		ef = &exactPrefixFilter{
			fieldName: "_msg",
			prefix:    "bar",
		}
		testFilterMatchForColumns(t, columns, ef, "_msg", nil)

		ef = &exactPrefixFilter{
			fieldName: "_msg",
			prefix:    "0",
		}
		testFilterMatchForColumns(t, columns, ef, "_msg", nil)
	})
}

func TestExactPrefixFilter(t *testing.T) {
	t.Run("single-row", func(t *testing.T) {
		columns := []column{
			{
				name: "foo",
				values: []string{
					"abc def",
				},
			},
		}

		// match
		ef := &exactPrefixFilter{
			fieldName: "foo",
			prefix:    "abc def",
		}
		testFilterMatchForColumns(t, columns, ef, "foo", []int{0})

		ef = &exactPrefixFilter{
			fieldName: "foo",
			prefix:    "abc d",
		}
		testFilterMatchForColumns(t, columns, ef, "foo", []int{0})

		ef = &exactPrefixFilter{
			fieldName: "foo",
			prefix:    "",
		}
		testFilterMatchForColumns(t, columns, ef, "foo", []int{0})

		ef = &exactPrefixFilter{
			fieldName: "non-existing-column",
			prefix:    "",
		}
		testFilterMatchForColumns(t, columns, ef, "foo", []int{0})

		// mismatch
		ef = &exactPrefixFilter{
			fieldName: "foo",
			prefix:    "xabc",
		}
		testFilterMatchForColumns(t, columns, ef, "foo", nil)

		ef = &exactPrefixFilter{
			fieldName: "non-existing column",
			prefix:    "abc",
		}
		testFilterMatchForColumns(t, columns, ef, "foo", nil)
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
		}

		// match
		ef := &exactPrefixFilter{
			fieldName: "foo",
			prefix:    "abc def",
		}
		testFilterMatchForColumns(t, columns, ef, "foo", []int{0, 1, 2})

		ef = &exactPrefixFilter{
			fieldName: "foo",
			prefix:    "ab",
		}
		testFilterMatchForColumns(t, columns, ef, "foo", []int{0, 1, 2})

		ef = &exactPrefixFilter{
			fieldName: "foo",
			prefix:    "",
		}
		testFilterMatchForColumns(t, columns, ef, "foo", []int{0, 1, 2})

		ef = &exactPrefixFilter{
			fieldName: "non-existing-column",
			prefix:    "",
		}
		testFilterMatchForColumns(t, columns, ef, "foo", []int{0, 1, 2})

		// mismatch
		ef = &exactPrefixFilter{
			fieldName: "foo",
			prefix:    "foobar",
		}
		testFilterMatchForColumns(t, columns, ef, "foo", nil)

		ef = &exactPrefixFilter{
			fieldName: "non-existing column",
			prefix:    "x",
		}
		testFilterMatchForColumns(t, columns, ef, "foo", nil)
	})

}

func TestExactFilter(t *testing.T) {
	t.Run("single-row", func(t *testing.T) {
		columns := []column{
			{
				name: "foo",
				values: []string{
					"abc def",
				},
			},
		}

		// match
		ef := &exactFilter{
			fieldName: "foo",
			value:     "abc def",
		}
		testFilterMatchForColumns(t, columns, ef, "foo", []int{0})

		ef = &exactFilter{
			fieldName: "non-existing-column",
			value:     "",
		}
		testFilterMatchForColumns(t, columns, ef, "foo", []int{0})

		// mismatch
		ef = &exactFilter{
			fieldName: "foo",
			value:     "abc",
		}
		testFilterMatchForColumns(t, columns, ef, "foo", nil)

		ef = &exactFilter{
			fieldName: "non-existing column",
			value:     "abc",
		}
		testFilterMatchForColumns(t, columns, ef, "foo", nil)
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
		}

		// match
		ef := &exactFilter{
			fieldName: "foo",
			value:     "abc def",
		}
		testFilterMatchForColumns(t, columns, ef, "foo", []int{0, 1, 2})

		ef = &exactFilter{
			fieldName: "non-existing-column",
			value:     "",
		}
		testFilterMatchForColumns(t, columns, ef, "foo", []int{0, 1, 2})

		// mismatch
		ef = &exactFilter{
			fieldName: "foo",
			value:     "foobar",
		}
		testFilterMatchForColumns(t, columns, ef, "foo", nil)

		ef = &exactFilter{
			fieldName: "foo",
			value:     "",
		}
		testFilterMatchForColumns(t, columns, ef, "foo", nil)

		ef = &exactFilter{
			fieldName: "non-existing column",
			value:     "x",
		}
		testFilterMatchForColumns(t, columns, ef, "foo", nil)
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
		ef := &exactFilter{
			fieldName: "foo",
			value:     "foobar",
		}
		testFilterMatchForColumns(t, columns, ef, "foo", []int{1, 6})

		ef = &exactFilter{
			fieldName: "foo",
			value:     "",
		}
		testFilterMatchForColumns(t, columns, ef, "foo", []int{0})

		// mismatch
		ef = &exactFilter{
			fieldName: "foo",
			value:     "baz",
		}
		testFilterMatchForColumns(t, columns, ef, "foo", nil)

		ef = &exactFilter{
			fieldName: "non-existing column",
			value:     "foobar",
		}
		testFilterMatchForColumns(t, columns, ef, "foo", nil)
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
					"a foobar baz",
					"a kjlkjf dfff",
					"a ТЕСТЙЦУК НГКШ ",
					"a !!,23.(!1)",
				},
			},
		}

		// match
		ef := &exactFilter{
			fieldName: "foo",
			value:     "aa abc a",
		}
		testFilterMatchForColumns(t, columns, ef, "foo", []int{2})

		ef = &exactFilter{
			fieldName: "non-existing-column",
			value:     "",
		}
		testFilterMatchForColumns(t, columns, ef, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9})

		// mismatch
		ef = &exactFilter{
			fieldName: "foo",
			value:     "aa a",
		}
		testFilterMatchForColumns(t, columns, ef, "foo", nil)

		ef = &exactFilter{
			fieldName: "foo",
			value:     "fooaaazz a",
		}
		testFilterMatchForColumns(t, columns, ef, "foo", nil)

		ef = &exactFilter{
			fieldName: "foo",
			value:     "",
		}
		testFilterMatchForColumns(t, columns, ef, "foo", nil)
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
		ef := &exactFilter{
			fieldName: "foo",
			value:     "12",
		}
		testFilterMatchForColumns(t, columns, ef, "foo", []int{1, 5})

		ef = &exactFilter{
			fieldName: "non-existing-column",
			value:     "",
		}
		testFilterMatchForColumns(t, columns, ef, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10})

		// mismatch
		ef = &exactFilter{
			fieldName: "foo",
			value:     "bar",
		}
		testFilterMatchForColumns(t, columns, ef, "foo", nil)

		ef = &exactFilter{
			fieldName: "foo",
			value:     "",
		}
		testFilterMatchForColumns(t, columns, ef, "foo", nil)

		ef = &exactFilter{
			fieldName: "foo",
			value:     "33",
		}
		testFilterMatchForColumns(t, columns, ef, "foo", nil)
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
		}

		// match
		ef := &exactFilter{
			fieldName: "foo",
			value:     "12",
		}
		testFilterMatchForColumns(t, columns, ef, "foo", []int{1, 5})

		ef = &exactFilter{
			fieldName: "non-existing-column",
			value:     "",
		}
		testFilterMatchForColumns(t, columns, ef, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10})

		// mismatch
		ef = &exactFilter{
			fieldName: "foo",
			value:     "bar",
		}
		testFilterMatchForColumns(t, columns, ef, "foo", nil)

		ef = &exactFilter{
			fieldName: "foo",
			value:     "",
		}
		testFilterMatchForColumns(t, columns, ef, "foo", nil)

		ef = &exactFilter{
			fieldName: "foo",
			value:     "33",
		}
		testFilterMatchForColumns(t, columns, ef, "foo", nil)
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
		}

		// match
		ef := &exactFilter{
			fieldName: "foo",
			value:     "12",
		}
		testFilterMatchForColumns(t, columns, ef, "foo", []int{1, 5})

		ef = &exactFilter{
			fieldName: "non-existing-column",
			value:     "",
		}
		testFilterMatchForColumns(t, columns, ef, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10})

		// mismatch
		ef = &exactFilter{
			fieldName: "foo",
			value:     "bar",
		}
		testFilterMatchForColumns(t, columns, ef, "foo", nil)

		ef = &exactFilter{
			fieldName: "foo",
			value:     "",
		}
		testFilterMatchForColumns(t, columns, ef, "foo", nil)

		ef = &exactFilter{
			fieldName: "foo",
			value:     "33",
		}
		testFilterMatchForColumns(t, columns, ef, "foo", nil)
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
		}

		// match
		ef := &exactFilter{
			fieldName: "foo",
			value:     "12",
		}
		testFilterMatchForColumns(t, columns, ef, "foo", []int{1, 5})

		ef = &exactFilter{
			fieldName: "non-existing-column",
			value:     "",
		}
		testFilterMatchForColumns(t, columns, ef, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10})

		// mismatch
		ef = &exactFilter{
			fieldName: "foo",
			value:     "bar",
		}
		testFilterMatchForColumns(t, columns, ef, "foo", nil)

		ef = &exactFilter{
			fieldName: "foo",
			value:     "",
		}
		testFilterMatchForColumns(t, columns, ef, "foo", nil)

		ef = &exactFilter{
			fieldName: "foo",
			value:     "33",
		}
		testFilterMatchForColumns(t, columns, ef, "foo", nil)
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
		ef := &exactFilter{
			fieldName: "foo",
			value:     "1234",
		}
		testFilterMatchForColumns(t, columns, ef, "foo", []int{0})

		ef = &exactFilter{
			fieldName: "foo",
			value:     "1234.5678901",
		}
		testFilterMatchForColumns(t, columns, ef, "foo", []int{4})

		ef = &exactFilter{
			fieldName: "foo",
			value:     "-65536",
		}
		testFilterMatchForColumns(t, columns, ef, "foo", []int{3})

		ef = &exactFilter{
			fieldName: "non-existing-column",
			value:     "",
		}
		testFilterMatchForColumns(t, columns, ef, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8})

		// mismatch
		ef = &exactFilter{
			fieldName: "foo",
			value:     "bar",
		}
		testFilterMatchForColumns(t, columns, ef, "foo", nil)

		ef = &exactFilter{
			fieldName: "foo",
			value:     "65536",
		}
		testFilterMatchForColumns(t, columns, ef, "foo", nil)

		ef = &exactFilter{
			fieldName: "foo",
			value:     "",
		}
		testFilterMatchForColumns(t, columns, ef, "foo", nil)

		ef = &exactFilter{
			fieldName: "foo",
			value:     "123",
		}
		testFilterMatchForColumns(t, columns, ef, "foo", nil)

		ef = &exactFilter{
			fieldName: "foo",
			value:     "12345678901234567890",
		}
		testFilterMatchForColumns(t, columns, ef, "foo", nil)
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
		ef := &exactFilter{
			fieldName: "foo",
			value:     "127.0.0.1",
		}
		testFilterMatchForColumns(t, columns, ef, "foo", []int{2, 4, 5, 7})

		ef = &exactFilter{
			fieldName: "non-existing-column",
			value:     "",
		}
		testFilterMatchForColumns(t, columns, ef, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11})

		// mismatch
		ef = &exactFilter{
			fieldName: "foo",
			value:     "bar",
		}
		testFilterMatchForColumns(t, columns, ef, "foo", nil)

		ef = &exactFilter{
			fieldName: "foo",
			value:     "",
		}
		testFilterMatchForColumns(t, columns, ef, "foo", nil)

		ef = &exactFilter{
			fieldName: "foo",
			value:     "127.0",
		}
		testFilterMatchForColumns(t, columns, ef, "foo", nil)

		ef = &exactFilter{
			fieldName: "foo",
			value:     "255.255.255.255",
		}
		testFilterMatchForColumns(t, columns, ef, "foo", nil)
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
		ef := &exactFilter{
			fieldName: "_msg",
			value:     "2006-01-02T15:04:05.005Z",
		}
		testFilterMatchForColumns(t, columns, ef, "_msg", []int{4})

		ef = &exactFilter{
			fieldName: "non-existing-column",
			value:     "",
		}
		testFilterMatchForColumns(t, columns, ef, "_msg", []int{0, 1, 2, 3, 4, 5, 6, 7, 8})

		// mimatch
		ef = &exactFilter{
			fieldName: "_msg",
			value:     "bar",
		}
		testFilterMatchForColumns(t, columns, ef, "_msg", nil)

		ef = &exactFilter{
			fieldName: "_msg",
			value:     "",
		}
		testFilterMatchForColumns(t, columns, ef, "_msg", nil)

		ef = &exactFilter{
			fieldName: "_msg",
			value:     "2006-03-02T15:04:05.005Z",
		}
		testFilterMatchForColumns(t, columns, ef, "_msg", nil)
	})
}

func TestInFilter(t *testing.T) {
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
		af := &inFilter{
			fieldName: "foo",
			values:    []string{"abc def", "abc", "foobar"},
		}
		testFilterMatchForColumns(t, columns, af, "foo", []int{0})

		af = &inFilter{
			fieldName: "other column",
			values:    []string{"asdfdsf", ""},
		}
		testFilterMatchForColumns(t, columns, af, "foo", []int{0})

		af = &inFilter{
			fieldName: "non-existing-column",
			values:    []string{"", "foo"},
		}
		testFilterMatchForColumns(t, columns, af, "foo", []int{0})

		// mismatch
		af = &inFilter{
			fieldName: "foo",
			values:    []string{"abc", "def"},
		}
		testFilterMatchForColumns(t, columns, af, "foo", nil)

		af = &inFilter{
			fieldName: "foo",
			values:    []string{},
		}
		testFilterMatchForColumns(t, columns, af, "foo", nil)

		af = &inFilter{
			fieldName: "foo",
			values:    []string{"", "abc"},
		}
		testFilterMatchForColumns(t, columns, af, "foo", nil)

		af = &inFilter{
			fieldName: "other column",
			values:    []string{"sd"},
		}
		testFilterMatchForColumns(t, columns, af, "foo", nil)

		af = &inFilter{
			fieldName: "non-existing column",
			values:    []string{"abc", "def"},
		}
		testFilterMatchForColumns(t, columns, af, "foo", nil)
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
		}

		// match
		af := &inFilter{
			fieldName: "foo",
			values:    []string{"aaaa", "abc def", "foobar"},
		}
		testFilterMatchForColumns(t, columns, af, "foo", []int{0, 1, 2})

		af = &inFilter{
			fieldName: "non-existing-column",
			values:    []string{"", "abc"},
		}
		testFilterMatchForColumns(t, columns, af, "foo", []int{0, 1, 2})

		// mismatch
		af = &inFilter{
			fieldName: "foo",
			values:    []string{"abc def ", "foobar"},
		}
		testFilterMatchForColumns(t, columns, af, "foo", nil)

		af = &inFilter{
			fieldName: "foo",
			values:    []string{},
		}
		testFilterMatchForColumns(t, columns, af, "foo", nil)

		af = &inFilter{
			fieldName: "foo",
			values:    []string{""},
		}
		testFilterMatchForColumns(t, columns, af, "foo", nil)

		af = &inFilter{
			fieldName: "non-existing column",
			values:    []string{"x"},
		}
		testFilterMatchForColumns(t, columns, af, "foo", nil)
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
		af := &inFilter{
			fieldName: "foo",
			values:    []string{"foobar", "aaaa", "abc", "baz"},
		}
		testFilterMatchForColumns(t, columns, af, "foo", []int{1, 2, 6})

		af = &inFilter{
			fieldName: "foo",
			values:    []string{"bbbb", "", "aaaa"},
		}
		testFilterMatchForColumns(t, columns, af, "foo", []int{0})

		af = &inFilter{
			fieldName: "non-existing-column",
			values:    []string{""},
		}
		testFilterMatchForColumns(t, columns, af, "foo", []int{0, 1, 2, 3, 4, 5, 6})

		// mismatch
		af = &inFilter{
			fieldName: "foo",
			values:    []string{"bar", "aaaa"},
		}
		testFilterMatchForColumns(t, columns, af, "foo", nil)

		af = &inFilter{
			fieldName: "foo",
			values:    []string{},
		}
		testFilterMatchForColumns(t, columns, af, "foo", nil)

		af = &inFilter{
			fieldName: "non-existing column",
			values:    []string{"foobar", "aaaa"},
		}
		testFilterMatchForColumns(t, columns, af, "foo", nil)
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
		af := &inFilter{
			fieldName: "foo",
			values:    []string{"a foobar", "aa abc a"},
		}
		testFilterMatchForColumns(t, columns, af, "foo", []int{1, 2, 6})

		af = &inFilter{
			fieldName: "non-existing-column",
			values:    []string{""},
		}
		testFilterMatchForColumns(t, columns, af, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9})

		// mismatch
		af = &inFilter{
			fieldName: "foo",
			values:    []string{"aa a"},
		}
		testFilterMatchForColumns(t, columns, af, "foo", nil)

		af = &inFilter{
			fieldName: "foo",
			values:    []string{},
		}
		testFilterMatchForColumns(t, columns, af, "foo", nil)

		af = &inFilter{
			fieldName: "foo",
			values:    []string{""},
		}
		testFilterMatchForColumns(t, columns, af, "foo", nil)
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
		af := &inFilter{
			fieldName: "foo",
			values:    []string{"12", "32"},
		}
		testFilterMatchForColumns(t, columns, af, "foo", []int{1, 2, 5})

		af = &inFilter{
			fieldName: "foo",
			values:    []string{"0"},
		}
		testFilterMatchForColumns(t, columns, af, "foo", []int{3, 4})

		af = &inFilter{
			fieldName: "non-existing-column",
			values:    []string{""},
		}
		testFilterMatchForColumns(t, columns, af, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10})

		// mismatch
		af = &inFilter{
			fieldName: "foo",
			values:    []string{"bar"},
		}
		testFilterMatchForColumns(t, columns, af, "foo", nil)

		af = &inFilter{
			fieldName: "foo",
			values:    []string{},
		}
		testFilterMatchForColumns(t, columns, af, "foo", nil)

		af = &inFilter{
			fieldName: "foo",
			values:    []string{"33"},
		}
		testFilterMatchForColumns(t, columns, af, "foo", nil)

		af = &inFilter{
			fieldName: "foo",
			values:    []string{"1234"},
		}
		testFilterMatchForColumns(t, columns, af, "foo", nil)
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
		}

		// match
		af := &inFilter{
			fieldName: "foo",
			values:    []string{"12", "32"},
		}
		testFilterMatchForColumns(t, columns, af, "foo", []int{1, 2, 5})

		af = &inFilter{
			fieldName: "foo",
			values:    []string{"0"},
		}
		testFilterMatchForColumns(t, columns, af, "foo", []int{3, 4})

		af = &inFilter{
			fieldName: "non-existing-column",
			values:    []string{""},
		}
		testFilterMatchForColumns(t, columns, af, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10})

		// mismatch
		af = &inFilter{
			fieldName: "foo",
			values:    []string{"bar"},
		}
		testFilterMatchForColumns(t, columns, af, "foo", nil)

		af = &inFilter{
			fieldName: "foo",
			values:    []string{},
		}
		testFilterMatchForColumns(t, columns, af, "foo", nil)

		af = &inFilter{
			fieldName: "foo",
			values:    []string{"33"},
		}
		testFilterMatchForColumns(t, columns, af, "foo", nil)

		af = &inFilter{
			fieldName: "foo",
			values:    []string{"123456"},
		}
		testFilterMatchForColumns(t, columns, af, "foo", nil)
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
		}

		// match
		af := &inFilter{
			fieldName: "foo",
			values:    []string{"12", "32"},
		}
		testFilterMatchForColumns(t, columns, af, "foo", []int{1, 2, 5})

		af = &inFilter{
			fieldName: "foo",
			values:    []string{"0"},
		}
		testFilterMatchForColumns(t, columns, af, "foo", []int{3, 4})

		af = &inFilter{
			fieldName: "non-existing-column",
			values:    []string{""},
		}
		testFilterMatchForColumns(t, columns, af, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10})

		// mismatch
		af = &inFilter{
			fieldName: "foo",
			values:    []string{"bar"},
		}
		testFilterMatchForColumns(t, columns, af, "foo", nil)

		af = &inFilter{
			fieldName: "foo",
			values:    []string{},
		}
		testFilterMatchForColumns(t, columns, af, "foo", nil)

		af = &inFilter{
			fieldName: "foo",
			values:    []string{"33"},
		}
		testFilterMatchForColumns(t, columns, af, "foo", nil)

		af = &inFilter{
			fieldName: "foo",
			values:    []string{"12345678901"},
		}
		testFilterMatchForColumns(t, columns, af, "foo", nil)
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
		}

		// match
		af := &inFilter{
			fieldName: "foo",
			values:    []string{"12", "32"},
		}
		testFilterMatchForColumns(t, columns, af, "foo", []int{1, 2, 5})

		af = &inFilter{
			fieldName: "foo",
			values:    []string{"0"},
		}
		testFilterMatchForColumns(t, columns, af, "foo", []int{3, 4})

		af = &inFilter{
			fieldName: "non-existing-column",
			values:    []string{""},
		}
		testFilterMatchForColumns(t, columns, af, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10})

		// mismatch
		af = &inFilter{
			fieldName: "foo",
			values:    []string{"bar"},
		}
		testFilterMatchForColumns(t, columns, af, "foo", nil)

		af = &inFilter{
			fieldName: "foo",
			values:    []string{},
		}
		testFilterMatchForColumns(t, columns, af, "foo", nil)

		af = &inFilter{
			fieldName: "foo",
			values:    []string{"33"},
		}
		testFilterMatchForColumns(t, columns, af, "foo", nil)
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
		af := &inFilter{
			fieldName: "foo",
			values:    []string{"1234", "1", "foobar", "123211"},
		}
		testFilterMatchForColumns(t, columns, af, "foo", []int{0, 5})

		af = &inFilter{
			fieldName: "foo",
			values:    []string{"1234.5678901"},
		}
		testFilterMatchForColumns(t, columns, af, "foo", []int{4})

		af = &inFilter{
			fieldName: "foo",
			values:    []string{"-65536"},
		}
		testFilterMatchForColumns(t, columns, af, "foo", []int{3})

		af = &inFilter{
			fieldName: "non-existing-column",
			values:    []string{""},
		}
		testFilterMatchForColumns(t, columns, af, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8})

		// mismatch
		af = &inFilter{
			fieldName: "foo",
			values:    []string{"bar"},
		}
		testFilterMatchForColumns(t, columns, af, "foo", nil)

		af = &inFilter{
			fieldName: "foo",
			values:    []string{"65536"},
		}
		testFilterMatchForColumns(t, columns, af, "foo", nil)

		af = &inFilter{
			fieldName: "foo",
			values:    []string{},
		}
		testFilterMatchForColumns(t, columns, af, "foo", nil)

		af = &inFilter{
			fieldName: "foo",
			values:    []string{""},
		}
		testFilterMatchForColumns(t, columns, af, "foo", nil)

		af = &inFilter{
			fieldName: "foo",
			values:    []string{"123"},
		}
		testFilterMatchForColumns(t, columns, af, "foo", nil)

		af = &inFilter{
			fieldName: "foo",
			values:    []string{"12345678901234567890"},
		}
		testFilterMatchForColumns(t, columns, af, "foo", nil)
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
		af := &inFilter{
			fieldName: "foo",
			values:    []string{"127.0.0.1", "24.54.1.2", "127.0.4.2"},
		}
		testFilterMatchForColumns(t, columns, af, "foo", []int{2, 4, 5, 6, 7})

		af = &inFilter{
			fieldName: "non-existing-column",
			values:    []string{""},
		}
		testFilterMatchForColumns(t, columns, af, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11})

		// mismatch
		af = &inFilter{
			fieldName: "foo",
			values:    []string{"bar"},
		}
		testFilterMatchForColumns(t, columns, af, "foo", nil)

		af = &inFilter{
			fieldName: "foo",
			values:    []string{},
		}
		testFilterMatchForColumns(t, columns, af, "foo", nil)

		af = &inFilter{
			fieldName: "foo",
			values:    []string{""},
		}
		testFilterMatchForColumns(t, columns, af, "foo", nil)

		af = &inFilter{
			fieldName: "foo",
			values:    []string{"5"},
		}
		testFilterMatchForColumns(t, columns, af, "foo", nil)

		af = &inFilter{
			fieldName: "foo",
			values:    []string{"255.255.255.255"},
		}
		testFilterMatchForColumns(t, columns, af, "foo", nil)
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
		af := &inFilter{
			fieldName: "_msg",
			values:    []string{"2006-01-02T15:04:05.005Z", "foobar"},
		}
		testFilterMatchForColumns(t, columns, af, "_msg", []int{4})

		af = &inFilter{
			fieldName: "non-existing-column",
			values:    []string{""},
		}
		testFilterMatchForColumns(t, columns, af, "_msg", []int{0, 1, 2, 3, 4, 5, 6, 7, 8})

		// mimatch
		af = &inFilter{
			fieldName: "_msg",
			values:    []string{"bar"},
		}
		testFilterMatchForColumns(t, columns, af, "_msg", nil)

		af = &inFilter{
			fieldName: "_msg",
			values:    []string{},
		}
		testFilterMatchForColumns(t, columns, af, "_msg", nil)

		af = &inFilter{
			fieldName: "_msg",
			values:    []string{""},
		}
		testFilterMatchForColumns(t, columns, af, "_msg", nil)

		af = &inFilter{
			fieldName: "_msg",
			values:    []string{"2006-04-02T15:04:05.005Z"},
		}
		testFilterMatchForColumns(t, columns, af, "_msg", nil)
	})
}

func TestRegexpFilter(t *testing.T) {
	t.Run("const-column", func(t *testing.T) {
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
		rf := &regexpFilter{
			fieldName: "foo",
			re:        regexp.MustCompile("0.0"),
		}
		testFilterMatchForColumns(t, columns, rf, "foo", []int{0, 1, 2})

		rf = &regexpFilter{
			fieldName: "foo",
			re:        regexp.MustCompile(`^127\.0\.0\.1$`),
		}
		testFilterMatchForColumns(t, columns, rf, "foo", []int{0, 1, 2})

		rf = &regexpFilter{
			fieldName: "non-existing-column",
			re:        regexp.MustCompile("foo.+bar|"),
		}
		testFilterMatchForColumns(t, columns, rf, "foo", []int{0, 1, 2})

		// mismatch
		rf = &regexpFilter{
			fieldName: "foo",
			re:        regexp.MustCompile("foo.+bar"),
		}
		testFilterMatchForColumns(t, columns, rf, "foo", nil)

		rf = &regexpFilter{
			fieldName: "non-existing-column",
			re:        regexp.MustCompile("foo.+bar"),
		}
		testFilterMatchForColumns(t, columns, rf, "foo", nil)
	})

	t.Run("dict", func(t *testing.T) {
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
		rf := &regexpFilter{
			fieldName: "foo",
			re:        regexp.MustCompile("foo|bar|^$"),
		}
		testFilterMatchForColumns(t, columns, rf, "foo", []int{0, 5, 6})

		rf = &regexpFilter{
			fieldName: "foo",
			re:        regexp.MustCompile("27.0"),
		}
		testFilterMatchForColumns(t, columns, rf, "foo", []int{1, 5, 6, 7})

		// mismatch
		rf = &regexpFilter{
			fieldName: "foo",
			re:        regexp.MustCompile("bar.+foo"),
		}
		testFilterMatchForColumns(t, columns, rf, "foo", nil)
	})

	t.Run("strings", func(t *testing.T) {
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
		rf := &regexpFilter{
			fieldName: "foo",
			re:        regexp.MustCompile("(?i)foo|йцу"),
		}
		testFilterMatchForColumns(t, columns, rf, "foo", []int{0, 6, 8})

		// mismatch
		rf = &regexpFilter{
			fieldName: "foo",
			re:        regexp.MustCompile("qwe.+rty|^$"),
		}
		testFilterMatchForColumns(t, columns, rf, "foo", nil)
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
		rf := &regexpFilter{
			fieldName: "foo",
			re:        regexp.MustCompile("[32][23]?"),
		}
		testFilterMatchForColumns(t, columns, rf, "foo", []int{0, 1, 2, 5, 7, 8})

		// mismatch
		rf = &regexpFilter{
			fieldName: "foo",
			re:        regexp.MustCompile("foo|bar"),
		}
		testFilterMatchForColumns(t, columns, rf, "foo", nil)
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
		rf := &regexpFilter{
			fieldName: "foo",
			re:        regexp.MustCompile("[32][23]?"),
		}
		testFilterMatchForColumns(t, columns, rf, "foo", []int{0, 1, 2, 5, 7, 8})

		// mismatch
		rf = &regexpFilter{
			fieldName: "foo",
			re:        regexp.MustCompile("foo|bar"),
		}
		testFilterMatchForColumns(t, columns, rf, "foo", nil)
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
		rf := &regexpFilter{
			fieldName: "foo",
			re:        regexp.MustCompile("[32][23]?"),
		}
		testFilterMatchForColumns(t, columns, rf, "foo", []int{0, 1, 2, 5, 7, 8})

		// mismatch
		rf = &regexpFilter{
			fieldName: "foo",
			re:        regexp.MustCompile("foo|bar"),
		}
		testFilterMatchForColumns(t, columns, rf, "foo", nil)
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
		rf := &regexpFilter{
			fieldName: "foo",
			re:        regexp.MustCompile("[32][23]?"),
		}
		testFilterMatchForColumns(t, columns, rf, "foo", []int{0, 1, 2, 5, 7, 8})

		// mismatch
		rf = &regexpFilter{
			fieldName: "foo",
			re:        regexp.MustCompile("foo|bar"),
		}
		testFilterMatchForColumns(t, columns, rf, "foo", nil)
	})

	t.Run("float64", func(t *testing.T) {
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
		rf := &regexpFilter{
			fieldName: "foo",
			re:        regexp.MustCompile("[32][23]?"),
		}
		testFilterMatchForColumns(t, columns, rf, "foo", []int{0, 1, 2, 5, 6, 7, 8})

		// mismatch
		rf = &regexpFilter{
			fieldName: "foo",
			re:        regexp.MustCompile("foo|bar"),
		}
		testFilterMatchForColumns(t, columns, rf, "foo", nil)
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
					"55.55.12.55",
					"66.66.66.66",
					"7.7.7.7",
				},
			},
		}

		// match
		rf := &regexpFilter{
			fieldName: "foo",
			re:        regexp.MustCompile("127.0.[40].(1|2)"),
		}
		testFilterMatchForColumns(t, columns, rf, "foo", []int{2, 4, 5, 6, 7})

		// mismatch
		rf = &regexpFilter{
			fieldName: "foo",
			re:        regexp.MustCompile("foo|bar|834"),
		}
		testFilterMatchForColumns(t, columns, rf, "foo", nil)
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
		rf := &regexpFilter{
			fieldName: "_msg",
			re:        regexp.MustCompile("2006-[0-9]{2}-.+?(2|5)Z"),
		}
		testFilterMatchForColumns(t, columns, rf, "_msg", []int{1, 4})

		// mismatch
		rf = &regexpFilter{
			fieldName: "_msg",
			re:        regexp.MustCompile("^01|04$"),
		}
		testFilterMatchForColumns(t, columns, rf, "_msg", nil)
	})
}

func TestStringRangeFilter(t *testing.T) {
	t.Run("const-column", func(t *testing.T) {
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
		rf := &stringRangeFilter{
			fieldName: "foo",
			minValue:  "127.0.0.1",
			maxValue:  "255.",
		}
		testFilterMatchForColumns(t, columns, rf, "foo", []int{0, 1, 2})

		rf = &stringRangeFilter{
			fieldName: "foo",
			minValue:  "127.0.0.1",
			maxValue:  "127.0.0.1",
		}
		testFilterMatchForColumns(t, columns, rf, "foo", []int{0, 1, 2})

		// mismatch
		rf = &stringRangeFilter{
			fieldName: "foo",
			minValue:  "",
			maxValue:  "127.0.0.0",
		}
		testFilterMatchForColumns(t, columns, rf, "foo", nil)

		rf = &stringRangeFilter{
			fieldName: "non-existing-column",
			minValue:  "1",
			maxValue:  "2",
		}
		testFilterMatchForColumns(t, columns, rf, "foo", nil)

		rf = &stringRangeFilter{
			fieldName: "foo",
			minValue:  "127.0.0.2",
			maxValue:  "",
		}
		testFilterMatchForColumns(t, columns, rf, "foo", nil)
	})

	t.Run("dict", func(t *testing.T) {
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
		rf := &stringRangeFilter{
			fieldName: "foo",
			minValue:  "127.0.0.0",
			maxValue:  "128.0.0.0",
		}
		testFilterMatchForColumns(t, columns, rf, "foo", []int{1, 3, 6, 7})

		rf = &stringRangeFilter{
			fieldName: "foo",
			minValue:  "127",
			maxValue:  "127.0.0.1",
		}
		testFilterMatchForColumns(t, columns, rf, "foo", []int{1, 7})

		// mismatch
		rf = &stringRangeFilter{
			fieldName: "foo",
			minValue:  "0",
			maxValue:  "10",
		}
		testFilterMatchForColumns(t, columns, rf, "foo", nil)

		rf = &stringRangeFilter{
			fieldName: "foo",
			minValue:  "127.0.0.2",
			maxValue:  "127.127.0.0",
		}
		testFilterMatchForColumns(t, columns, rf, "foo", nil)

		rf = &stringRangeFilter{
			fieldName: "foo",
			minValue:  "128.0.0.0",
			maxValue:  "127.0.0.0",
		}
		testFilterMatchForColumns(t, columns, rf, "foo", nil)
	})

	t.Run("strings", func(t *testing.T) {
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
		rf := &stringRangeFilter{
			fieldName: "foo",
			minValue:  "127.0.0.1",
			maxValue:  "255.255.255.255",
		}
		testFilterMatchForColumns(t, columns, rf, "foo", []int{2, 3, 4})

		// mismatch
		rf = &stringRangeFilter{
			fieldName: "foo",
			minValue:  "0",
			maxValue:  "10",
		}
		testFilterMatchForColumns(t, columns, rf, "foo", nil)

		rf = &stringRangeFilter{
			fieldName: "foo",
			minValue:  "255.255.255.255",
			maxValue:  "127.0.0.1",
		}
		testFilterMatchForColumns(t, columns, rf, "foo", nil)
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
		rf := &stringRangeFilter{
			fieldName: "foo",
			minValue:  "33",
			maxValue:  "5",
		}
		testFilterMatchForColumns(t, columns, rf, "foo", []int{9, 10})

		// mismatch
		rf = &stringRangeFilter{
			fieldName: "foo",
			minValue:  "a",
			maxValue:  "b",
		}
		testFilterMatchForColumns(t, columns, rf, "foo", nil)

		rf = &stringRangeFilter{
			fieldName: "foo",
			minValue:  "100",
			maxValue:  "101",
		}
		testFilterMatchForColumns(t, columns, rf, "foo", nil)

		rf = &stringRangeFilter{
			fieldName: "foo",
			minValue:  "5",
			maxValue:  "33",
		}
		testFilterMatchForColumns(t, columns, rf, "foo", nil)
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
		rf := &stringRangeFilter{
			fieldName: "foo",
			minValue:  "33",
			maxValue:  "5",
		}
		testFilterMatchForColumns(t, columns, rf, "foo", []int{9, 10})

		// mismatch
		rf = &stringRangeFilter{
			fieldName: "foo",
			minValue:  "a",
			maxValue:  "b",
		}
		testFilterMatchForColumns(t, columns, rf, "foo", nil)

		rf = &stringRangeFilter{
			fieldName: "foo",
			minValue:  "100",
			maxValue:  "101",
		}
		testFilterMatchForColumns(t, columns, rf, "foo", nil)

		rf = &stringRangeFilter{
			fieldName: "foo",
			minValue:  "5",
			maxValue:  "33",
		}
		testFilterMatchForColumns(t, columns, rf, "foo", nil)
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
		rf := &stringRangeFilter{
			fieldName: "foo",
			minValue:  "33",
			maxValue:  "5",
		}
		testFilterMatchForColumns(t, columns, rf, "foo", []int{9, 10})

		// mismatch
		rf = &stringRangeFilter{
			fieldName: "foo",
			minValue:  "a",
			maxValue:  "b",
		}
		testFilterMatchForColumns(t, columns, rf, "foo", nil)

		rf = &stringRangeFilter{
			fieldName: "foo",
			minValue:  "100",
			maxValue:  "101",
		}
		testFilterMatchForColumns(t, columns, rf, "foo", nil)

		rf = &stringRangeFilter{
			fieldName: "foo",
			minValue:  "5",
			maxValue:  "33",
		}
		testFilterMatchForColumns(t, columns, rf, "foo", nil)
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
		rf := &stringRangeFilter{
			fieldName: "foo",
			minValue:  "33",
			maxValue:  "5",
		}
		testFilterMatchForColumns(t, columns, rf, "foo", []int{9, 10})

		// mismatch
		rf = &stringRangeFilter{
			fieldName: "foo",
			minValue:  "a",
			maxValue:  "b",
		}
		testFilterMatchForColumns(t, columns, rf, "foo", nil)

		rf = &stringRangeFilter{
			fieldName: "foo",
			minValue:  "100",
			maxValue:  "101",
		}
		testFilterMatchForColumns(t, columns, rf, "foo", nil)

		rf = &stringRangeFilter{
			fieldName: "foo",
			minValue:  "5",
			maxValue:  "33",
		}
		testFilterMatchForColumns(t, columns, rf, "foo", nil)
	})
	t.Run("float64", func(t *testing.T) {
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
		rf := &stringRangeFilter{
			fieldName: "foo",
			minValue:  "33",
			maxValue:  "5",
		}
		testFilterMatchForColumns(t, columns, rf, "foo", []int{9, 10})
		rf = &stringRangeFilter{
			fieldName: "foo",
			minValue:  "-0",
			maxValue:  "-1",
		}
		testFilterMatchForColumns(t, columns, rf, "foo", []int{6})

		// mismatch
		rf = &stringRangeFilter{
			fieldName: "foo",
			minValue:  "a",
			maxValue:  "b",
		}
		testFilterMatchForColumns(t, columns, rf, "foo", nil)

		rf = &stringRangeFilter{
			fieldName: "foo",
			minValue:  "100",
			maxValue:  "101",
		}
		testFilterMatchForColumns(t, columns, rf, "foo", nil)

		rf = &stringRangeFilter{
			fieldName: "foo",
			minValue:  "5",
			maxValue:  "33",
		}
		testFilterMatchForColumns(t, columns, rf, "foo", nil)
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
					"55.55.12.55",
					"66.66.66.66",
					"7.7.7.7",
				},
			},
		}

		// match
		rf := &stringRangeFilter{
			fieldName: "foo",
			minValue:  "127.0.0",
			maxValue:  "128.0.0.0",
		}
		testFilterMatchForColumns(t, columns, rf, "foo", []int{2, 4, 5, 6, 7})

		// mismatch
		rf = &stringRangeFilter{
			fieldName: "foo",
			minValue:  "a",
			maxValue:  "b",
		}
		testFilterMatchForColumns(t, columns, rf, "foo", nil)

		rf = &stringRangeFilter{
			fieldName: "foo",
			minValue:  "128.0.0.0",
			maxValue:  "129.0.0.0",
		}
		testFilterMatchForColumns(t, columns, rf, "foo", nil)

		rf = &stringRangeFilter{
			fieldName: "foo",
			minValue:  "255.0.0.0",
			maxValue:  "255.255.255.255",
		}
		testFilterMatchForColumns(t, columns, rf, "foo", nil)

		rf = &stringRangeFilter{
			fieldName: "foo",
			minValue:  "128.0.0.0",
			maxValue:  "",
		}
		testFilterMatchForColumns(t, columns, rf, "foo", nil)
	})

	t.Run("timestamp-iso8601", func(t *testing.T) {
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
		rf := &stringRangeFilter{
			fieldName: "_msg",
			minValue:  "2006-01-02",
			maxValue:  "2006-01-03",
		}
		testFilterMatchForColumns(t, columns, rf, "_msg", []int{2, 3})

		rf = &stringRangeFilter{
			fieldName: "_msg",
			minValue:  "",
			maxValue:  "2006",
		}
		testFilterMatchForColumns(t, columns, rf, "_msg", []int{0})

		// mismatch
		rf = &stringRangeFilter{
			fieldName: "_msg",
			minValue:  "3",
			maxValue:  "4",
		}
		testFilterMatchForColumns(t, columns, rf, "_msg", nil)

		rf = &stringRangeFilter{
			fieldName: "_msg",
			minValue:  "a",
			maxValue:  "b",
		}
		testFilterMatchForColumns(t, columns, rf, "_msg", nil)

		rf = &stringRangeFilter{
			fieldName: "_msg",
			minValue:  "2006-01-03",
			maxValue:  "2006-01-02",
		}
		testFilterMatchForColumns(t, columns, rf, "_msg", nil)
	})
}

func TestIPv4RangeFilter(t *testing.T) {
	t.Run("const-column", func(t *testing.T) {
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
		rf := &ipv4RangeFilter{
			fieldName: "foo",
			minValue:  0,
			maxValue:  0x80000000,
		}
		testFilterMatchForColumns(t, columns, rf, "foo", []int{0, 1, 2})

		rf = &ipv4RangeFilter{
			fieldName: "foo",
			minValue:  0x7f000001,
			maxValue:  0x7f000001,
		}
		testFilterMatchForColumns(t, columns, rf, "foo", []int{0, 1, 2})

		// mismatch
		rf = &ipv4RangeFilter{
			fieldName: "foo",
			minValue:  0,
			maxValue:  0x7f000000,
		}
		testFilterMatchForColumns(t, columns, rf, "foo", nil)

		rf = &ipv4RangeFilter{
			fieldName: "non-existing-column",
			minValue:  0,
			maxValue:  20000,
		}
		testFilterMatchForColumns(t, columns, rf, "foo", nil)

		rf = &ipv4RangeFilter{
			fieldName: "foo",
			minValue:  0x80000000,
			maxValue:  0,
		}
		testFilterMatchForColumns(t, columns, rf, "foo", nil)
	})

	t.Run("dict", func(t *testing.T) {
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
		rf := &ipv4RangeFilter{
			fieldName: "foo",
			minValue:  0x7f000000,
			maxValue:  0x80000000,
		}
		testFilterMatchForColumns(t, columns, rf, "foo", []int{1, 3, 7})

		rf = &ipv4RangeFilter{
			fieldName: "foo",
			minValue:  0,
			maxValue:  0x7f000001,
		}
		testFilterMatchForColumns(t, columns, rf, "foo", []int{1, 7})

		// mismatch
		rf = &ipv4RangeFilter{
			fieldName: "foo",
			minValue:  0,
			maxValue:  1000,
		}
		testFilterMatchForColumns(t, columns, rf, "foo", nil)

		rf = &ipv4RangeFilter{
			fieldName: "foo",
			minValue:  0x7f000002,
			maxValue:  0x7f7f0000,
		}
		testFilterMatchForColumns(t, columns, rf, "foo", nil)

		rf = &ipv4RangeFilter{
			fieldName: "foo",
			minValue:  0x80000000,
			maxValue:  0x7f000000,
		}
		testFilterMatchForColumns(t, columns, rf, "foo", nil)
	})

	t.Run("strings", func(t *testing.T) {
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
		rf := &ipv4RangeFilter{
			fieldName: "foo",
			minValue:  0x7f000000,
			maxValue:  0xffffffff,
		}
		testFilterMatchForColumns(t, columns, rf, "foo", []int{2})

		// mismatch
		rf = &ipv4RangeFilter{
			fieldName: "foo",
			minValue:  0,
			maxValue:  10000,
		}
		testFilterMatchForColumns(t, columns, rf, "foo", nil)

		rf = &ipv4RangeFilter{
			fieldName: "foo",
			minValue:  0xffffffff,
			maxValue:  0x7f000000,
		}
		testFilterMatchForColumns(t, columns, rf, "foo", nil)
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

		// mismatch
		rf := &ipv4RangeFilter{
			fieldName: "foo",
			minValue:  0,
			maxValue:  0xffffffff,
		}
		testFilterMatchForColumns(t, columns, rf, "foo", nil)
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
		rf := &ipv4RangeFilter{
			fieldName: "foo",
			minValue:  0,
			maxValue:  0xffffffff,
		}
		testFilterMatchForColumns(t, columns, rf, "foo", nil)
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
		rf := &ipv4RangeFilter{
			fieldName: "foo",
			minValue:  0,
			maxValue:  0xffffffff,
		}
		testFilterMatchForColumns(t, columns, rf, "foo", nil)
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
		rf := &ipv4RangeFilter{
			fieldName: "foo",
			minValue:  0,
			maxValue:  0xffffffff,
		}
		testFilterMatchForColumns(t, columns, rf, "foo", nil)
	})

	t.Run("float64", func(t *testing.T) {
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
		rf := &ipv4RangeFilter{
			fieldName: "foo",
			minValue:  0,
			maxValue:  0xffffffff,
		}
		testFilterMatchForColumns(t, columns, rf, "foo", nil)
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
					"55.55.12.55",
					"66.66.66.66",
					"7.7.7.7",
				},
			},
		}

		// match
		rf := &ipv4RangeFilter{
			fieldName: "foo",
			minValue:  0,
			maxValue:  0x08000000,
		}
		testFilterMatchForColumns(t, columns, rf, "foo", []int{0, 1, 11})

		// mismatch
		rf = &ipv4RangeFilter{
			fieldName: "foo",
			minValue:  0x80000000,
			maxValue:  0x90000000,
		}
		testFilterMatchForColumns(t, columns, rf, "foo", nil)

		rf = &ipv4RangeFilter{
			fieldName: "foo",
			minValue:  0xff000000,
			maxValue:  0xffffffff,
		}
		testFilterMatchForColumns(t, columns, rf, "foo", nil)

		rf = &ipv4RangeFilter{
			fieldName: "foo",
			minValue:  0x08000000,
			maxValue:  0,
		}
		testFilterMatchForColumns(t, columns, rf, "foo", nil)
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

		// mismatch
		rf := &ipv4RangeFilter{
			fieldName: "_msg",
			minValue:  0,
			maxValue:  0xffffffff,
		}
		testFilterMatchForColumns(t, columns, rf, "_msg", nil)
	})
}

func TestLenRangeFilter(t *testing.T) {
	t.Run("const-column", func(t *testing.T) {
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
		rf := &lenRangeFilter{
			fieldName: "foo",
			minLen:    2,
			maxLen:    20,
		}
		testFilterMatchForColumns(t, columns, rf, "foo", []int{0, 1, 2})

		rf = &lenRangeFilter{
			fieldName: "non-existing-column",
			minLen:    0,
			maxLen:    10,
		}
		testFilterMatchForColumns(t, columns, rf, "foo", []int{0, 1, 2})

		// mismatch
		rf = &lenRangeFilter{
			fieldName: "foo",
			minLen:    3,
			maxLen:    20,
		}
		testFilterMatchForColumns(t, columns, rf, "foo", nil)

		rf = &lenRangeFilter{
			fieldName: "non-existing-column",
			minLen:    10,
			maxLen:    20,
		}
		testFilterMatchForColumns(t, columns, rf, "foo", nil)
	})

	t.Run("dict", func(t *testing.T) {
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
		rf := &lenRangeFilter{
			fieldName: "foo",
			minLen:    2,
			maxLen:    3,
		}
		testFilterMatchForColumns(t, columns, rf, "foo", []int{1, 2, 3})

		rf = &lenRangeFilter{
			fieldName: "foo",
			minLen:    0,
			maxLen:    1,
		}
		testFilterMatchForColumns(t, columns, rf, "foo", []int{0})

		// mismatch
		rf = &lenRangeFilter{
			fieldName: "foo",
			minLen:    20,
			maxLen:    30,
		}
		testFilterMatchForColumns(t, columns, rf, "foo", nil)
	})

	t.Run("strings", func(t *testing.T) {
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
		rf := &lenRangeFilter{
			fieldName: "foo",
			minLen:    2,
			maxLen:    3,
		}
		testFilterMatchForColumns(t, columns, rf, "foo", []int{2, 3, 5})

		// mismatch
		rf = &lenRangeFilter{
			fieldName: "foo",
			minLen:    100,
			maxLen:    200,
		}
		testFilterMatchForColumns(t, columns, rf, "foo", nil)
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
		rf := &lenRangeFilter{
			fieldName: "foo",
			minLen:    2,
			maxLen:    2,
		}
		testFilterMatchForColumns(t, columns, rf, "foo", []int{2, 3, 6})

		// mismatch
		rf = &lenRangeFilter{
			fieldName: "foo",
			minLen:    0,
			maxLen:    0,
		}
		testFilterMatchForColumns(t, columns, rf, "foo", nil)

		rf = &lenRangeFilter{
			fieldName: "foo",
			minLen:    10,
			maxLen:    10,
		}
		testFilterMatchForColumns(t, columns, rf, "foo", nil)
	})

	t.Run("uint16", func(t *testing.T) {
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
		rf := &lenRangeFilter{
			fieldName: "foo",
			minLen:    2,
			maxLen:    2,
		}
		testFilterMatchForColumns(t, columns, rf, "foo", []int{2, 3, 6})

		// mismatch
		rf = &lenRangeFilter{
			fieldName: "foo",
			minLen:    0,
			maxLen:    0,
		}
		testFilterMatchForColumns(t, columns, rf, "foo", nil)

		rf = &lenRangeFilter{
			fieldName: "foo",
			minLen:    10,
			maxLen:    10,
		}
		testFilterMatchForColumns(t, columns, rf, "foo", nil)
	})

	t.Run("uint32", func(t *testing.T) {
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
		rf := &lenRangeFilter{
			fieldName: "foo",
			minLen:    2,
			maxLen:    2,
		}
		testFilterMatchForColumns(t, columns, rf, "foo", []int{2, 3, 6})

		// mismatch
		rf = &lenRangeFilter{
			fieldName: "foo",
			minLen:    0,
			maxLen:    0,
		}
		testFilterMatchForColumns(t, columns, rf, "foo", nil)

		rf = &lenRangeFilter{
			fieldName: "foo",
			minLen:    10,
			maxLen:    10,
		}
		testFilterMatchForColumns(t, columns, rf, "foo", nil)
	})

	t.Run("uint64", func(t *testing.T) {
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
		rf := &lenRangeFilter{
			fieldName: "foo",
			minLen:    2,
			maxLen:    2,
		}
		testFilterMatchForColumns(t, columns, rf, "foo", []int{2, 3, 6})

		// mismatch
		rf = &lenRangeFilter{
			fieldName: "foo",
			minLen:    0,
			maxLen:    0,
		}
		testFilterMatchForColumns(t, columns, rf, "foo", nil)

		rf = &lenRangeFilter{
			fieldName: "foo",
			minLen:    20,
			maxLen:    20,
		}
		testFilterMatchForColumns(t, columns, rf, "foo", nil)
	})

	t.Run("float64", func(t *testing.T) {
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
		rf := &lenRangeFilter{
			fieldName: "foo",
			minLen:    2,
			maxLen:    2,
		}
		testFilterMatchForColumns(t, columns, rf, "foo", []int{1, 2})

		// mismatch
		rf = &lenRangeFilter{
			fieldName: "foo",
			minLen:    100,
			maxLen:    200,
		}
		testFilterMatchForColumns(t, columns, rf, "foo", nil)
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
					"55.55.12.55",
					"66.66.66.66",
					"7.7.7.7",
				},
			},
		}

		// match
		rf := &lenRangeFilter{
			fieldName: "foo",
			minLen:    3,
			maxLen:    7,
		}
		testFilterMatchForColumns(t, columns, rf, "foo", []int{0, 1, 11})

		// mismatch
		rf = &lenRangeFilter{
			fieldName: "foo",
			minLen:    20,
			maxLen:    30,
		}
		testFilterMatchForColumns(t, columns, rf, "foo", nil)
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
		rf := &lenRangeFilter{
			fieldName: "_msg",
			minLen:    10,
			maxLen:    30,
		}
		testFilterMatchForColumns(t, columns, rf, "_msg", []int{0, 1, 2, 3, 4, 5, 6, 7, 8})

		// mismatch
		rf = &lenRangeFilter{
			fieldName: "_msg",
			minLen:    10,
			maxLen:    11,
		}
		testFilterMatchForColumns(t, columns, rf, "_msg", nil)
	})
}

func TestRangeFilter(t *testing.T) {
	t.Run("const-column", func(t *testing.T) {
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
		rf := &rangeFilter{
			fieldName: "foo",
			minValue:  -10,
			maxValue:  20,
		}
		testFilterMatchForColumns(t, columns, rf, "foo", []int{0, 1, 2})

		rf = &rangeFilter{
			fieldName: "foo",
			minValue:  10,
			maxValue:  10,
		}
		testFilterMatchForColumns(t, columns, rf, "foo", []int{0, 1, 2})

		rf = &rangeFilter{
			fieldName: "foo",
			minValue:  10,
			maxValue:  20,
		}
		testFilterMatchForColumns(t, columns, rf, "foo", []int{0, 1, 2})

		// mismatch
		rf = &rangeFilter{
			fieldName: "foo",
			minValue:  -10,
			maxValue:  9.99,
		}
		testFilterMatchForColumns(t, columns, rf, "foo", nil)

		rf = &rangeFilter{
			fieldName: "foo",
			minValue:  20,
			maxValue:  -10,
		}
		testFilterMatchForColumns(t, columns, rf, "foo", nil)

		rf = &rangeFilter{
			fieldName: "foo",
			minValue:  10.1,
			maxValue:  20,
		}
		testFilterMatchForColumns(t, columns, rf, "foo", nil)

		rf = &rangeFilter{
			fieldName: "non-existing-column",
			minValue:  10,
			maxValue:  20,
		}
		testFilterMatchForColumns(t, columns, rf, "foo", nil)

		rf = &rangeFilter{
			fieldName: "foo",
			minValue:  11,
			maxValue:  10,
		}
		testFilterMatchForColumns(t, columns, rf, "foo", nil)
	})

	t.Run("dict", func(t *testing.T) {
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
		rf := &rangeFilter{
			fieldName: "foo",
			minValue:  -10,
			maxValue:  20,
		}
		testFilterMatchForColumns(t, columns, rf, "foo", []int{1, 3, 4})

		rf = &rangeFilter{
			fieldName: "foo",
			minValue:  10,
			maxValue:  20,
		}
		testFilterMatchForColumns(t, columns, rf, "foo", []int{1, 3, 4})

		rf = &rangeFilter{
			fieldName: "foo",
			minValue:  10.1,
			maxValue:  19.9,
		}
		testFilterMatchForColumns(t, columns, rf, "foo", []int{4})

		// mismatch
		rf = &rangeFilter{
			fieldName: "foo",
			minValue:  -11,
			maxValue:  0,
		}
		testFilterMatchForColumns(t, columns, rf, "foo", nil)

		rf = &rangeFilter{
			fieldName: "foo",
			minValue:  11,
			maxValue:  19,
		}
		testFilterMatchForColumns(t, columns, rf, "foo", nil)

		rf = &rangeFilter{
			fieldName: "foo",
			minValue:  20.1,
			maxValue:  100,
		}
		testFilterMatchForColumns(t, columns, rf, "foo", nil)

		rf = &rangeFilter{
			fieldName: "foo",
			minValue:  20,
			maxValue:  10,
		}
		testFilterMatchForColumns(t, columns, rf, "foo", nil)

	})

	t.Run("strings", func(t *testing.T) {
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
		rf := &rangeFilter{
			fieldName: "foo",
			minValue:  -100,
			maxValue:  100,
		}
		testFilterMatchForColumns(t, columns, rf, "foo", []int{2, 3, 4, 5})

		rf = &rangeFilter{
			fieldName: "foo",
			minValue:  10,
			maxValue:  20,
		}
		testFilterMatchForColumns(t, columns, rf, "foo", []int{2, 3, 4})

		rf = &rangeFilter{
			fieldName: "foo",
			minValue:  -5,
			maxValue:  -5,
		}
		testFilterMatchForColumns(t, columns, rf, "foo", []int{5})

		// mismatch
		rf = &rangeFilter{
			fieldName: "foo",
			minValue:  -10,
			maxValue:  -5.1,
		}
		testFilterMatchForColumns(t, columns, rf, "foo", nil)

		rf = &rangeFilter{
			fieldName: "foo",
			minValue:  20.1,
			maxValue:  100,
		}
		testFilterMatchForColumns(t, columns, rf, "foo", nil)

		rf = &rangeFilter{
			fieldName: "foo",
			minValue:  20,
			maxValue:  10,
		}
		testFilterMatchForColumns(t, columns, rf, "foo", nil)
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
		rf := &rangeFilter{
			fieldName: "foo",
			minValue:  0,
			maxValue:  3,
		}
		testFilterMatchForColumns(t, columns, rf, "foo", []int{3, 4, 6, 7, 8})

		rf = &rangeFilter{
			fieldName: "foo",
			minValue:  0.1,
			maxValue:  2.9,
		}
		testFilterMatchForColumns(t, columns, rf, "foo", []int{6, 7})

		rf = &rangeFilter{
			fieldName: "foo",
			minValue:  -1e18,
			maxValue:  2.9,
		}
		testFilterMatchForColumns(t, columns, rf, "foo", []int{3, 4, 6, 7})

		// mismatch
		rf = &rangeFilter{
			fieldName: "foo",
			minValue:  -1e18,
			maxValue:  -0.1,
		}
		testFilterMatchForColumns(t, columns, rf, "foo", nil)

		rf = &rangeFilter{
			fieldName: "foo",
			minValue:  0.1,
			maxValue:  0.9,
		}
		testFilterMatchForColumns(t, columns, rf, "foo", nil)

		rf = &rangeFilter{
			fieldName: "foo",
			minValue:  2.9,
			maxValue:  0.1,
		}
		testFilterMatchForColumns(t, columns, rf, "foo", nil)

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
		rf := &rangeFilter{
			fieldName: "foo",
			minValue:  0,
			maxValue:  3,
		}
		testFilterMatchForColumns(t, columns, rf, "foo", []int{3, 4, 6, 7, 8})

		rf = &rangeFilter{
			fieldName: "foo",
			minValue:  0.1,
			maxValue:  2.9,
		}
		testFilterMatchForColumns(t, columns, rf, "foo", []int{6, 7})

		rf = &rangeFilter{
			fieldName: "foo",
			minValue:  -1e18,
			maxValue:  2.9,
		}
		testFilterMatchForColumns(t, columns, rf, "foo", []int{3, 4, 6, 7})

		// mismatch
		rf = &rangeFilter{
			fieldName: "foo",
			minValue:  -1e18,
			maxValue:  -0.1,
		}
		testFilterMatchForColumns(t, columns, rf, "foo", nil)

		rf = &rangeFilter{
			fieldName: "foo",
			minValue:  0.1,
			maxValue:  0.9,
		}
		testFilterMatchForColumns(t, columns, rf, "foo", nil)

		rf = &rangeFilter{
			fieldName: "foo",
			minValue:  2.9,
			maxValue:  0.1,
		}
		testFilterMatchForColumns(t, columns, rf, "foo", nil)
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
		rf := &rangeFilter{
			fieldName: "foo",
			minValue:  0,
			maxValue:  3,
		}
		testFilterMatchForColumns(t, columns, rf, "foo", []int{3, 4, 6, 7, 8})

		rf = &rangeFilter{
			fieldName: "foo",
			minValue:  0.1,
			maxValue:  2.9,
		}
		testFilterMatchForColumns(t, columns, rf, "foo", []int{6, 7})

		rf = &rangeFilter{
			fieldName: "foo",
			minValue:  -1e18,
			maxValue:  2.9,
		}
		testFilterMatchForColumns(t, columns, rf, "foo", []int{3, 4, 6, 7})

		// mismatch
		rf = &rangeFilter{
			fieldName: "foo",
			minValue:  -1e18,
			maxValue:  -0.1,
		}
		testFilterMatchForColumns(t, columns, rf, "foo", nil)

		rf = &rangeFilter{
			fieldName: "foo",
			minValue:  0.1,
			maxValue:  0.9,
		}
		testFilterMatchForColumns(t, columns, rf, "foo", nil)

		rf = &rangeFilter{
			fieldName: "foo",
			minValue:  2.9,
			maxValue:  0.1,
		}
		testFilterMatchForColumns(t, columns, rf, "foo", nil)
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
		rf := &rangeFilter{
			fieldName: "foo",
			minValue:  math.Inf(-1),
			maxValue:  3,
		}
		testFilterMatchForColumns(t, columns, rf, "foo", []int{3, 4, 6, 7, 8})

		rf = &rangeFilter{
			fieldName: "foo",
			minValue:  0.1,
			maxValue:  2.9,
		}
		testFilterMatchForColumns(t, columns, rf, "foo", []int{6, 7})

		rf = &rangeFilter{
			fieldName: "foo",
			minValue:  -1e18,
			maxValue:  2.9,
		}
		testFilterMatchForColumns(t, columns, rf, "foo", []int{3, 4, 6, 7})

		rf = &rangeFilter{
			fieldName: "foo",
			minValue:  1000,
			maxValue:  math.Inf(1),
		}
		testFilterMatchForColumns(t, columns, rf, "foo", []int{5})

		// mismatch
		rf = &rangeFilter{
			fieldName: "foo",
			minValue:  -1e18,
			maxValue:  -0.1,
		}
		testFilterMatchForColumns(t, columns, rf, "foo", nil)

		rf = &rangeFilter{
			fieldName: "foo",
			minValue:  0.1,
			maxValue:  0.9,
		}
		testFilterMatchForColumns(t, columns, rf, "foo", nil)

		rf = &rangeFilter{
			fieldName: "foo",
			minValue:  2.9,
			maxValue:  0.1,
		}
		testFilterMatchForColumns(t, columns, rf, "foo", nil)
	})

	t.Run("float64", func(t *testing.T) {
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
		rf := &rangeFilter{
			fieldName: "foo",
			minValue:  math.Inf(-1),
			maxValue:  3,
		}
		testFilterMatchForColumns(t, columns, rf, "foo", []int{3, 4, 6, 7, 8})

		rf = &rangeFilter{
			fieldName: "foo",
			minValue:  0.1,
			maxValue:  2.9,
		}
		testFilterMatchForColumns(t, columns, rf, "foo", []int{7})

		rf = &rangeFilter{
			fieldName: "foo",
			minValue:  -1e18,
			maxValue:  1.9,
		}
		testFilterMatchForColumns(t, columns, rf, "foo", []int{3, 4, 6, 8})

		rf = &rangeFilter{
			fieldName: "foo",
			minValue:  1000,
			maxValue:  math.Inf(1),
		}
		testFilterMatchForColumns(t, columns, rf, "foo", []int{5})

		// mismatch
		rf = &rangeFilter{
			fieldName: "foo",
			minValue:  -1e18,
			maxValue:  -334.1,
		}
		testFilterMatchForColumns(t, columns, rf, "foo", nil)

		rf = &rangeFilter{
			fieldName: "foo",
			minValue:  0.1,
			maxValue:  0.9,
		}
		testFilterMatchForColumns(t, columns, rf, "foo", nil)

		rf = &rangeFilter{
			fieldName: "foo",
			minValue:  2.9,
			maxValue:  0.1,
		}
		testFilterMatchForColumns(t, columns, rf, "foo", nil)
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
					"55.55.12.55",
					"66.66.66.66",
					"7.7.7.7",
				},
			},
		}

		// range filter always mismatches ipv4
		rf := &rangeFilter{
			fieldName: "foo",
			minValue:  -100,
			maxValue:  100,
		}
		testFilterMatchForColumns(t, columns, rf, "foo", nil)
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

		// range filter always mismatches timestmap
		rf := &rangeFilter{
			fieldName: "_msg",
			minValue:  -100,
			maxValue:  100,
		}
		testFilterMatchForColumns(t, columns, rf, "_msg", nil)
	})
}

func TestAnyCasePrefixFilter(t *testing.T) {
	t.Run("single-row", func(t *testing.T) {
		columns := []column{
			{
				name: "foo",
				values: []string{
					"aBc DEf",
				},
			},
			{
				name: "other column",
				values: []string{
					"aSDfdsf",
				},
			},
		}

		// match
		pf := &anyCasePrefixFilter{
			fieldName: "foo",
			prefix:    "abc",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0})

		pf = &anyCasePrefixFilter{
			fieldName: "foo",
			prefix:    "ABC",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0})

		pf = &anyCasePrefixFilter{
			fieldName: "foo",
			prefix:    "",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0})

		pf = &anyCasePrefixFilter{
			fieldName: "foo",
			prefix:    "ab",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0})

		pf = &anyCasePrefixFilter{
			fieldName: "foo",
			prefix:    "abc def",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0})

		pf = &anyCasePrefixFilter{
			fieldName: "foo",
			prefix:    "def",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0})

		pf = &anyCasePrefixFilter{
			fieldName: "other column",
			prefix:    "asdfdSF",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0})

		pf = &anyCasePrefixFilter{
			fieldName: "foo",
			prefix:    "",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0})

		// mismatch
		pf = &anyCasePrefixFilter{
			fieldName: "foo",
			prefix:    "bc",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &anyCasePrefixFilter{
			fieldName: "other column",
			prefix:    "sd",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &anyCasePrefixFilter{
			fieldName: "non-existing column",
			prefix:    "abc",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &anyCasePrefixFilter{
			fieldName: "non-existing column",
			prefix:    "",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)
	})

	t.Run("const-column", func(t *testing.T) {
		columns := []column{
			{
				name: "other-column",
				values: []string{
					"x",
					"X",
					"X",
				},
			},
			{
				name: "foo",
				values: []string{
					"abc def",
					"ABC DEF",
					"AbC Def",
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
		pf := &anyCasePrefixFilter{
			fieldName: "foo",
			prefix:    "Abc",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0, 1, 2})

		pf = &anyCasePrefixFilter{
			fieldName: "foo",
			prefix:    "",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0, 1, 2})

		pf = &anyCasePrefixFilter{
			fieldName: "foo",
			prefix:    "AB",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0, 1, 2})

		pf = &anyCasePrefixFilter{
			fieldName: "foo",
			prefix:    "abc de",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0, 1, 2})

		pf = &anyCasePrefixFilter{
			fieldName: "foo",
			prefix:    " de",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0, 1, 2})

		pf = &anyCasePrefixFilter{
			fieldName: "foo",
			prefix:    "abc def",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0, 1, 2})

		pf = &anyCasePrefixFilter{
			fieldName: "other-column",
			prefix:    "x",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0, 1, 2})

		pf = &anyCasePrefixFilter{
			fieldName: "_msg",
			prefix:    " 2 ",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0, 1, 2})

		// mismatch
		pf = &anyCasePrefixFilter{
			fieldName: "foo",
			prefix:    "abc def ",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &anyCasePrefixFilter{
			fieldName: "foo",
			prefix:    "x",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &anyCasePrefixFilter{
			fieldName: "other-column",
			prefix:    "foo",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &anyCasePrefixFilter{
			fieldName: "non-existing column",
			prefix:    "x",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &anyCasePrefixFilter{
			fieldName: "non-existing column",
			prefix:    "",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &anyCasePrefixFilter{
			fieldName: "_msg",
			prefix:    "foo",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)
	})

	t.Run("dict", func(t *testing.T) {
		columns := []column{
			{
				name: "foo",
				values: []string{
					"",
					"fOObar",
					"Abc",
					"aFDf FooBar baz",
					"fddf FOObarBAZ",
					"AFoobarbaz",
					"foobar",
				},
			},
		}

		// match
		pf := &anyCasePrefixFilter{
			fieldName: "foo",
			prefix:    "FooBar",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{1, 3, 4, 6})

		pf = &anyCasePrefixFilter{
			fieldName: "foo",
			prefix:    "",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{1, 2, 3, 4, 5, 6})

		pf = &anyCasePrefixFilter{
			fieldName: "foo",
			prefix:    "ba",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{3})

		// mismatch
		pf = &anyCasePrefixFilter{
			fieldName: "foo",
			prefix:    "bar",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &anyCasePrefixFilter{
			fieldName: "non-existing column",
			prefix:    "foobar",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &anyCasePrefixFilter{
			fieldName: "non-existing column",
			prefix:    "",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)
	})

	t.Run("strings", func(t *testing.T) {
		columns := []column{
			{
				name: "foo",
				values: []string{
					"A FOO",
					"a fOoBar",
					"aA aBC A",
					"ca afdf a,foobar baz",
					"a fddf foobarbaz",
					"a afoobarbaz",
					"a fooBaR",
					"a kjlkjf dfff",
					"a ТЕСТЙЦУК НГКШ ",
					"a !!,23.(!1)",
				},
			},
		}

		// match
		pf := &anyCasePrefixFilter{
			fieldName: "foo",
			prefix:    "",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9})

		pf = &anyCasePrefixFilter{
			fieldName: "foo",
			prefix:    "a",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9})

		pf = &anyCasePrefixFilter{
			fieldName: "foo",
			prefix:    "нГк",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{8})

		pf = &anyCasePrefixFilter{
			fieldName: "foo",
			prefix:    "aa a",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{2})

		pf = &anyCasePrefixFilter{
			fieldName: "foo",
			prefix:    "!,",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{9})

		// mismatch
		pf = &anyCasePrefixFilter{
			fieldName: "foo",
			prefix:    "aa ax",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &anyCasePrefixFilter{
			fieldName: "foo",
			prefix:    "qwe rty abc",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &anyCasePrefixFilter{
			fieldName: "foo",
			prefix:    "bar",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &anyCasePrefixFilter{
			fieldName: "non-existing-column",
			prefix:    "",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &anyCasePrefixFilter{
			fieldName: "foo",
			prefix:    "@",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)
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
		pf := &anyCasePrefixFilter{
			fieldName: "foo",
			prefix:    "12",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0, 1, 5})

		pf = &anyCasePrefixFilter{
			fieldName: "foo",
			prefix:    "",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10})

		pf = &anyCasePrefixFilter{
			fieldName: "foo",
			prefix:    "0",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{3, 4})

		// mismatch
		pf = &anyCasePrefixFilter{
			fieldName: "foo",
			prefix:    "bar",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &anyCasePrefixFilter{
			fieldName: "foo",
			prefix:    "33",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &anyCasePrefixFilter{
			fieldName: "foo",
			prefix:    "1234",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &anyCasePrefixFilter{
			fieldName: "non-existing-column",
			prefix:    "",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)
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
		pf := &anyCasePrefixFilter{
			fieldName: "foo",
			prefix:    "123",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0, 4})

		pf = &anyCasePrefixFilter{
			fieldName: "foo",
			prefix:    "",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9})

		pf = &anyCasePrefixFilter{
			fieldName: "foo",
			prefix:    "0",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{1})

		// mismatch
		pf = &anyCasePrefixFilter{
			fieldName: "foo",
			prefix:    "bar",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &anyCasePrefixFilter{
			fieldName: "foo",
			prefix:    "33",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &anyCasePrefixFilter{
			fieldName: "foo",
			prefix:    "123456",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &anyCasePrefixFilter{
			fieldName: "non-existing-column",
			prefix:    "",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)
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
		pf := &anyCasePrefixFilter{
			fieldName: "foo",
			prefix:    "123",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0, 4})

		pf = &anyCasePrefixFilter{
			fieldName: "foo",
			prefix:    "",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9})

		pf = &anyCasePrefixFilter{
			fieldName: "foo",
			prefix:    "65536",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{3})

		// mismatch
		pf = &anyCasePrefixFilter{
			fieldName: "foo",
			prefix:    "bar",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &anyCasePrefixFilter{
			fieldName: "foo",
			prefix:    "33",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &anyCasePrefixFilter{
			fieldName: "foo",
			prefix:    "12345678901",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &anyCasePrefixFilter{
			fieldName: "non-existing-column",
			prefix:    "",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)
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
		pf := &anyCasePrefixFilter{
			fieldName: "foo",
			prefix:    "1234",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0, 4})

		pf = &anyCasePrefixFilter{
			fieldName: "foo",
			prefix:    "",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8})

		pf = &anyCasePrefixFilter{
			fieldName: "foo",
			prefix:    "12345678901",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{4})

		// mismatch
		pf = &anyCasePrefixFilter{
			fieldName: "foo",
			prefix:    "bar",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &anyCasePrefixFilter{
			fieldName: "foo",
			prefix:    "33",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &anyCasePrefixFilter{
			fieldName: "foo",
			prefix:    "12345678901234567890",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &anyCasePrefixFilter{
			fieldName: "non-existing-column",
			prefix:    "",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)
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
					"0.0002",
					"-320001",
					"4",
				},
			},
		}

		// match
		pf := &anyCasePrefixFilter{
			fieldName: "foo",
			prefix:    "123",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0, 4})

		pf = &anyCasePrefixFilter{
			fieldName: "foo",
			prefix:    "",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8})

		pf = &anyCasePrefixFilter{
			fieldName: "foo",
			prefix:    "1234.5678901",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{4})

		pf = &anyCasePrefixFilter{
			fieldName: "foo",
			prefix:    "56789",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{4})

		pf = &anyCasePrefixFilter{
			fieldName: "foo",
			prefix:    "-6553",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{3})

		pf = &anyCasePrefixFilter{
			fieldName: "foo",
			prefix:    "65536",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{3})

		// mismatch
		pf = &anyCasePrefixFilter{
			fieldName: "foo",
			prefix:    "bar",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &anyCasePrefixFilter{
			fieldName: "foo",
			prefix:    "7344.8943",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &anyCasePrefixFilter{
			fieldName: "foo",
			prefix:    "-1234",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &anyCasePrefixFilter{
			fieldName: "foo",
			prefix:    "+1234",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &anyCasePrefixFilter{
			fieldName: "foo",
			prefix:    "23",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &anyCasePrefixFilter{
			fieldName: "foo",
			prefix:    "678",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &anyCasePrefixFilter{
			fieldName: "foo",
			prefix:    "33",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &anyCasePrefixFilter{
			fieldName: "foo",
			prefix:    "12345678901234567890",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &anyCasePrefixFilter{
			fieldName: "non-existing-column",
			prefix:    "",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)
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
					"55.55.12.55",
					"66.66.66.66",
					"7.7.7.7",
				},
			},
		}

		// match
		pf := &anyCasePrefixFilter{
			fieldName: "foo",
			prefix:    "127.0.0.1",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{2, 4, 5, 7})

		pf = &anyCasePrefixFilter{
			fieldName: "foo",
			prefix:    "",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11})

		pf = &anyCasePrefixFilter{
			fieldName: "foo",
			prefix:    "12",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{2, 4, 5, 6, 7, 8, 9})

		pf = &anyCasePrefixFilter{
			fieldName: "foo",
			prefix:    "127.0.0",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{2, 4, 5, 7})

		pf = &anyCasePrefixFilter{
			fieldName: "foo",
			prefix:    "2.3.",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0})

		pf = &anyCasePrefixFilter{
			fieldName: "foo",
			prefix:    "0",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{1, 2, 4, 5, 6, 7, 8})

		// mismatch
		pf = &anyCasePrefixFilter{
			fieldName: "foo",
			prefix:    "bar",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &anyCasePrefixFilter{
			fieldName: "foo",
			prefix:    "8",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &anyCasePrefixFilter{
			fieldName: "foo",
			prefix:    "127.1",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &anyCasePrefixFilter{
			fieldName: "foo",
			prefix:    "27.0",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &anyCasePrefixFilter{
			fieldName: "foo",
			prefix:    "255.255.255.255",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &anyCasePrefixFilter{
			fieldName: "non-existing-column",
			prefix:    "",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)
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
		pf := &anyCasePrefixFilter{
			fieldName: "_msg",
			prefix:    "2006-01-02t15:04:05.005z",
		}
		testFilterMatchForColumns(t, columns, pf, "_msg", []int{4})

		pf = &anyCasePrefixFilter{
			fieldName: "_msg",
			prefix:    "",
		}
		testFilterMatchForColumns(t, columns, pf, "_msg", []int{0, 1, 2, 3, 4, 5, 6, 7, 8})

		pf = &anyCasePrefixFilter{
			fieldName: "_msg",
			prefix:    "2006-01-0",
		}
		testFilterMatchForColumns(t, columns, pf, "_msg", []int{0, 1, 2, 3, 4, 5, 6, 7, 8})

		pf = &anyCasePrefixFilter{
			fieldName: "_msg",
			prefix:    "002",
		}
		testFilterMatchForColumns(t, columns, pf, "_msg", []int{1})

		// mimatch
		pf = &anyCasePrefixFilter{
			fieldName: "_msg",
			prefix:    "bar",
		}
		testFilterMatchForColumns(t, columns, pf, "_msg", nil)

		pf = &anyCasePrefixFilter{
			fieldName: "_msg",
			prefix:    "2006-03-02T15:04:05.005Z",
		}
		testFilterMatchForColumns(t, columns, pf, "_msg", nil)

		pf = &anyCasePrefixFilter{
			fieldName: "_msg",
			prefix:    "06",
		}
		testFilterMatchForColumns(t, columns, pf, "_msg", nil)

		// This filter shouldn't match row=4, since it has different string representation of the timestamp
		pf = &anyCasePrefixFilter{
			fieldName: "_msg",
			prefix:    "2006-01-02T16:04:05.005+01:00",
		}
		testFilterMatchForColumns(t, columns, pf, "_msg", nil)

		// This filter shouldn't match row=4, since it contains too many digits for millisecond part
		pf = &anyCasePrefixFilter{
			fieldName: "_msg",
			prefix:    "2006-01-02T15:04:05.00500Z",
		}
		testFilterMatchForColumns(t, columns, pf, "_msg", nil)

		pf = &anyCasePrefixFilter{
			fieldName: "non-existing-column",
			prefix:    "",
		}
		testFilterMatchForColumns(t, columns, pf, "_msg", nil)
	})
}

func TestPrefixFilter(t *testing.T) {
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
		pf := &prefixFilter{
			fieldName: "foo",
			prefix:    "abc",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0})

		pf = &prefixFilter{
			fieldName: "foo",
			prefix:    "",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0})

		pf = &prefixFilter{
			fieldName: "foo",
			prefix:    "ab",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0})

		pf = &prefixFilter{
			fieldName: "foo",
			prefix:    "abc def",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0})

		pf = &prefixFilter{
			fieldName: "foo",
			prefix:    "def",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0})

		pf = &prefixFilter{
			fieldName: "other column",
			prefix:    "asdfdsf",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0})

		pf = &prefixFilter{
			fieldName: "foo",
			prefix:    "",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0})

		// mismatch
		pf = &prefixFilter{
			fieldName: "foo",
			prefix:    "bc",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &prefixFilter{
			fieldName: "other column",
			prefix:    "sd",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &prefixFilter{
			fieldName: "non-existing column",
			prefix:    "abc",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &prefixFilter{
			fieldName: "non-existing column",
			prefix:    "",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)
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
		pf := &prefixFilter{
			fieldName: "foo",
			prefix:    "abc",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0, 1, 2})

		pf = &prefixFilter{
			fieldName: "foo",
			prefix:    "",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0, 1, 2})

		pf = &prefixFilter{
			fieldName: "foo",
			prefix:    "ab",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0, 1, 2})

		pf = &prefixFilter{
			fieldName: "foo",
			prefix:    "abc de",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0, 1, 2})

		pf = &prefixFilter{
			fieldName: "foo",
			prefix:    " de",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0, 1, 2})

		pf = &prefixFilter{
			fieldName: "foo",
			prefix:    "abc def",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0, 1, 2})

		pf = &prefixFilter{
			fieldName: "other-column",
			prefix:    "x",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0, 1, 2})

		pf = &prefixFilter{
			fieldName: "_msg",
			prefix:    " 2 ",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0, 1, 2})

		// mismatch
		pf = &prefixFilter{
			fieldName: "foo",
			prefix:    "abc def ",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &prefixFilter{
			fieldName: "foo",
			prefix:    "x",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &prefixFilter{
			fieldName: "other-column",
			prefix:    "foo",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &prefixFilter{
			fieldName: "non-existing column",
			prefix:    "x",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &prefixFilter{
			fieldName: "non-existing column",
			prefix:    "",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &prefixFilter{
			fieldName: "_msg",
			prefix:    "foo",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)
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
		pf := &prefixFilter{
			fieldName: "foo",
			prefix:    "foobar",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{1, 3, 4, 6})

		pf = &prefixFilter{
			fieldName: "foo",
			prefix:    "",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{1, 2, 3, 4, 5, 6})

		pf = &prefixFilter{
			fieldName: "foo",
			prefix:    "ba",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{3})

		// mismatch
		pf = &prefixFilter{
			fieldName: "foo",
			prefix:    "bar",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &prefixFilter{
			fieldName: "non-existing column",
			prefix:    "foobar",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &prefixFilter{
			fieldName: "non-existing column",
			prefix:    "",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)
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
		pf := &prefixFilter{
			fieldName: "foo",
			prefix:    "",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9})

		pf = &prefixFilter{
			fieldName: "foo",
			prefix:    "a",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9})

		pf = &prefixFilter{
			fieldName: "foo",
			prefix:    "НГК",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{8})

		pf = &prefixFilter{
			fieldName: "foo",
			prefix:    "aa a",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{2})

		pf = &prefixFilter{
			fieldName: "foo",
			prefix:    "!,",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{9})

		// mismatch
		pf = &prefixFilter{
			fieldName: "foo",
			prefix:    "aa ax",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &prefixFilter{
			fieldName: "foo",
			prefix:    "qwe rty abc",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &prefixFilter{
			fieldName: "foo",
			prefix:    "bar",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &prefixFilter{
			fieldName: "non-existing-column",
			prefix:    "",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &prefixFilter{
			fieldName: "foo",
			prefix:    "@",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)
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
		pf := &prefixFilter{
			fieldName: "foo",
			prefix:    "12",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0, 1, 5})

		pf = &prefixFilter{
			fieldName: "foo",
			prefix:    "",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10})

		pf = &prefixFilter{
			fieldName: "foo",
			prefix:    "0",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{3, 4})

		// mismatch
		pf = &prefixFilter{
			fieldName: "foo",
			prefix:    "bar",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &prefixFilter{
			fieldName: "foo",
			prefix:    "33",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &prefixFilter{
			fieldName: "foo",
			prefix:    "1234",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &prefixFilter{
			fieldName: "non-existing-column",
			prefix:    "",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)
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
		pf := &prefixFilter{
			fieldName: "foo",
			prefix:    "123",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0, 4})

		pf = &prefixFilter{
			fieldName: "foo",
			prefix:    "",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9})

		pf = &prefixFilter{
			fieldName: "foo",
			prefix:    "0",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{1})

		// mismatch
		pf = &prefixFilter{
			fieldName: "foo",
			prefix:    "bar",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &prefixFilter{
			fieldName: "foo",
			prefix:    "33",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &prefixFilter{
			fieldName: "foo",
			prefix:    "123456",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &prefixFilter{
			fieldName: "non-existing-column",
			prefix:    "",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)
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
		pf := &prefixFilter{
			fieldName: "foo",
			prefix:    "123",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0, 4})

		pf = &prefixFilter{
			fieldName: "foo",
			prefix:    "",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9})

		pf = &prefixFilter{
			fieldName: "foo",
			prefix:    "65536",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{3})

		// mismatch
		pf = &prefixFilter{
			fieldName: "foo",
			prefix:    "bar",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &prefixFilter{
			fieldName: "foo",
			prefix:    "33",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &prefixFilter{
			fieldName: "foo",
			prefix:    "12345678901",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &prefixFilter{
			fieldName: "non-existing-column",
			prefix:    "",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)
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
		pf := &prefixFilter{
			fieldName: "foo",
			prefix:    "1234",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0, 4})

		pf = &prefixFilter{
			fieldName: "foo",
			prefix:    "",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8})

		pf = &prefixFilter{
			fieldName: "foo",
			prefix:    "12345678901",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{4})

		// mismatch
		pf = &prefixFilter{
			fieldName: "foo",
			prefix:    "bar",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &prefixFilter{
			fieldName: "foo",
			prefix:    "33",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &prefixFilter{
			fieldName: "foo",
			prefix:    "12345678901234567890",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &prefixFilter{
			fieldName: "non-existing-column",
			prefix:    "",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)
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
		pf := &prefixFilter{
			fieldName: "foo",
			prefix:    "123",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0, 4})

		pf = &prefixFilter{
			fieldName: "foo",
			prefix:    "",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8})

		pf = &prefixFilter{
			fieldName: "foo",
			prefix:    "1234.5678901",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{4})

		pf = &prefixFilter{
			fieldName: "foo",
			prefix:    "56789",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{4})

		pf = &prefixFilter{
			fieldName: "foo",
			prefix:    "-6553",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{3})

		pf = &prefixFilter{
			fieldName: "foo",
			prefix:    "65536",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{3})

		// mismatch
		pf = &prefixFilter{
			fieldName: "foo",
			prefix:    "bar",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &prefixFilter{
			fieldName: "foo",
			prefix:    "7344.8943",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &prefixFilter{
			fieldName: "foo",
			prefix:    "-1234",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &prefixFilter{
			fieldName: "foo",
			prefix:    "+1234",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &prefixFilter{
			fieldName: "foo",
			prefix:    "23",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &prefixFilter{
			fieldName: "foo",
			prefix:    "678",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &prefixFilter{
			fieldName: "foo",
			prefix:    "33",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &prefixFilter{
			fieldName: "foo",
			prefix:    "12345678901234567890",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &prefixFilter{
			fieldName: "non-existing-column",
			prefix:    "",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)
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
					"55.55.12.55",
					"66.66.66.66",
					"7.7.7.7",
				},
			},
		}

		// match
		pf := &prefixFilter{
			fieldName: "foo",
			prefix:    "127.0.0.1",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{2, 4, 5, 7})

		pf = &prefixFilter{
			fieldName: "foo",
			prefix:    "",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11})

		pf = &prefixFilter{
			fieldName: "foo",
			prefix:    "12",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{2, 4, 5, 6, 7, 8, 9})

		pf = &prefixFilter{
			fieldName: "foo",
			prefix:    "127.0.0",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{2, 4, 5, 7})

		pf = &prefixFilter{
			fieldName: "foo",
			prefix:    "2.3.",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0})

		pf = &prefixFilter{
			fieldName: "foo",
			prefix:    "0",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{1, 2, 4, 5, 6, 7, 8})

		// mismatch
		pf = &prefixFilter{
			fieldName: "foo",
			prefix:    "bar",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &prefixFilter{
			fieldName: "foo",
			prefix:    "8",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &prefixFilter{
			fieldName: "foo",
			prefix:    "127.1",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &prefixFilter{
			fieldName: "foo",
			prefix:    "27.0",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &prefixFilter{
			fieldName: "foo",
			prefix:    "255.255.255.255",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &prefixFilter{
			fieldName: "non-existing-column",
			prefix:    "",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)
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
		pf := &prefixFilter{
			fieldName: "_msg",
			prefix:    "2006-01-02T15:04:05.005Z",
		}
		testFilterMatchForColumns(t, columns, pf, "_msg", []int{4})

		pf = &prefixFilter{
			fieldName: "_msg",
			prefix:    "",
		}
		testFilterMatchForColumns(t, columns, pf, "_msg", []int{0, 1, 2, 3, 4, 5, 6, 7, 8})

		pf = &prefixFilter{
			fieldName: "_msg",
			prefix:    "2006-01-0",
		}
		testFilterMatchForColumns(t, columns, pf, "_msg", []int{0, 1, 2, 3, 4, 5, 6, 7, 8})

		pf = &prefixFilter{
			fieldName: "_msg",
			prefix:    "002",
		}
		testFilterMatchForColumns(t, columns, pf, "_msg", []int{1})

		// mimatch
		pf = &prefixFilter{
			fieldName: "_msg",
			prefix:    "bar",
		}
		testFilterMatchForColumns(t, columns, pf, "_msg", nil)

		pf = &prefixFilter{
			fieldName: "_msg",
			prefix:    "2006-03-02T15:04:05.005Z",
		}
		testFilterMatchForColumns(t, columns, pf, "_msg", nil)

		pf = &prefixFilter{
			fieldName: "_msg",
			prefix:    "06",
		}
		testFilterMatchForColumns(t, columns, pf, "_msg", nil)

		// This filter shouldn't match row=4, since it has different string representation of the timestamp
		pf = &prefixFilter{
			fieldName: "_msg",
			prefix:    "2006-01-02T16:04:05.005+01:00",
		}
		testFilterMatchForColumns(t, columns, pf, "_msg", nil)

		// This filter shouldn't match row=4, since it contains too many digits for millisecond part
		pf = &prefixFilter{
			fieldName: "_msg",
			prefix:    "2006-01-02T15:04:05.00500Z",
		}
		testFilterMatchForColumns(t, columns, pf, "_msg", nil)

		pf = &prefixFilter{
			fieldName: "non-existing-column",
			prefix:    "",
		}
		testFilterMatchForColumns(t, columns, pf, "_msg", nil)
	})
}

func TestAnyCasePhraseFilter(t *testing.T) {
	t.Run("single-row", func(t *testing.T) {
		columns := []column{
			{
				name: "foo",
				values: []string{
					"aBc DEf",
				},
			},
			{
				name: "other column",
				values: []string{
					"aSDfdsF",
				},
			},
		}

		// match
		pf := &anyCasePhraseFilter{
			fieldName: "foo",
			phrase:    "Abc",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0})

		pf = &anyCasePhraseFilter{
			fieldName: "foo",
			phrase:    "abc def",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0})

		pf = &anyCasePhraseFilter{
			fieldName: "foo",
			phrase:    "def",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0})

		pf = &anyCasePhraseFilter{
			fieldName: "other column",
			phrase:    "ASdfdsf",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0})

		pf = &anyCasePhraseFilter{
			fieldName: "non-existing-column",
			phrase:    "",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0})

		// mismatch
		pf = &anyCasePhraseFilter{
			fieldName: "foo",
			phrase:    "ab",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &anyCasePhraseFilter{
			fieldName: "foo",
			phrase:    "",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &anyCasePhraseFilter{
			fieldName: "other column",
			phrase:    "sd",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &anyCasePhraseFilter{
			fieldName: "non-existing column",
			phrase:    "abc",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)
	})

	t.Run("const-column", func(t *testing.T) {
		columns := []column{
			{
				name: "other-column",
				values: []string{
					"X",
					"x",
					"x",
				},
			},
			{
				name: "foo",
				values: []string{
					"aBC def",
					"abc DEf",
					"Abc deF",
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
		pf := &anyCasePhraseFilter{
			fieldName: "foo",
			phrase:    "abc",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0, 1, 2})

		pf = &anyCasePhraseFilter{
			fieldName: "foo",
			phrase:    "def",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0, 1, 2})

		pf = &anyCasePhraseFilter{
			fieldName: "foo",
			phrase:    " def",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0, 1, 2})

		pf = &anyCasePhraseFilter{
			fieldName: "foo",
			phrase:    "abc def",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0, 1, 2})

		pf = &anyCasePhraseFilter{
			fieldName: "other-column",
			phrase:    "x",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0, 1, 2})

		pf = &anyCasePhraseFilter{
			fieldName: "_msg",
			phrase:    " 2 ",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0, 1, 2})

		pf = &anyCasePhraseFilter{
			fieldName: "non-existing-column",
			phrase:    "",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0, 1, 2})

		// mismatch
		pf = &anyCasePhraseFilter{
			fieldName: "foo",
			phrase:    "abc def ",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &anyCasePhraseFilter{
			fieldName: "foo",
			phrase:    "",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &anyCasePhraseFilter{
			fieldName: "foo",
			phrase:    "x",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &anyCasePhraseFilter{
			fieldName: "other-column",
			phrase:    "foo",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &anyCasePhraseFilter{
			fieldName: "non-existing column",
			phrase:    "x",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &anyCasePhraseFilter{
			fieldName: "_msg",
			phrase:    "foo",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)
	})

	t.Run("dict", func(t *testing.T) {
		columns := []column{
			{
				name: "foo",
				values: []string{
					"",
					"fooBar",
					"ABc",
					"afdf foobar BAz",
					"fddf fOObARbaz",
					"AfooBarbaz",
					"foobar",
				},
			},
		}

		// match
		pf := &anyCasePhraseFilter{
			fieldName: "foo",
			phrase:    "FoobAr",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{1, 3, 6})

		pf = &anyCasePhraseFilter{
			fieldName: "foo",
			phrase:    "",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0})

		pf = &anyCasePhraseFilter{
			fieldName: "foo",
			phrase:    "baZ",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{3})

		pf = &anyCasePhraseFilter{
			fieldName: "non-existing-column",
			phrase:    "",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0, 1, 2, 3, 4, 5, 6})

		// mismatch
		pf = &anyCasePhraseFilter{
			fieldName: "foo",
			phrase:    "bar",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &anyCasePhraseFilter{
			fieldName: "non-existing column",
			phrase:    "foobar",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)
	})

	t.Run("strings", func(t *testing.T) {
		columns := []column{
			{
				name: "foo",
				values: []string{
					"a foo",
					"A Foobar",
					"aA aBC a",
					"ca afdf a,foobar baz",
					"a fddf foobarbaz",
					"a aFOObarbaz",
					"a foobar",
					"a kjlkjf dfff",
					"a ТЕСТЙЦУК НГКШ ",
					"a !!,23.(!1)",
				},
			},
		}

		// match
		pf := &anyCasePhraseFilter{
			fieldName: "foo",
			phrase:    "A",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9})

		pf = &anyCasePhraseFilter{
			fieldName: "foo",
			phrase:    "НгкШ",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{8})

		pf = &anyCasePhraseFilter{
			fieldName: "non-existing-column",
			phrase:    "",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9})

		pf = &anyCasePhraseFilter{
			fieldName: "foo",
			phrase:    "!,",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{9})

		// mismatch
		pf = &anyCasePhraseFilter{
			fieldName: "foo",
			phrase:    "aa a",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &anyCasePhraseFilter{
			fieldName: "foo",
			phrase:    "",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &anyCasePhraseFilter{
			fieldName: "foo",
			phrase:    "bar",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &anyCasePhraseFilter{
			fieldName: "foo",
			phrase:    "@",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)
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
		pf := &anyCasePhraseFilter{
			fieldName: "foo",
			phrase:    "12",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{1, 5})

		pf = &anyCasePhraseFilter{
			fieldName: "foo",
			phrase:    "0",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{3, 4})

		pf = &anyCasePhraseFilter{
			fieldName: "non-existing-column",
			phrase:    "",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10})

		// mismatch
		pf = &anyCasePhraseFilter{
			fieldName: "foo",
			phrase:    "bar",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &anyCasePhraseFilter{
			fieldName: "foo",
			phrase:    "",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &anyCasePhraseFilter{
			fieldName: "foo",
			phrase:    "33",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &anyCasePhraseFilter{
			fieldName: "foo",
			phrase:    "1234",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)
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
		pf := &anyCasePhraseFilter{
			fieldName: "foo",
			phrase:    "1234",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0, 4})

		pf = &anyCasePhraseFilter{
			fieldName: "foo",
			phrase:    "0",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{1})

		pf = &anyCasePhraseFilter{
			fieldName: "non-existing-column",
			phrase:    "",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9})

		// mismatch
		pf = &anyCasePhraseFilter{
			fieldName: "foo",
			phrase:    "bar",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &anyCasePhraseFilter{
			fieldName: "foo",
			phrase:    "",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &anyCasePhraseFilter{
			fieldName: "foo",
			phrase:    "33",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &anyCasePhraseFilter{
			fieldName: "foo",
			phrase:    "123456",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)
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
		pf := &anyCasePhraseFilter{
			fieldName: "foo",
			phrase:    "1234",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0, 4})

		pf = &anyCasePhraseFilter{
			fieldName: "foo",
			phrase:    "65536",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{3})

		pf = &anyCasePhraseFilter{
			fieldName: "non-existing-column",
			phrase:    "",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9})

		// mismatch
		pf = &anyCasePhraseFilter{
			fieldName: "foo",
			phrase:    "bar",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &anyCasePhraseFilter{
			fieldName: "foo",
			phrase:    "",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &anyCasePhraseFilter{
			fieldName: "foo",
			phrase:    "33",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &anyCasePhraseFilter{
			fieldName: "foo",
			phrase:    "12345678901",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)
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
		pf := &anyCasePhraseFilter{
			fieldName: "foo",
			phrase:    "1234",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0})

		pf = &anyCasePhraseFilter{
			fieldName: "foo",
			phrase:    "12345678901",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{4})

		pf = &anyCasePhraseFilter{
			fieldName: "non-existing-column",
			phrase:    "",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8})

		// mismatch
		pf = &anyCasePhraseFilter{
			fieldName: "foo",
			phrase:    "bar",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &anyCasePhraseFilter{
			fieldName: "foo",
			phrase:    "",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &anyCasePhraseFilter{
			fieldName: "foo",
			phrase:    "33",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &anyCasePhraseFilter{
			fieldName: "foo",
			phrase:    "12345678901234567890",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)
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
		pf := &anyCasePhraseFilter{
			fieldName: "foo",
			phrase:    "1234",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0, 4})

		pf = &anyCasePhraseFilter{
			fieldName: "foo",
			phrase:    "1234.5678901",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{4})

		pf = &anyCasePhraseFilter{
			fieldName: "foo",
			phrase:    "5678901",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{4})

		pf = &anyCasePhraseFilter{
			fieldName: "foo",
			phrase:    "-65536",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{3})

		pf = &anyCasePhraseFilter{
			fieldName: "foo",
			phrase:    "65536",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{3})

		pf = &anyCasePhraseFilter{
			fieldName: "non-existing-column",
			phrase:    "",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8})

		// mismatch
		pf = &anyCasePhraseFilter{
			fieldName: "foo",
			phrase:    "bar",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &anyCasePhraseFilter{
			fieldName: "foo",
			phrase:    "-1234",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &anyCasePhraseFilter{
			fieldName: "foo",
			phrase:    "+1234",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &anyCasePhraseFilter{
			fieldName: "foo",
			phrase:    "",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &anyCasePhraseFilter{
			fieldName: "foo",
			phrase:    "123",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &anyCasePhraseFilter{
			fieldName: "foo",
			phrase:    "5678",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &anyCasePhraseFilter{
			fieldName: "foo",
			phrase:    "33",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &anyCasePhraseFilter{
			fieldName: "foo",
			phrase:    "12345678901234567890",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)
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
		pf := &anyCasePhraseFilter{
			fieldName: "foo",
			phrase:    "127.0.0.1",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{2, 4, 5, 7})

		pf = &anyCasePhraseFilter{
			fieldName: "foo",
			phrase:    "127",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{2, 4, 5, 6, 7, 8})

		pf = &anyCasePhraseFilter{
			fieldName: "foo",
			phrase:    "127.0.0",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{2, 4, 5, 7})

		pf = &anyCasePhraseFilter{
			fieldName: "foo",
			phrase:    "2.3",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0})

		pf = &anyCasePhraseFilter{
			fieldName: "foo",
			phrase:    "0",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{1, 2, 4, 5, 6, 7, 8})

		pf = &anyCasePhraseFilter{
			fieldName: "non-existing-column",
			phrase:    "",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11})

		// mismatch
		pf = &anyCasePhraseFilter{
			fieldName: "foo",
			phrase:    "bar",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &anyCasePhraseFilter{
			fieldName: "foo",
			phrase:    "",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &anyCasePhraseFilter{
			fieldName: "foo",
			phrase:    "5",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &anyCasePhraseFilter{
			fieldName: "foo",
			phrase:    "127.1",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &anyCasePhraseFilter{
			fieldName: "foo",
			phrase:    "27.0",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &anyCasePhraseFilter{
			fieldName: "foo",
			phrase:    "255.255.255.255",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)
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
		pf := &anyCasePhraseFilter{
			fieldName: "_msg",
			phrase:    "2006-01-02t15:04:05.005z",
		}
		testFilterMatchForColumns(t, columns, pf, "_msg", []int{4})

		pf = &anyCasePhraseFilter{
			fieldName: "_msg",
			phrase:    "2006-01",
		}
		testFilterMatchForColumns(t, columns, pf, "_msg", []int{0, 1, 2, 3, 4, 5, 6, 7, 8})

		pf = &anyCasePhraseFilter{
			fieldName: "_msg",
			phrase:    "002Z",
		}
		testFilterMatchForColumns(t, columns, pf, "_msg", []int{1})

		pf = &anyCasePhraseFilter{
			fieldName: "non-existing-column",
			phrase:    "",
		}
		testFilterMatchForColumns(t, columns, pf, "_msg", []int{0, 1, 2, 3, 4, 5, 6, 7, 8})

		// mimatch
		pf = &anyCasePhraseFilter{
			fieldName: "_msg",
			phrase:    "bar",
		}
		testFilterMatchForColumns(t, columns, pf, "_msg", nil)

		pf = &anyCasePhraseFilter{
			fieldName: "_msg",
			phrase:    "",
		}
		testFilterMatchForColumns(t, columns, pf, "_msg", nil)

		pf = &anyCasePhraseFilter{
			fieldName: "_msg",
			phrase:    "2006-03-02T15:04:05.005Z",
		}
		testFilterMatchForColumns(t, columns, pf, "_msg", nil)

		pf = &anyCasePhraseFilter{
			fieldName: "_msg",
			phrase:    "06",
		}
		testFilterMatchForColumns(t, columns, pf, "_msg", nil)

		// This filter shouldn't match row=4, since it has different string representation of the timestamp
		pf = &anyCasePhraseFilter{
			fieldName: "_msg",
			phrase:    "2006-01-02T16:04:05.005+01:00",
		}
		testFilterMatchForColumns(t, columns, pf, "_msg", nil)

		// This filter shouldn't match row=4, since it contains too many digits for millisecond part
		pf = &anyCasePhraseFilter{
			fieldName: "_msg",
			phrase:    "2006-01-02T15:04:05.00500Z",
		}
		testFilterMatchForColumns(t, columns, pf, "_msg", nil)
	})
}

func TestPhraseFilter(t *testing.T) {
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
		pf := &phraseFilter{
			fieldName: "foo",
			phrase:    "abc",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0})

		pf = &phraseFilter{
			fieldName: "foo",
			phrase:    "abc def",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0})

		pf = &phraseFilter{
			fieldName: "foo",
			phrase:    "def",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0})

		pf = &phraseFilter{
			fieldName: "other column",
			phrase:    "asdfdsf",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0})

		pf = &phraseFilter{
			fieldName: "non-existing-column",
			phrase:    "",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0})

		// mismatch
		pf = &phraseFilter{
			fieldName: "foo",
			phrase:    "ab",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &phraseFilter{
			fieldName: "foo",
			phrase:    "",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &phraseFilter{
			fieldName: "other column",
			phrase:    "sd",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &phraseFilter{
			fieldName: "non-existing column",
			phrase:    "abc",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)
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
		pf := &phraseFilter{
			fieldName: "foo",
			phrase:    "abc",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0, 1, 2})

		pf = &phraseFilter{
			fieldName: "foo",
			phrase:    "def",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0, 1, 2})

		pf = &phraseFilter{
			fieldName: "foo",
			phrase:    " def",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0, 1, 2})

		pf = &phraseFilter{
			fieldName: "foo",
			phrase:    "abc def",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0, 1, 2})

		pf = &phraseFilter{
			fieldName: "other-column",
			phrase:    "x",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0, 1, 2})

		pf = &phraseFilter{
			fieldName: "_msg",
			phrase:    " 2 ",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0, 1, 2})

		pf = &phraseFilter{
			fieldName: "non-existing-column",
			phrase:    "",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0, 1, 2})

		// mismatch
		pf = &phraseFilter{
			fieldName: "foo",
			phrase:    "abc def ",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &phraseFilter{
			fieldName: "foo",
			phrase:    "",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &phraseFilter{
			fieldName: "foo",
			phrase:    "x",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &phraseFilter{
			fieldName: "other-column",
			phrase:    "foo",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &phraseFilter{
			fieldName: "non-existing column",
			phrase:    "x",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &phraseFilter{
			fieldName: "_msg",
			phrase:    "foo",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)
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
		pf := &phraseFilter{
			fieldName: "foo",
			phrase:    "foobar",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{1, 3, 6})

		pf = &phraseFilter{
			fieldName: "foo",
			phrase:    "",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0})

		pf = &phraseFilter{
			fieldName: "foo",
			phrase:    "baz",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{3})

		pf = &phraseFilter{
			fieldName: "non-existing-column",
			phrase:    "",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0, 1, 2, 3, 4, 5, 6})

		// mismatch
		pf = &phraseFilter{
			fieldName: "foo",
			phrase:    "bar",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &phraseFilter{
			fieldName: "non-existing column",
			phrase:    "foobar",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)
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
		pf := &phraseFilter{
			fieldName: "foo",
			phrase:    "a",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9})

		pf = &phraseFilter{
			fieldName: "foo",
			phrase:    "НГКШ",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{8})

		pf = &phraseFilter{
			fieldName: "non-existing-column",
			phrase:    "",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9})

		pf = &phraseFilter{
			fieldName: "foo",
			phrase:    "!,",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{9})

		// mismatch
		pf = &phraseFilter{
			fieldName: "foo",
			phrase:    "aa a",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &phraseFilter{
			fieldName: "foo",
			phrase:    "",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &phraseFilter{
			fieldName: "foo",
			phrase:    "bar",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &phraseFilter{
			fieldName: "foo",
			phrase:    "@",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)
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
		pf := &phraseFilter{
			fieldName: "foo",
			phrase:    "12",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{1, 5})

		pf = &phraseFilter{
			fieldName: "foo",
			phrase:    "0",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{3, 4})

		pf = &phraseFilter{
			fieldName: "non-existing-column",
			phrase:    "",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10})

		// mismatch
		pf = &phraseFilter{
			fieldName: "foo",
			phrase:    "bar",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &phraseFilter{
			fieldName: "foo",
			phrase:    "",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &phraseFilter{
			fieldName: "foo",
			phrase:    "33",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &phraseFilter{
			fieldName: "foo",
			phrase:    "1234",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)
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
		pf := &phraseFilter{
			fieldName: "foo",
			phrase:    "1234",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0, 4})

		pf = &phraseFilter{
			fieldName: "foo",
			phrase:    "0",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{1})

		pf = &phraseFilter{
			fieldName: "non-existing-column",
			phrase:    "",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9})

		// mismatch
		pf = &phraseFilter{
			fieldName: "foo",
			phrase:    "bar",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &phraseFilter{
			fieldName: "foo",
			phrase:    "",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &phraseFilter{
			fieldName: "foo",
			phrase:    "33",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &phraseFilter{
			fieldName: "foo",
			phrase:    "123456",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)
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
		pf := &phraseFilter{
			fieldName: "foo",
			phrase:    "1234",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0, 4})

		pf = &phraseFilter{
			fieldName: "foo",
			phrase:    "65536",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{3})

		pf = &phraseFilter{
			fieldName: "non-existing-column",
			phrase:    "",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9})

		// mismatch
		pf = &phraseFilter{
			fieldName: "foo",
			phrase:    "bar",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &phraseFilter{
			fieldName: "foo",
			phrase:    "",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &phraseFilter{
			fieldName: "foo",
			phrase:    "33",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &phraseFilter{
			fieldName: "foo",
			phrase:    "12345678901",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)
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
		pf := &phraseFilter{
			fieldName: "foo",
			phrase:    "1234",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0})

		pf = &phraseFilter{
			fieldName: "foo",
			phrase:    "12345678901",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{4})

		pf = &phraseFilter{
			fieldName: "non-existing-column",
			phrase:    "",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8})

		// mismatch
		pf = &phraseFilter{
			fieldName: "foo",
			phrase:    "bar",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &phraseFilter{
			fieldName: "foo",
			phrase:    "",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &phraseFilter{
			fieldName: "foo",
			phrase:    "33",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &phraseFilter{
			fieldName: "foo",
			phrase:    "12345678901234567890",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)
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
		pf := &phraseFilter{
			fieldName: "foo",
			phrase:    "1234",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0, 4})

		pf = &phraseFilter{
			fieldName: "foo",
			phrase:    "1234.5678901",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{4})

		pf = &phraseFilter{
			fieldName: "foo",
			phrase:    "5678901",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{4})

		pf = &phraseFilter{
			fieldName: "foo",
			phrase:    "-65536",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{3})

		pf = &phraseFilter{
			fieldName: "foo",
			phrase:    "65536",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{3})

		pf = &phraseFilter{
			fieldName: "non-existing-column",
			phrase:    "",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8})

		// mismatch
		pf = &phraseFilter{
			fieldName: "foo",
			phrase:    "bar",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &phraseFilter{
			fieldName: "foo",
			phrase:    "-1234",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &phraseFilter{
			fieldName: "foo",
			phrase:    "+1234",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &phraseFilter{
			fieldName: "foo",
			phrase:    "",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &phraseFilter{
			fieldName: "foo",
			phrase:    "123",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &phraseFilter{
			fieldName: "foo",
			phrase:    "5678",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &phraseFilter{
			fieldName: "foo",
			phrase:    "33",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &phraseFilter{
			fieldName: "foo",
			phrase:    "12345678901234567890",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)
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
		pf := &phraseFilter{
			fieldName: "foo",
			phrase:    "127.0.0.1",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{2, 4, 5, 7})

		pf = &phraseFilter{
			fieldName: "foo",
			phrase:    "127",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{2, 4, 5, 6, 7, 8})

		pf = &phraseFilter{
			fieldName: "foo",
			phrase:    "127.0.0",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{2, 4, 5, 7})

		pf = &phraseFilter{
			fieldName: "foo",
			phrase:    "2.3",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0})

		pf = &phraseFilter{
			fieldName: "foo",
			phrase:    "0",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{1, 2, 4, 5, 6, 7, 8})

		pf = &phraseFilter{
			fieldName: "non-existing-column",
			phrase:    "",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11})

		// mismatch
		pf = &phraseFilter{
			fieldName: "foo",
			phrase:    "bar",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &phraseFilter{
			fieldName: "foo",
			phrase:    "",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &phraseFilter{
			fieldName: "foo",
			phrase:    "5",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &phraseFilter{
			fieldName: "foo",
			phrase:    "127.1",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &phraseFilter{
			fieldName: "foo",
			phrase:    "27.0",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)

		pf = &phraseFilter{
			fieldName: "foo",
			phrase:    "255.255.255.255",
		}
		testFilterMatchForColumns(t, columns, pf, "foo", nil)
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
		pf := &phraseFilter{
			fieldName: "_msg",
			phrase:    "2006-01-02T15:04:05.005Z",
		}
		testFilterMatchForColumns(t, columns, pf, "_msg", []int{4})

		pf = &phraseFilter{
			fieldName: "_msg",
			phrase:    "2006-01",
		}
		testFilterMatchForColumns(t, columns, pf, "_msg", []int{0, 1, 2, 3, 4, 5, 6, 7, 8})

		pf = &phraseFilter{
			fieldName: "_msg",
			phrase:    "002Z",
		}
		testFilterMatchForColumns(t, columns, pf, "_msg", []int{1})

		pf = &phraseFilter{
			fieldName: "non-existing-column",
			phrase:    "",
		}
		testFilterMatchForColumns(t, columns, pf, "_msg", []int{0, 1, 2, 3, 4, 5, 6, 7, 8})

		// mimatch
		pf = &phraseFilter{
			fieldName: "_msg",
			phrase:    "bar",
		}
		testFilterMatchForColumns(t, columns, pf, "_msg", nil)

		pf = &phraseFilter{
			fieldName: "_msg",
			phrase:    "",
		}
		testFilterMatchForColumns(t, columns, pf, "_msg", nil)

		pf = &phraseFilter{
			fieldName: "_msg",
			phrase:    "2006-03-02T15:04:05.005Z",
		}
		testFilterMatchForColumns(t, columns, pf, "_msg", nil)

		pf = &phraseFilter{
			fieldName: "_msg",
			phrase:    "06",
		}
		testFilterMatchForColumns(t, columns, pf, "_msg", nil)

		// This filter shouldn't match row=4, since it has different string representation of the timestamp
		pf = &phraseFilter{
			fieldName: "_msg",
			phrase:    "2006-01-02T16:04:05.005+01:00",
		}
		testFilterMatchForColumns(t, columns, pf, "_msg", nil)

		// This filter shouldn't match row=4, since it contains too many digits for millisecond part
		pf = &phraseFilter{
			fieldName: "_msg",
			phrase:    "2006-01-02T15:04:05.00500Z",
		}
		testFilterMatchForColumns(t, columns, pf, "_msg", nil)
	})
}

func testFilterMatchForTimestamps(t *testing.T, timestamps []int64, f filter, expectedRowIdxs []int) {
	t.Helper()

	// Create the test storage
	const storagePath = "testFilterMatchForTimestamps"
	cfg := &StorageConfig{}
	s := MustOpenStorage(storagePath, cfg)

	// Generate rows
	getValue := func(rowIdx int) string {
		return fmt.Sprintf("some value for row %d", rowIdx)
	}
	tenantID := TenantID{
		AccountID: 123,
		ProjectID: 456,
	}
	generateRowsFromTimestamps(s, tenantID, timestamps, getValue)

	expectedResults := make([]string, len(expectedRowIdxs))
	expectedTimestamps := make([]int64, len(expectedRowIdxs))
	for i, idx := range expectedRowIdxs {
		expectedResults[i] = getValue(idx)
		expectedTimestamps[i] = timestamps[idx]
	}

	testFilterMatchForStorage(t, s, tenantID, f, "_msg", expectedResults, expectedTimestamps)

	// Close and delete the test storage
	s.MustClose()
	fs.MustRemoveAll(storagePath)
}

func testFilterMatchForColumns(t *testing.T, columns []column, f filter, resultColumnName string, expectedRowIdxs []int) {
	t.Helper()

	// Create the test storage
	const storagePath = "testFilterMatchForColumns"
	cfg := &StorageConfig{}
	s := MustOpenStorage(storagePath, cfg)

	// Generate rows
	tenantID := TenantID{
		AccountID: 123,
		ProjectID: 456,
	}
	generateRowsFromColumns(s, tenantID, columns)

	var values []string
	for _, c := range columns {
		if c.name == resultColumnName {
			values = c.values
			break
		}
	}
	expectedResults := make([]string, len(expectedRowIdxs))
	expectedTimestamps := make([]int64, len(expectedRowIdxs))
	for i, idx := range expectedRowIdxs {
		expectedResults[i] = values[idx]
		expectedTimestamps[i] = int64(idx) * 1e9
	}

	testFilterMatchForStorage(t, s, tenantID, f, resultColumnName, expectedResults, expectedTimestamps)

	// Close and delete the test storage
	s.MustClose()
	fs.MustRemoveAll(storagePath)
}

func testFilterMatchForStorage(t *testing.T, s *Storage, tenantID TenantID, f filter, resultColumnName string, expectedResults []string, expectedTimestamps []int64) {
	t.Helper()

	so := &genericSearchOptions{
		tenantIDs:         []TenantID{tenantID},
		filter:            f,
		resultColumnNames: []string{resultColumnName},
	}
	workersCount := 3
	s.search(workersCount, so, nil, func(workerID uint, br *blockResult) {
		// Verify tenantID
		if !br.streamID.tenantID.equal(&tenantID) {
			t.Fatalf("unexpected tenantID in blockResult; got %s; want %s", &br.streamID.tenantID, &tenantID)
		}

		// Verify columns
		if len(br.cs) != 1 {
			t.Fatalf("unexpected number of columns in blockResult; got %d; want 1", len(br.cs))
		}
		results := br.getColumnValues(0)
		if !reflect.DeepEqual(results, expectedResults) {
			t.Fatalf("unexpected results matched;\ngot\n%q\nwant\n%q", results, expectedResults)
		}

		// Verify timestamps
		if br.timestamps == nil {
			br.timestamps = []int64{}
		}
		if !reflect.DeepEqual(br.timestamps, expectedTimestamps) {
			t.Fatalf("unexpected timestamps;\ngot\n%d\nwant\n%d", br.timestamps, expectedTimestamps)
		}
	})
}

func generateRowsFromColumns(s *Storage, tenantID TenantID, columns []column) {
	streamTags := []string{
		"job",
		"instance",
	}
	lr := GetLogRows(streamTags, nil)
	var fields []Field
	for i := range columns[0].values {
		// Add stream tags
		fields = append(fields[:0], Field{
			Name:  "job",
			Value: "foobar",
		}, Field{
			Name:  "instance",
			Value: "host1:234",
		})
		// Add other columns
		for j := range columns {
			fields = append(fields, Field{
				Name:  columns[j].name,
				Value: columns[j].values[i],
			})
		}
		timestamp := int64(i) * 1e9
		lr.MustAdd(tenantID, timestamp, fields)
	}
	s.MustAddRows(lr)
	PutLogRows(lr)
}

func generateRowsFromTimestamps(s *Storage, tenantID TenantID, timestamps []int64, getValue func(rowIdx int) string) {
	lr := GetLogRows(nil, nil)
	var fields []Field
	for i, timestamp := range timestamps {
		fields = append(fields[:0], Field{
			Name:  "_msg",
			Value: getValue(i),
		})
		lr.MustAdd(tenantID, timestamp, fields)
	}
	s.MustAddRows(lr)
	PutLogRows(lr)
}
