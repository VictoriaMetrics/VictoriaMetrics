package logstorage

import (
	"testing"
)

func TestParsePipeTopSuccess(t *testing.T) {
	f := func(pipeStr string) {
		t.Helper()
		expectParsePipeSuccess(t, pipeStr)
	}

	f(`top by (x)`)
	f(`top 5 by (x)`)
	f(`top by (x, y)`)
	f(`top 5 by (x, y)`)
	f(`top by (x) rank`)
	f(`top by (x) rank as foo`)
	f(`top by (x) hits as abc`)
}

func TestParsePipeTopFailure(t *testing.T) {
	f := func(pipeStr string) {
		t.Helper()
		expectParsePipeFailure(t, pipeStr)
	}

	f(`top 5 foo bar`)
	f(`top 5 foo,`)
	f(`top 5 by`)
	f(`top 5 by (`)
	f(`top 5foo bar`)
	f(`top foo bar`)
	f(`top by`)
	f(`top (x) rank a b`)
	f(`top (x) hits`)
	f(`top`)
	f(`top rank`)
	f(`top ()`)
	f(`top (*)`)
	f(`top (a*)`)
}

func TestPipeTop(t *testing.T) {
	f := func(pipeStr string, rows, rowsExpected [][]Field) {
		t.Helper()
		expectPipeResults(t, pipeStr, rows, rowsExpected)
	}

	f("top by (a)", [][]Field{
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

	f("top b hits abc", [][]Field{
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
			{"abc", "2"},
		},
		{
			{"b", "54"},
			{"abc", "1"},
		},
	})

	f("top by (b) rank as x", [][]Field{
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
			{"x", "1"},
		},
		{
			{"b", "54"},
			{"hits", "1"},
			{"x", "2"},
		},
	})

	f("top by (hits)", [][]Field{
		{
			{"a", `2`},
			{"hits", `3`},
		},
		{
			{"a", "2"},
			{"hits", "3"},
		},
		{
			{"a", `2`},
			{"hits", `54`},
			{"c", "d"},
		},
	}, [][]Field{
		{
			{"hits", "3"},
			{"hitss", "2"},
		},
		{
			{"hits", "54"},
			{"hitss", "1"},
		},
	})

	f("top by (c)", [][]Field{
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

	f("top by (d)", [][]Field{
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

	f("top by (a, b)", [][]Field{
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
			{"b", "3"},
			{"hits", "2"},
		},
		{
			{"a", "2"},
			{"b", "54"},
			{"hits", "1"},
		},
	})

	f("top 10 by a, b", [][]Field{
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
			{"b", "3"},
			{"hits", "2"},
		},
		{
			{"a", "2"},
			{"b", "54"},
			{"hits", "1"},
		},
	})

	f("top 1 a, b", [][]Field{
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
			{"b", "3"},
			{"hits", "2"},
		},
	})
}

func TestPipeTopUpdateNeededFields(t *testing.T) {
	f := func(s, allowFilters, denyFilters, allowFiltersExpected, denyFiltersExpected string) {
		t.Helper()
		expectPipeNeededFields(t, s, allowFilters, denyFilters, allowFiltersExpected, denyFiltersExpected)
	}

	// all the needed fields
	f("top by(x)", "*", "", "x", "")
	f("top by(f1,f2)", "*", "", "f1,f2", "")

	// all the needed fields, unneeded fields do not intersect with src
	f("top by(s1, s2)", "*", "f1,f2", "s1,s2", "")
	f("top by(s1, s2)", "*", "f*", "s1,s2", "")

	// all the needed fields, unneeded fields intersect with src
	f("top by(s1, s2)", "*", "s1,f1,f2", "s1,s2", "")
	f("top by(s1, s2)", "*", "s1,s2,f1", "s1,s2", "")
	f("top by(s1, s2)", "*", "s*,f*", "s1,s2", "")

	// needed fields do not intersect with src
	f("top by (s1, s2)", "f1,f2", "", "s1,s2", "")
	f("top by (s1, s2)", "f*", "", "s1,s2", "")

	// needed fields intersect with src
	f("top by (s1, s2)", "s1,f1,f2", "", "s1,s2", "")
	f("top by (s1, s2)", "s*,f*", "", "s1,s2", "")
}
