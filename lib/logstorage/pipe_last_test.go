package logstorage

import (
	"testing"
)

func TestParsePipeLastSuccess(t *testing.T) {
	f := func(pipeStr string) {
		t.Helper()
		expectParsePipeSuccess(t, pipeStr)
	}

	f(`last`)
	f(`last rank`)
	f(`last rank as foo`)
	f(`last by (x)`)
	f(`last by (x, y) rank`)
	f(`last 10 by (x)`)
	f(`last 10 by (x) rank as bar`)
	f(`last partition by (x)`)
	f(`last 5 partition by (x)`)
	f(`last 5 by (y, z) partition by (x, a) rank as bar`)
}

func TestParsePipeLastFailure(t *testing.T) {
	f := func(pipeStr string) {
		t.Helper()
		expectParsePipeFailure(t, pipeStr)
	}

	f(`last a`)
	f(`last by`)
	f(`last by(x) foo`)
	f(`last rank by (x)`)
	f(`last partition`)
	f(`last partition by`)
}

func TestPipeLast(t *testing.T) {
	f := func(pipeStr string, rows, rowsExpected [][]Field) {
		t.Helper()
		expectPipeResults(t, pipeStr, rows, rowsExpected)
	}

	// Sort by all fields
	f("last", [][]Field{
		{
			{"_msg", `def`},
			{"a", `1`},
		},
		{
			{"_msg", `abc`},
			{"a", `2`},
		},
	}, [][]Field{
		{
			{"_msg", `def`},
			{"a", `1`},
		},
	})

	// Sort by all fields with rank
	f("last rank x", [][]Field{
		{
			{"_msg", `def`},
			{"a", `1`},
		},
		{
			{"_msg", `abc`},
			{"a", `2`},
		},
	}, [][]Field{
		{
			{"_msg", `def`},
			{"a", `1`},
			{"x", "1"},
		},
	})

	// Sort by a single field
	f("last by (a asc)", [][]Field{
		{
			{"_msg", `abc`},
			{"a", `2`},
		},
		{
			{"_msg", `def`},
			{"a", `1`},
		},
	}, [][]Field{
		{
			{"_msg", `abc`},
			{"a", `2`},
		},
	})

	// Sort by a in descending order
	f("last by (a desc)", [][]Field{
		{
			{"_msg", `abc`},
			{"a", `2`},
		},
		{
			{"_msg", `def`},
			{"a", `1`},
		},
	}, [][]Field{
		{
			{"_msg", `def`},
			{"a", `1`},
		},
	})

	// Sort by multiple fields
	f("last by (a, b desc)", [][]Field{
		{
			{"_msg", `abc`},
			{"a", `2`},
			{"b", `3`},
		},
		{
			{"_msg", `def`},
			{"a", `1`},
		},
		{
			{"a", `2`},
			{"b", `54`},
		},
	}, [][]Field{
		{
			{"_msg", `abc`},
			{"a", `2`},
			{"b", `3`},
		},
	})

	// Sort by multiple fields with limit
	f("last 2 by (a, b)", [][]Field{
		{
			{"_msg", `abc`},
			{"a", `2`},
			{"b", `3`},
		},
		{
			{"_msg", `def`},
			{"a", `1`},
		},
		{
			{"a", `2`},
			{"b", `54`},
		},
	}, [][]Field{
		{
			{"a", `2`},
			{"b", `54`},
		},
		{
			{"_msg", `abc`},
			{"a", `2`},
			{"b", `3`},
		},
	})

	// Sort with partition
	f(`last by (a) partition by (b)`, [][]Field{
		{
			{"a", "foo"},
			{"b", "x"},
		},
		{
			{"a", "bar"},
			{"b", "x"},
		},
		{
			{"a", "xyz"},
			{"b", "abc"},
		},
	}, [][]Field{
		{
			{"a", "foo"},
			{"b", "x"},
		},
		{
			{"a", "xyz"},
			{"b", "abc"},
		},
	})
}

func TestPipeLastUpdateNeededFields(t *testing.T) {
	f := func(s, neededFields, unneededFields, neededFieldsExpected, unneededFieldsExpected string) {
		t.Helper()
		expectPipeNeededFields(t, s, neededFields, unneededFields, neededFieldsExpected, unneededFieldsExpected)
	}

	// all the needed fields
	f("last", "*", "", "*", "")
	f("last rank x", "*", "", "*", "")
	f("last 10 by(s1,s2)", "*", "", "*", "")
	f("last 3 by(s1,s2) rank as x", "*", "", "*", "x")
	f("last 3 by(s1,s2) partition by (x) rank as x", "*", "", "*", "")
	f("last 3 by(x,s2) rank as x", "*", "", "*", "")

	// all the needed fields, unneeded fields do not intersect with src
	f("last by(s1,s2)", "*", "f1,f2", "*", "f1,f2")
	f("last by(s1,s2) rank as x", "*", "f1,f2", "*", "f1,f2,x")
	f("last by(x,s2) rank as x", "*", "f1,f2", "*", "f1,f2")

	// all the needed fields, unneeded fields intersect with src
	f("last by(s1,s2)", "*", "s1,f1,f2", "*", "f1,f2")
	f("last by(s1,s2) rank as x", "*", "s1,f1,f2", "*", "f1,f2,x")
	f("last by(x,s2) rank as x", "*", "s1,f1,f2", "*", "f1,f2,s1")

	// needed fields do not intersect with src
	f("last by(s1,s2)", "f1,f2", "", "s1,s2,f1,f2", "")
	f("last by(s1,s2) rank as x", "f1,f2", "", "s1,s2,f1,f2", "")

	// needed fields intersect with src
	f("last by(s1,s2)", "s1,f1,f2", "", "s1,s2,f1,f2", "")
	f("last by(s1,s2) rank as x", "s1,f1,f2,x", "", "s1,s2,f1,f2", "")
}
