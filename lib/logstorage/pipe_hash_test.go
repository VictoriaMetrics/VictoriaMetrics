package logstorage

import (
	"testing"
)

func TestParsePipeHashSuccess(t *testing.T) {
	f := func(pipeStr string) {
		t.Helper()
		expectParsePipeSuccess(t, pipeStr)
	}

	f(`hash(foo)`)
	f(`hash(foo) as bar`)
}

func TestParsePipeHashFailure(t *testing.T) {
	f := func(pipeStr string) {
		t.Helper()
		expectParsePipeFailure(t, pipeStr)
	}

	f(`hash`)
	f(`hash(`)
	f(`hash()`)
	f(`hash(x) y z`)
}

func TestPipeHash(t *testing.T) {
	f := func(pipeStr string, rows, rowsExpected [][]Field) {
		t.Helper()
		expectPipeResults(t, pipeStr, rows, rowsExpected)
	}

	f(`hash(foo) x`, [][]Field{
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
			{"x", "957726378018795"},
		},
		{
			{"foo", `abc`},
			{"bar", `de`},
			{"x", "7930733036767641"},
		},
		{
			{"baz", "xyz"},
			{"x", "1929880503118233"},
		},
	})
}

func TestPipeHashUpdateNeededFields(t *testing.T) {
	f := func(s string, neededFields, unneededFields, neededFieldsExpected, unneededFieldsExpected string) {
		t.Helper()
		expectPipeNeededFields(t, s, neededFields, unneededFields, neededFieldsExpected, unneededFieldsExpected)
	}

	// all the needed fields
	f(`hash(y) x`, "*", "", "*", "x")
	f(`hash(x) x`, "*", "", "*", "")

	// unneeded fields do not intersect with output field
	f(`hash(y) as x`, "*", "f1,f2", "*", "f1,f2,x")
	f(`hash(x) as x`, "*", "f1,f2", "*", "f1,f2")

	// unneeded fields intersect with output field
	f(`hash(z) as x`, "*", "x,y", "*", "x,y")
	f(`hash(y) as x`, "*", "x,y", "*", "x,y")
	f(`hash(x) as x`, "*", "x,y", "*", "x,y")

	// needed fields do not intersect with output field
	f(`hash(y) as z`, "x,y", "", "x,y", "")
	f(`hash(z) as z`, "x,y", "", "x,y", "")

	// needed fields intersect with output field
	f(`hash (z) as f2`, "f2,y", "", "y,z", "")
	f(`hash (y) as f2`, "f2,y", "", "y", "")
	f(`hash (y) as y`, "f2,y", "", "f2,y", "")
}
