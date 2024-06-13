package logstorage

import (
	"testing"
)

func TestParsePipeFieldsSuccess(t *testing.T) {
	f := func(pipeStr string) {
		t.Helper()
		expectParsePipeSuccess(t, pipeStr)
	}

	f(`fields *`)
	f(`fields f1`)
	f(`fields f1, f2, f3`)
}

func TestParsePipeFieldsFailure(t *testing.T) {
	f := func(pipeStr string) {
		t.Helper()
		expectParsePipeFailure(t, pipeStr)
	}

	f(`fields`)
	f(`fields x y`)
}

func TestPipeFields(t *testing.T) {
	f := func(pipeStr string, rows, rowsExpected [][]Field) {
		t.Helper()
		expectPipeResults(t, pipeStr, rows, rowsExpected)
	}

	// single row, star
	f("fields *", [][]Field{
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

	// single row, leave existing field
	f("fields a", [][]Field{
		{
			{"_msg", `{"foo":"bar"}`},
			{"a", `test`},
		},
	}, [][]Field{
		{
			{"a", `test`},
		},
	})

	// single row, no existing fields
	f("fields x, y", [][]Field{
		{
			{"_msg", `{"foo":"bar"}`},
			{"a", `test`},
		},
	}, [][]Field{
		{
			{"x", ``},
			{"y", ``},
		},
	})

	// single row, mention existing field multiple times
	f("fields a, a", [][]Field{
		{
			{"_msg", `{"foo":"bar"}`},
			{"a", `test`},
		},
	}, [][]Field{
		{
			{"a", `test`},
		},
	})

	// mention non-existing fields
	f("fields foo, a, bar", [][]Field{
		{
			{"_msg", `{"foo":"bar"}`},
			{"a", `test`},
		},
	}, [][]Field{
		{
			{"foo", ""},
			{"bar", ""},
			{"a", `test`},
		},
	})

	// Multiple rows
	f("fields a, b", [][]Field{
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
			{"d", "df"},
		},
	}, [][]Field{
		{
			{"a", `test`},
			{"b", ``},
		},
		{
			{"a", `foobar`},
			{"b", ""},
		},
		{
			{"a", ""},
			{"b", "baz"},
		},
		{
			{"a", ""},
			{"b", ""},
		},
	})
}

func TestPipeFieldsUpdateNeededFields(t *testing.T) {
	f := func(s, neededFields, unneededFields, neededFieldsExpected, unneededFieldsExpected string) {
		t.Helper()
		expectPipeNeededFields(t, s, neededFields, unneededFields, neededFieldsExpected, unneededFieldsExpected)
	}

	// all the needed fields
	f("fields s1, s2", "*", "", "s1,s2", "")
	f("fields *", "*", "", "*", "")

	// all the needed fields, unneeded fields do not intersect with src
	f("fields s1, s2", "*", "f1,f2", "s1,s2", "")
	f("fields *", "*", "f1,f2", "*", "f1,f2")

	// all the needed fields, unneeded fields intersect with src
	f("fields s1, s2", "*", "s1,f1,f2", "s2", "")
	f("fields *", "*", "s1,f1,f2", "*", "s1,f1,f2")

	// needed fields do not intersect with src
	f("fields s1, s2", "f1,f2", "", "", "")

	// needed fields intersect with src
	f("fields s1, s2", "s1,f1,f2", "", "s1", "")
	f("fields *", "s1,f1,f2", "", "s1,f1,f2", "")
}
