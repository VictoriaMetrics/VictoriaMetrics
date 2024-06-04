package logstorage

import (
	"testing"
)

func TestParsePipeDropEmptyFieldsSuccess(t *testing.T) {
	f := func(pipeStr string) {
		t.Helper()
		expectParsePipeSuccess(t, pipeStr)
	}

	f(`drop_empty_fields`)
}

func TestParsePipeDropEmptyFieldsFailure(t *testing.T) {
	f := func(pipeStr string) {
		t.Helper()
		expectParsePipeFailure(t, pipeStr)
	}

	f(`drop_empty_fields foo`)
}

func TestPipeDropEmptyFields(t *testing.T) {
	f := func(pipeStr string, rows, rowsExpected [][]Field) {
		t.Helper()
		expectPipeResults(t, pipeStr, rows, rowsExpected)
	}

	f(`drop_empty_fields`, [][]Field{
		{
			{"a", "foo"},
			{"b", "bar"},
			{"c", "baz"},
		},
	}, [][]Field{
		{
			{"a", "foo"},
			{"b", "bar"},
			{"c", "baz"},
		},
	})
	f(`drop_empty_fields`, [][]Field{
		{
			{"a", "foo"},
			{"b", "bar"},
			{"c", "baz"},
		},
		{
			{"a", "foo1"},
			{"b", ""},
			{"c", "baz1"},
		},
		{
			{"a", ""},
			{"b", "bar2"},
		},
		{
			{"a", ""},
			{"b", ""},
			{"c", ""},
		},
	}, [][]Field{
		{
			{"a", "foo"},
			{"b", "bar"},
			{"c", "baz"},
		},
		{
			{"a", "foo1"},
			{"c", "baz1"},
		},
		{
			{"b", "bar2"},
		},
	})
}

func TestPipeDropEmptyFieldsUpdateNeededFields(t *testing.T) {
	f := func(s string, neededFields, unneededFields, neededFieldsExpected, unneededFieldsExpected string) {
		t.Helper()
		expectPipeNeededFields(t, s, neededFields, unneededFields, neededFieldsExpected, unneededFieldsExpected)
	}

	// all the needed fields
	f(`drop_empty_fields`, "*", "", "*", "")

	// non-empty unneeded fields
	f(`drop_empty_fields`, "*", "f1,f2", "*", "f1,f2")

	// non-empty needed fields
	f(`drop_empty_fields`, "x,y", "", "x,y", "")
}
