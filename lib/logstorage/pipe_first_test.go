package logstorage

import (
	"testing"
)

func TestParsePipeFirstSuccess(t *testing.T) {
	f := func(pipeStr string) {
		t.Helper()
		expectParsePipeSuccess(t, pipeStr)
	}

	f(`first`)
	f(`first rank`)
	f(`first rank as foo`)
	f(`first by (x)`)
	f(`first by (x) rank`)
	f(`first 10 by (x)`)
	f(`first 10 by (x) rank as bar`)
	f(`first partition by (x)`)
	f(`first 3 by (a, b) partition by (x, y)`)
}

func TestParsePipeFirstFailure(t *testing.T) {
	f := func(pipeStr string) {
		t.Helper()
		expectParsePipeFailure(t, pipeStr)
	}

	f(`first a`)
	f(`first by`)
	f(`first by(x) foo`)
	f(`first by (x) partition`)
	f(`first by (x) partition by`)
}

func TestPipeFirst(t *testing.T) {
	f := func(pipeStr string, rows, rowsExpected [][]Field) {
		t.Helper()
		expectPipeResults(t, pipeStr, rows, rowsExpected)
	}

	// Sort by all fields
	f("first", [][]Field{
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
			{"_msg", `abc`},
			{"a", `2`},
		},
	})

	// Sort by all fields with rank
	f("first rank x", [][]Field{
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
			{"_msg", `abc`},
			{"a", `2`},
			{"x", "1"},
		},
	})

	// Sort by a single field
	f("first by (a asc)", [][]Field{
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

	// Sort by a in descending order
	f("first by (a desc)", [][]Field{
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

	// Sort by multiple fields
	f("first by (a, b desc)", [][]Field{
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
			{"_msg", `def`},
			{"a", `1`},
			{"b", ""},
		},
	})

	// Sort by multiple fields with limit
	f("first 2 by (a, b)", [][]Field{
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
			{"_msg", `def`},
			{"a", `1`},
			{"b", ""},
		},
		{
			{"_msg", `abc`},
			{"a", `2`},
			{"b", `3`},
		},
	})

	// Sort with partition
	f(`first by (a) partition by (b)`, [][]Field{
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
			{"a", "xyz"},
			{"b", "abc"},
		},
		{
			{"a", "bar"},
			{"b", "x"},
		},
	})
}

func TestPipeFirstUpdateNeededFields(t *testing.T) {
	f := func(s, neededFields, unneededFields, neededFieldsExpected, unneededFieldsExpected string) {
		t.Helper()
		expectPipeNeededFields(t, s, neededFields, unneededFields, neededFieldsExpected, unneededFieldsExpected)
	}

	// all the needed fields
	f("first", "*", "", "*", "")
	f("first rank x", "*", "", "*", "")
	f("first 10 by(s1,s2)", "*", "", "*", "")
	f("first 3 by(s1,s2) rank as x", "*", "", "*", "x")
	f("first 3 by(s1,s2) partition by (x, y) rank as x", "*", "", "*", "")
	f("first 3 by(x,s2) rank as x", "*", "", "*", "")

	// all the needed fields, unneeded fields do not intersect with src
	f("first by(s1,s2)", "*", "f1,f2", "*", "f1,f2")
	f("first by(s1,s2) rank as x", "*", "f1,f2", "*", "f1,f2,x")
	f("first by(x,s2) rank as x", "*", "f1,f2", "*", "f1,f2")

	// all the needed fields, unneeded fields intersect with src
	f("first by(s1,s2)", "*", "s1,f1,f2", "*", "f1,f2")
	f("first by(s1,s2) rank as x", "*", "s1,f1,f2", "*", "f1,f2,x")
	f("first by(x,s2) rank as x", "*", "s1,f1,f2", "*", "f1,f2,s1")

	// needed fields do not intersect with src
	f("first by(s1,s2)", "f1,f2", "", "s1,s2,f1,f2", "")
	f("first by(s1,s2) rank as x", "f1,f2", "", "s1,s2,f1,f2", "")

	// needed fields intersect with src
	f("first by(s1,s2)", "s1,f1,f2", "", "s1,s2,f1,f2", "")
	f("first by(s1,s2) rank as x", "s1,f1,f2,x", "", "s1,s2,f1,f2", "")
}
