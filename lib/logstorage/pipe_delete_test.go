package logstorage

import (
	"testing"
)

func TestParsePipeDeleteSuccess(t *testing.T) {
	f := func(pipeStr string) {
		t.Helper()
		expectParsePipeSuccess(t, pipeStr)
	}

	f(`delete f1`)
	f(`delete f1, f2`)
	f(`delete *`)
	f(`delete f*, bar, baz*`)
}

func TestParsePipeDeleteFailure(t *testing.T) {
	f := func(pipeStr string) {
		t.Helper()
		expectParsePipeFailure(t, pipeStr)
	}

	f(`delete`)
	f(`delete x y`)
}

func TestPipeDelete(t *testing.T) {
	f := func(pipeStr string, rows, rowsExpected [][]Field) {
		t.Helper()
		expectPipeResults(t, pipeStr, rows, rowsExpected)
	}

	// single row, drop existing field
	f("delete _msg", [][]Field{
		{
			{"_msg", `{"foo":"bar"}`},
			{"a", `test`},
		},
	}, [][]Field{
		{
			{"a", `test`},
		},
	})

	// single row, drop existing field multiple times
	f("delete _msg, _msg", [][]Field{
		{
			{"_msg", `{"foo":"bar"}`},
			{"a", `test`},
		},
	}, [][]Field{
		{
			{"a", `test`},
		},
	})

	// single row, drop all the fields
	f("delete a, _msg", [][]Field{
		{
			{"_msg", `{"foo":"bar"}`},
			{"a", `test`},
		},
	}, [][]Field{
		{},
	})

	// delete non-existing fields
	f("delete foo, _msg, bar", [][]Field{
		{
			{"_msg", `{"foo":"bar"}`},
			{"a", `test`},
		},
	}, [][]Field{
		{
			{"a", `test`},
		},
	})

	// Wildcard delete all
	f("delete *", [][]Field{
		{
			{"a", "foo"},
			{"b", "bar"},
			{"c", "1235"},
		},
	}, [][]Field{
		{},
	})

	// Wildcard delete some
	f("delete b*", [][]Field{
		{
			{"a", "foo"},
			{"b", "bar"},
			{"bc", "1235"},
		},
	}, [][]Field{
		{
			{"a", "foo"},
		},
	})

	// Multiple rows
	f("delete _msg, a", [][]Field{
		{
			{"_msg", `{"foo":"bar"}`},
			{"a", `test`},
		},
		{
			{"a", `foobar`},
		},
		{
			{"b", `baz`},
			{"c", "d"},
			{"e", "afdf"},
		},
		{
			{"c", "dss"},
			{"b", "df"},
		},
	}, [][]Field{
		{},
		{},
		{
			{"b", `baz`},
			{"c", "d"},
			{"e", "afdf"},
		},
		{
			{"c", "dss"},
			{"b", "df"},
		},
	})
}

func TestPipeDeleteUpdateNeededFields(t *testing.T) {
	f := func(s, allowFilters, denyFilters, allowFiltersExpected, denyFiltersExpected string) {
		t.Helper()
		expectPipeNeededFields(t, s, allowFilters, denyFilters, allowFiltersExpected, denyFiltersExpected)
	}

	// all the needed fields
	f("del s1,s2", "*", "", "*", "s1,s2")
	f("del s*,s2,x", "*", "", "*", "s*,x")
	f("del *", "*", "", "", "")

	// all the needed fields, unneeded fields do not intersect with src
	f("del s1,s2", "*", "f1,f2", "*", "s1,s2,f1,f2")
	f("del s1,s2", "*", "f*", "*", "s1,s2,f*")
	f("del s*,s2", "*", "f1,f2", "*", "s*,f1,f2")
	f("del s*,s2", "*", "f*", "*", "s*,f*")

	// all the needed fields, unneeded fields intersect with src
	f("del s1,s2", "*", "s1,f1,f2", "*", "s1,s2,f1,f2")
	f("del s1,s2", "*", "s*,f*", "*", "s*,f*")
	f("del s*", "*", "s1,f1,f2", "*", "s*,f1,f2")
	f("del s*", "*", "s*,f*", "*", "s*,f*")

	// needed fields do not intersect with src
	f("del s1,s2", "f1,f2", "", "f1,f2", "")
	f("del s1,s2", "f*", "", "f*", "")
	f("del s*", "f1,f2", "", "f1,f2", "")
	f("del s*", "f*", "", "f*", "")

	// needed fields intersect with src
	f("del s1,s2", "s1,f1,f2", "", "f1,f2", "")
	f("del s1,s2", "s*,f*", "", "f*,s*", "s1,s2")
	f("del s*", "s1,f1,f2", "", "f1,f2", "")
	f("del s*", "s*,f*", "", "f*", "s*")
}
