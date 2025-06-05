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
	f(`pack_logfmt fields (foo.*, bar, baz.abc.*) as x`)
}

func TestParsePipePackLogfmtFailure(t *testing.T) {
	f := func(pipeStr string) {
		t.Helper()
		expectParsePipeFailure(t, pipeStr)
	}

	f(`pack_logfmt foo bar`)
	f(`pack_logfmt fields`)
	f(`pack_logfmt as *`)
	f(`pack_logfmt as x*`)
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
			{"a", `foo=abc`},
		},
		{
			{"a", ""},
			{"c", "d"},
		},
	})

	// pack only the needed wildcard fields
	f(`pack_logfmt fields (x*,y) a`, [][]Field{
		{
			{"x", `abc`},
			{"xx", `xabc`},
			{"yy", `cde`},
			{"y", `xcde`},
		},
	}, [][]Field{
		{
			{"x", `abc`},
			{"xx", `xabc`},
			{"yy", `cde`},
			{"y", `xcde`},
			{"a", `x=abc xx=xabc y=xcde`},
		},
	})
}

func TestPipePackLogfmtUpdateNeededFields(t *testing.T) {
	f := func(s string, allowFilters, denyFilters, allowFiltersExpected, denyFiltersExpected string) {
		t.Helper()
		expectPipeNeededFields(t, s, allowFilters, denyFilters, allowFiltersExpected, denyFiltersExpected)
	}

	// all the needed fields
	f(`pack_logfmt as x`, "*", "", "*", "")
	f(`pack_logfmt fields (a,b) as x`, "*", "", "*", "x")
	f(`pack_logfmt fields (x) as x`, "*", "", "*", "")
	f(`pack_logfmt fields (x*,y)`, "*", "", "*", "_msg")

	// unneeded fields do not intersect with output
	f(`pack_logfmt as x`, "*", "f1,f2", "*", "")
	f(`pack_logfmt as x`, "*", "f*", "*", "")
	f(`pack_logfmt fields(f1,f3) as x`, "*", "f1,f2", "*", "f2,x")
	f(`pack_logfmt fields(f1,f3,x) as x`, "*", "f1,f2", "*", "f2")
	f(`pack_logfmt fields(f*,y) as x`, "*", "f1,f2", "*", "x")
	f(`pack_logfmt fields(f1,f3) as x`, "*", "f*", "*", "x")

	// unneeded fields intersect with output
	f(`pack_logfmt as f1`, "*", "f1,f2", "*", "f1,f2")
	f(`pack_logfmt as f1`, "*", "f*", "*", "f*")
	f(`pack_logfmt fields (f2,f3) as f1`, "*", "f1,f2", "*", "f1,f2")
	f(`pack_logfmt fields (f*,y) as f1`, "*", "f1,f2", "*", "f1,f2")
	f(`pack_logfmt fields (f*,y) as f1`, "*", "f*", "*", "f*")

	// needed fields do not intersect with output
	f(`pack_logfmt f1`, "x,y", "", "x,y", "")
	f(`pack_logfmt f1`, "x*", "", "x*", "")
	f(`pack_logfmt fields (x,z) f1`, "x,y", "", "x,y", "")
	f(`pack_logfmt fields (x*,z) f1`, "x,y", "", "x,y", "")
	f(`pack_logfmt fields (x*,z) f1`, "x*", "", "x*", "")

	// needed fields intersect with output
	f(`pack_logfmt as f2`, "f2,y", "", "*", "")
	f(`pack_logfmt as f2`, "f*,y", "", "*", "")
	f(`pack_logfmt fields (x,y) as f2`, "f2,y", "", "x,y", "")
	f(`pack_logfmt fields (x*,y) as f2`, "f2,y", "", "x*,y", "")
	f(`pack_logfmt fields (x*,y) as f2`, "f*,y", "", "f*,x*,y", "f2")
}
