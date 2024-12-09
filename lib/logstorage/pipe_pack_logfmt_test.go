package logstorage

import (
	"testing"
)

func TestParsePipePackLogfmtSuccess(t *testing.T) {
	f := func(pipeStr string) {
		t.Helper()
		expectParsePipeSuccess(t, pipeStr)
	}

	f(`pack_logfmt`)
	f(`pack_logfmt as x`)
	f(`pack_logfmt fields (a, b)`)
	f(`pack_logfmt fields (a, b) as x`)
}

func TestParsePipePackLogfmtFailure(t *testing.T) {
	f := func(pipeStr string) {
		t.Helper()
		expectParsePipeFailure(t, pipeStr)
	}

	f(`pack_logfmt foo bar`)
	f(`pack_logfmt fields`)
}

func TestPipePackLogfmt(t *testing.T) {
	f := func(pipeStr string, rows, rowsExpected [][]Field) {
		t.Helper()
		expectPipeResults(t, pipeStr, rows, rowsExpected)
	}

	// pack into _msg
	f(`pack_logfmt`, [][]Field{
		{
			{"_msg", "x"},
			{"foo", `abc`},
			{"bar", `cde=ab`},
		},
		{
			{"a", "b"},
			{"c", "d"},
		},
	}, [][]Field{
		{
			{"_msg", `_msg=x foo=abc bar=cde=ab`},
			{"foo", `abc`},
			{"bar", `cde=ab`},
		},
		{
			{"_msg", `a=b c=d`},
			{"a", "b"},
			{"c", "d"},
		},
	})

	// pack into other field
	f(`pack_logfmt as a`, [][]Field{
		{
			{"_msg", "x"},
			{"foo", `abc`},
			{"bar", `cde`},
		},
		{
			{"a", "b"},
			{"c", "d"},
		},
	}, [][]Field{
		{
			{"_msg", `x`},
			{"foo", `abc`},
			{"bar", `cde`},
			{"a", `_msg=x foo=abc bar=cde`},
		},
		{
			{"a", `a=b c=d`},
			{"c", "d"},
		},
	})

	// pack only the needed fields
	f(`pack_logfmt fields (foo, baz) a`, [][]Field{
		{
			{"_msg", "x"},
			{"foo", `abc`},
			{"bar", `cde`},
		},
		{
			{"a", "b"},
			{"c", "d"},
		},
	}, [][]Field{
		{
			{"_msg", `x`},
			{"foo", `abc`},
			{"bar", `cde`},
			{"a", `foo=abc baz=`},
		},
		{
			{"a", `foo= baz=`},
			{"c", "d"},
		},
	})
}

func TestPipePackLogfmtUpdateNeededFields(t *testing.T) {
	f := func(s string, neededFields, unneededFields, neededFieldsExpected, unneededFieldsExpected string) {
		t.Helper()
		expectPipeNeededFields(t, s, neededFields, unneededFields, neededFieldsExpected, unneededFieldsExpected)
	}

	// all the needed fields
	f(`pack_logfmt as x`, "*", "", "*", "")
	f(`pack_logfmt fields (a,b) as x`, "*", "", "*", "")

	// unneeded fields do not intersect with output
	f(`pack_logfmt as x`, "*", "f1,f2", "*", "")
	f(`pack_logfmt fields(f1,f3) as x`, "*", "f1,f2", "*", "f2")

	// unneeded fields intersect with output
	f(`pack_logfmt as f1`, "*", "f1,f2", "*", "f1,f2")
	f(`pack_logfmt fields (f2,f3) as f1`, "*", "f1,f2", "*", "f1,f2")

	// needed fields do not intersect with output
	f(`pack_logfmt f1`, "x,y", "", "x,y", "")
	f(`pack_logfmt fields (x,z) f1`, "x,y", "", "x,y", "")

	// needed fields intersect with output
	f(`pack_logfmt as f2`, "f2,y", "", "*", "")
	f(`pack_logfmt fields (x,y) as f2`, "f2,y", "", "x,y", "")
}
