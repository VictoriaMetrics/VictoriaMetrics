package logstorage

import (
	"testing"
)

func TestParsePipeFieldNamesSuccess(t *testing.T) {
	f := func(pipeStr string) {
		t.Helper()
		expectParsePipeSuccess(t, pipeStr)
	}

	f(`field_names`)
	f(`field_names as x`)
}

func TestParsePipeFieldNamesFailure(t *testing.T) {
	f := func(pipeStr string) {
		t.Helper()
		expectParsePipeFailure(t, pipeStr)
	}

	f(`field_names(foo)`)
	f(`field_names a b`)
	f(`field_names as`)
}

func TestPipeFieldNames(t *testing.T) {
	f := func(pipeStr string, rows, rowsExpected [][]Field) {
		t.Helper()
		expectPipeResults(t, pipeStr, rows, rowsExpected)
	}

	// single row, result column doesn't clash with original columns
	f("field_names", [][]Field{
		{
			{"_msg", `{"foo":"bar"}`},
			{"a", `test`},
		},
	}, [][]Field{
		{
			{"name", "_msg"},
			{"hits", "1"},
		},
		{
			{"name", "a"},
			{"hits", "1"},
		},
	})

	// single row, result column do clashes with original columns
	f("field_names as x", [][]Field{
		{
			{"a", `test`},
			{"b", "aaa"},
		},
		{
			{"a", `bar`},
		},
		{
			{"a", `bar`},
			{"c", `bar`},
		},
	}, [][]Field{
		{
			{"x", "a"},
			{"hits", "3"},
		},
		{
			{"x", "b"},
			{"hits", "1"},
		},
		{
			{"x", "c"},
			{"hits", "1"},
		},
	})
}

func TestPipeFieldNamesUpdateNeededFields(t *testing.T) {
	f := func(s string, neededFields, unneededFields, neededFieldsExpected, unneededFieldsExpected string) {
		t.Helper()
		expectPipeNeededFields(t, s, neededFields, unneededFields, neededFieldsExpected, unneededFieldsExpected)
	}

	// all the needed fields
	f("field_names as f1", "*", "", "*", "")

	// all the needed fields, unneeded fields do not intersect with src
	f("field_names as f3", "*", "f1,f2", "*", "")

	// all the needed fields, unneeded fields intersect with src
	f("field_names as f1", "*", "s1,f1,f2", "*", "")

	// needed fields do not intersect with src
	f("field_names as f3", "f1,f2", "", "*", "")

	// needed fields intersect with src
	f("field_names as f1", "s1,f1,f2", "", "*", "")
}
