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
	f := func(s, neededFields, unneededFields, neededFieldsExpected, unneededFieldsExpected string) {
		t.Helper()
		expectPipeNeededFields(t, s, neededFields, unneededFields, neededFieldsExpected, unneededFieldsExpected)
	}

	// all the needed fields
	f("del s1,s2", "*", "", "*", "s1,s2")

	// all the needed fields, unneeded fields do not intersect with src
	f("del s1,s2", "*", "f1,f2", "*", "s1,s2,f1,f2")

	// all the needed fields, unneeded fields intersect with src
	f("del s1,s2", "*", "s1,f1,f2", "*", "s1,s2,f1,f2")

	// needed fields do not intersect with src
	f("del s1,s2", "f1,f2", "", "f1,f2", "")

	// needed fields intersect with src
	f("del s1,s2", "s1,f1,f2", "", "f1,f2", "")
}
