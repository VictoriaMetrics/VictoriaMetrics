package logstorage

import (
	"testing"
)

func TestParsePipeJSONArrayLenSuccess(t *testing.T) {
	f := func(pipeStr string) {
		t.Helper()
		expectParsePipeSuccess(t, pipeStr)
	}

	f(`json_array_len(foo)`)
	f(`json_array_len(foo) as bar`)
}

func TestParsePipeJSONArrayLenFailure(t *testing.T) {
	f := func(pipeStr string) {
		t.Helper()
		expectParsePipeFailure(t, pipeStr)
	}

	f(`json_array_len`)
	f(`json_array_len(`)
	f(`json_array_len()`)
	f(`json_array_len(x) y z`)
}

func TestPipeJSONArrayLen(t *testing.T) {
	f := func(pipeStr string, rows, rowsExpected [][]Field) {
		t.Helper()
		expectPipeResults(t, pipeStr, rows, rowsExpected)
	}

	f(`json_array_len(foo) x`, [][]Field{
		{
			{"foo", `["abcde",2,{"bar":"x,y","z":[1,2]}]`},
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
			{"foo", `["abcde",2,{"bar":"x,y","z":[1,2]}]`},
			{"baz", "1234567890"},
			{"x", "3"},
		},
		{
			{"foo", `abc`},
			{"bar", `de`},
			{"x", "0"},
		},
		{
			{"baz", "xyz"},
			{"x", "0"},
		},
	})
}

func TestPipeJSONArrayLenUpdateNeededFields(t *testing.T) {
	f := func(s string, neededFields, unneededFields, neededFieldsExpected, unneededFieldsExpected string) {
		t.Helper()
		expectPipeNeededFields(t, s, neededFields, unneededFields, neededFieldsExpected, unneededFieldsExpected)
	}

	// all the needed fields
	f(`json_array_len(y) x`, "*", "", "*", "x")
	f(`json_array_len(x) x`, "*", "", "*", "")

	// unneeded fields do not intersect with output field
	f(`json_array_len(y) as x`, "*", "f1,f2", "*", "f1,f2,x")
	f(`json_array_len(x) as x`, "*", "f1,f2", "*", "f1,f2")

	// unneeded fields intersect with output field
	f(`json_array_len(z) as x`, "*", "x,y", "*", "x,y")
	f(`json_array_len(y) as x`, "*", "x,y", "*", "x,y")
	f(`json_array_len(x) as x`, "*", "x,y", "*", "x,y")

	// needed fields do not intersect with output field
	f(`json_array_len(y) as z`, "x,y", "", "x,y", "")
	f(`json_array_len(z) as z`, "x,y", "", "x,y", "")

	// needed fields intersect with output field
	f(`json_array_len (z) as f2`, "f2,y", "", "y,z", "")
	f(`json_array_len y as f2`, "f2,y", "", "y", "")
	f(`json_array_len y as y`, "f2,y", "", "f2,y", "")
}
