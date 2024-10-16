package logstorage

import (
	"testing"
)

func TestParsePipeLenSuccess(t *testing.T) {
	f := func(pipeStr string) {
		t.Helper()
		expectParsePipeSuccess(t, pipeStr)
	}

	f(`len(foo)`)
	f(`len(foo) as bar`)
}

func TestParsePipeLenFailure(t *testing.T) {
	f := func(pipeStr string) {
		t.Helper()
		expectParsePipeFailure(t, pipeStr)
	}

	f(`len`)
	f(`len(`)
	f(`len()`)
	f(`len(x) y z`)
}

func TestPipeLen(t *testing.T) {
	f := func(pipeStr string, rows, rowsExpected [][]Field) {
		t.Helper()
		expectPipeResults(t, pipeStr, rows, rowsExpected)
	}

	f(`len(foo) x`, [][]Field{
		{
			{"foo", `abcde`},
			{"baz", "1234567890"},
		},
		{
			{"foo", `abc`},
			{"bar", `de`},
		},
		{
			{"baz", "xyz"},
		},
	}, [][]Field{
		{
			{"foo", `abcde`},
			{"baz", "1234567890"},
			{"x", "5"},
		},
		{
			{"foo", `abc`},
			{"bar", `de`},
			{"x", "3"},
		},
		{
			{"baz", "xyz"},
			{"x", "0"},
		},
	})
}

func TestPipeLenUpdateNeededFields(t *testing.T) {
	f := func(s string, neededFields, unneededFields, neededFieldsExpected, unneededFieldsExpected string) {
		t.Helper()
		expectPipeNeededFields(t, s, neededFields, unneededFields, neededFieldsExpected, unneededFieldsExpected)
	}

	// all the needed fields
	f(`len(y) x`, "*", "", "*", "x")
	f(`len(x) x`, "*", "", "*", "")

	// unneeded fields do not intersect with output field
	f(`len(y) as x`, "*", "f1,f2", "*", "f1,f2,x")
	f(`len(x) as x`, "*", "f1,f2", "*", "f1,f2")

	// unneeded fields intersect with output field
	f(`len(z) as x`, "*", "x,y", "*", "x,y")
	f(`len(y) as x`, "*", "x,y", "*", "x,y")
	f(`len(x) as x`, "*", "x,y", "*", "x,y")

	// needed fields do not intersect with output field
	f(`len(y) as z`, "x,y", "", "x,y", "")
	f(`len(z) as z`, "x,y", "", "x,y", "")

	// needed fields intersect with output field
	f(`len (z) as f2`, "f2,y", "", "y,z", "")
	f(`len (y) as f2`, "f2,y", "", "y", "")
	f(`len (y) as y`, "f2,y", "", "f2,y", "")
}
