package logstorage

import (
	"testing"
)

func TestParsePipeOffsetSuccess(t *testing.T) {
	f := func(pipeStr string) {
		t.Helper()
		expectParsePipeSuccess(t, pipeStr)
	}

	f(`offset 10`)
	f(`offset 10000`)
}

func TestParsePipeOffsetFailure(t *testing.T) {
	f := func(pipeStr string) {
		t.Helper()
		expectParsePipeFailure(t, pipeStr)
	}

	f(`offset`)
	f(`offset -10`)
	f(`offset foo`)
}

func TestPipeOffset(t *testing.T) {
	f := func(pipeStr string, rows, rowsExpected [][]Field) {
		t.Helper()
		expectPipeResults(t, pipeStr, rows, rowsExpected)
	}

	f("offset 100", [][]Field{
		{
			{"_msg", `{"foo":"bar"}`},
			{"a", `test`},
		},
	}, [][]Field{})

	f("offset 0", [][]Field{
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

	f("offset 1", [][]Field{
		{
			{"_msg", `{"foo":"bar"}`},
			{"a", `test`},
		},
		{
			{"_msg", `abc`},
			{"a", `aiewr`},
		},
	}, [][]Field{
		{
			{"_msg", `abc`},
			{"a", `aiewr`},
		},
	})

	f("offset 2", [][]Field{
		{
			{"_msg", `{"foo":"bar"}`},
			{"a", `test`},
		},
		{
			{"_msg", `sdfsd`},
			{"adffd", `aiewr`},
			{"assdff", "fsf"},
		},
		{
			{"_msg", `abc`},
			{"a", `aiewr`},
			{"asdf", "fsf"},
		},
	}, [][]Field{
		{
			{"_msg", `abc`},
			{"a", `aiewr`},
			{"asdf", "fsf"},
		},
	})
}

func TestPipeOffsetUpdateNeededFields(t *testing.T) {
	f := func(s, neededFields, unneededFields, neededFieldsExpected, unneededFieldsExpected string) {
		t.Helper()
		expectPipeNeededFields(t, s, neededFields, unneededFields, neededFieldsExpected, unneededFieldsExpected)
	}

	// all the needed fields
	f("offset 10", "*", "", "*", "")

	// all the needed fields, plus unneeded fields
	f("offset 10", "*", "f1,f2", "*", "f1,f2")

	// needed fields
	f("offset 10", "f1,f2", "", "f1,f2", "")
}
