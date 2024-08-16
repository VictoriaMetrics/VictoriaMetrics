package logstorage

import (
	"testing"
)

func TestParsePipeFilterSuccess(t *testing.T) {
	f := func(pipeStr string) {
		t.Helper()
		expectParsePipeSuccess(t, pipeStr)
	}

	f(`filter *`)
	f(`filter foo bar`)
	f(`filter a:b or c:d in(x,y) z:>343`)
}

func TestParsePipeFilterFailure(t *testing.T) {
	f := func(pipeStr string) {
		t.Helper()
		expectParsePipeFailure(t, pipeStr)
	}

	f(`filter`)
	f(`filter |`)
	f(`filter ()`)
}

func TestPipeFilter(t *testing.T) {
	f := func(pipeStr string, rows, rowsExpected [][]Field) {
		t.Helper()
		expectPipeResults(t, pipeStr, rows, rowsExpected)
	}

	// filter mismatch, missing 'filter' prefix
	f("abc", [][]Field{
		{
			{"_msg", `{"foo":"bar"}`},
			{"a", `test`},
		},
	}, [][]Field{})

	// filter mismatch
	f("filter abc", [][]Field{
		{
			{"_msg", `{"foo":"bar"}`},
			{"a", `test`},
		},
	}, [][]Field{})

	// filter match, missing 'filter' prefix
	f("foo", [][]Field{
		{
			{"_msg", `{"foo":"bar"}`},
			{"a", `test`},
		},
	}, [][]Field{
		{
			{"_msg", `{"foo":"bar"}`},
			{"a", `test`},
		},
	})

	// filter match
	f("filter foo", [][]Field{
		{
			{"_msg", `{"foo":"bar"}`},
			{"a", `test`},
		},
	}, [][]Field{
		{
			{"_msg", `{"foo":"bar"}`},
			{"a", `test`},
		},
	})

	// multiple rows
	f("where x:foo y:bar", [][]Field{
		{
			{"a", "f1"},
			{"x", "foo"},
			{"y", "bar"},
		},
		{
			{"a", "f2"},
			{"x", "x foo bar"},
			{"y", "aa bar bbb"},
			{"z", "iwert"},
		},
		{
			{"a", "f3"},
			{"x", "x fo bar"},
			{"y", "aa bar bbb"},
			{"z", "it"},
		},
		{
			{"a", "f4"},
			{"x", "x foo bar"},
			{"y", "aa ba bbb"},
			{"z", "t"},
		},
		{
			{"x", "x foo"},
			{"y", "aa bar"},
		},
	}, [][]Field{
		{
			{"a", "f1"},
			{"x", "foo"},
			{"y", "bar"},
		},
		{
			{"a", "f2"},
			{"x", "x foo bar"},
			{"y", "aa bar bbb"},
			{"z", "iwert"},
		},
		{
			{"x", "x foo"},
			{"y", "aa bar"},
		},
	})
}

func TestPipeFilterUpdateNeededFields(t *testing.T) {
	f := func(s string, neededFields, unneededFields, neededFieldsExpected, unneededFieldsExpected string) {
		t.Helper()
		expectPipeNeededFields(t, s, neededFields, unneededFields, neededFieldsExpected, unneededFieldsExpected)
	}

	// all the needed fields
	f("filter foo f1:bar", "*", "", "*", "")

	// all the needed fields, unneeded fields do not intersect with src
	f("filter foo f3:bar", "*", "f1,f2", "*", "f1,f2")

	// all the needed fields, unneeded fields intersect with src
	f("filter foo f1:bar", "*", "s1,f1,f2", "*", "s1,f2")

	// needed fields do not intersect with src
	f("filter foo f3:bar", "f1,f2", "", "_msg,f1,f2,f3", "")

	// needed fields intersect with src
	f("filter foo f1:bar", "s1,f1,f2", "", "_msg,f1,f2,s1", "")
}
