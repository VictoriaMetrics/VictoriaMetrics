package logstorage

import (
	"testing"
)

func TestParsePipeDecolorizeSuccess(t *testing.T) {
	f := func(pipeStr string) {
		t.Helper()
		expectParsePipeSuccess(t, pipeStr)
	}

	f(`decolorize`)
	f(`decolorize foo`)
}

func TestParsePipeDecolorizeFailure(t *testing.T) {
	f := func(pipeStr string) {
		t.Helper()
		expectParsePipeFailure(t, pipeStr)
	}

	f(`decolorize *`)
	f(`decolorize foo*`)
	f(`decolorize (foo)`)
	f(`decolorize foo, bar`)
}

func TestPipeDecolorize(t *testing.T) {
	f := func(pipeStr string, rows, rowsExpected [][]Field) {
		t.Helper()
		expectPipeResults(t, pipeStr, rows, rowsExpected)
	}

	// decolorize _msg
	f(`decolorize`, [][]Field{
		{
			{"_msg", "\x1b[mfoo\x1b[1;31mERROR bar\x1b[10;5H"},
			{"bar", `cde`},
		},
		{
			{"_msg", `a_bc_def`},
		},
	}, [][]Field{
		{
			{"_msg", `fooERROR bar`},
			{"bar", `cde`},
		},
		{
			{"_msg", `a_bc_def`},
		},
	})

	// decolorize non-_msg field
	f(`decolorize bar`, [][]Field{
		{
			{"bar", "\x1b[mfoo\x1b[1;31mERROR bar\x1b[10;5H"},
			{"_msg", `cde`},
		},
		{
			{"bar", `a_bc_def`},
		},
	}, [][]Field{
		{
			{"bar", `fooERROR bar`},
			{"_msg", `cde`},
		},
		{
			{"bar", `a_bc_def`},
		},
	})
}

func TestPipeDecolorizeUpdateNeededFields(t *testing.T) {
	f := func(s string, allowFilters, denyFilters, allowFiltersExpected, denyFiltersExpected string) {
		t.Helper()
		expectPipeNeededFields(t, s, allowFilters, denyFilters, allowFiltersExpected, denyFiltersExpected)
	}

	// all the needed fields
	f(`decolorize x`, "*", "", "*", "")

	// unneeded fields do not intersect with field
	f(`decolorize x`, "*", "f1,f2", "*", "f1,f2")

	// unneeded fields intersect with field
	f(`decolorize x`, "*", "x,y", "*", "x,y")

	// needed fields do not intersect with field
	f(`decolorize x`, "f2,y", "", "f2,y", "")

	// needed fields intersect with field
	f(`decolorize y`, "f2,y", "", "f2,y", "")
}
