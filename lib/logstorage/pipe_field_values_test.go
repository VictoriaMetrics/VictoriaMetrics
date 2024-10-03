package logstorage

import (
	"testing"
)

func TestParsePipeFieldValuesSuccess(t *testing.T) {
	f := func(pipeStr string) {
		t.Helper()
		expectParsePipeSuccess(t, pipeStr)
	}

	f(`field_values x`)
	f(`field_values x limit 10`)
}

func TestParsePipeFieldValuesFailure(t *testing.T) {
	f := func(pipeStr string) {
		t.Helper()
		expectParsePipeFailure(t, pipeStr)
	}

	f(`field_values`)
	f(`field_values a b`)
	f(`field_values a limit`)
	f(`field_values limit N`)
}

func TestPipeFieldValues(t *testing.T) {
	f := func(pipeStr string, rows, rowsExpected [][]Field) {
		t.Helper()
		expectPipeResults(t, pipeStr, rows, rowsExpected)
	}

	f("field_values a", [][]Field{
		{
			{"a", `2`},
			{"b", `3`},
		},
		{
			{"a", "2"},
			{"b", "3"},
		},
		{
			{"a", `2`},
			{"b", `54`},
			{"c", "d"},
		},
	}, [][]Field{
		{
			{"a", "2"},
			{"hits", "3"},
		},
	})

	f("field_values (b)", [][]Field{
		{
			{"a", `2`},
			{"b", `3`},
		},
		{
			{"a", "2"},
			{"b", "3"},
		},
		{
			{"a", `2`},
			{"b", `54`},
			{"c", "d"},
		},
	}, [][]Field{
		{
			{"b", "3"},
			{"hits", "2"},
		},
		{
			{"b", "54"},
			{"hits", "1"},
		},
	})

	f("field_values c", [][]Field{
		{
			{"a", `2`},
			{"b", `3`},
		},
		{
			{"a", "2"},
			{"b", "3"},
		},
		{
			{"a", `2`},
			{"b", `54`},
			{"c", "d"},
		},
	}, [][]Field{
		{
			{"c", ""},
			{"hits", "2"},
		},
		{
			{"c", "d"},
			{"hits", "1"},
		},
	})

	f("field_values d", [][]Field{
		{
			{"a", `2`},
			{"b", `3`},
		},
		{
			{"a", "2"},
			{"b", "3"},
		},
		{
			{"a", `2`},
			{"b", `54`},
			{"c", "d"},
		},
	}, [][]Field{
		{
			{"d", ""},
			{"hits", "3"},
		},
	})
}

func TestPipeFieldValuesUpdateNeededFields(t *testing.T) {
	f := func(s, neededFields, unneededFields, neededFieldsExpected, unneededFieldsExpected string) {
		t.Helper()
		expectPipeNeededFields(t, s, neededFields, unneededFields, neededFieldsExpected, unneededFieldsExpected)
	}

	// all the needed fields
	f("field_values x", "*", "", "x", "")

	// all the needed fields, unneeded fields do not intersect with src
	f("field_values x", "*", "f1,f2", "x", "")

	// all the needed fields, unneeded fields intersect with src
	f("field_values x", "*", "f1,x", "", "")

	// needed fields do not intersect with src
	f("field_values x", "f1,f2", "", "", "")

	// needed fields intersect with src
	f("field_values x", "f1,x", "", "x", "")
}
