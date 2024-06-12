package logstorage

import (
	"testing"
)

func TestParsePipeSortSuccess(t *testing.T) {
	f := func(pipeStr string) {
		t.Helper()
		expectParsePipeSuccess(t, pipeStr)
	}

	f(`sort`)
	f(`sort by (x)`)
	f(`sort by (x) limit 10`)
	f(`sort by (x) offset 20 limit 10`)
	f(`sort by (x desc, y) desc`)
}

func TestParsePipeSortFailure(t *testing.T) {
	f := func(pipeStr string) {
		t.Helper()
		expectParsePipeFailure(t, pipeStr)
	}

	f(`sort a`)
	f(`sort by`)
	f(`sort by(x) foo`)
	f(`sort by(x) limit`)
	f(`sort by(x) limit N`)
	f(`sort by(x) offset`)
	f(`sort by(x) offset N`)
}

func TestPipeSort(t *testing.T) {
	f := func(pipeStr string, rows, rowsExpected [][]Field) {
		t.Helper()
		expectPipeResults(t, pipeStr, rows, rowsExpected)
	}

	// Sort by all fields
	f("sort", [][]Field{
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
		{
			{"_msg", `def`},
			{"a", `1`},
		},
	})

	// Sort by a single field
	f("sort by (a asc) asc", [][]Field{
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
		{
			{"_msg", `abc`},
			{"a", `2`},
		},
	})

	// Sort by a in descending order
	f("sort by (a) desc", [][]Field{
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
		{
			{"_msg", `def`},
			{"a", `1`},
		},
	})

	// Sort by multiple fields
	f("sort by (a, b desc) desc", [][]Field{
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
		{
			{"a", `2`},
			{"b", `54`},
		},
		{
			{"_msg", `def`},
			{"a", `1`},
			{"b", ""},
		},
	})

	// Sort by multiple fields with limit
	f("sort by (a, b) limit 1", [][]Field{
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

	// Sort by multiple fields with limit desc
	f("sort by (a, b) desc limit 1", [][]Field{
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
	})

	// Sort by multiple fields with offset
	f("sort by (a, b) offset 1", [][]Field{
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
		{
			{"a", `2`},
			{"b", `54`},
		},
	})

	// Sort by multiple fields with offset and limit
	f("sort by (a, b) offset 1 limit 1", [][]Field{
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

	// Sort by multiple fields with offset and limit
	f("sort by (a, b) desc offset 2 limit 100", [][]Field{
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
}

func TestPipeSortUpdateNeededFields(t *testing.T) {
	f := func(s, neededFields, unneededFields, neededFieldsExpected, unneededFieldsExpected string) {
		t.Helper()
		expectPipeNeededFields(t, s, neededFields, unneededFields, neededFieldsExpected, unneededFieldsExpected)
	}

	// all the needed fields
	f("sort by(s1,s2)", "*", "", "*", "")

	// all the needed fields, unneeded fields do not intersect with src
	f("sort by(s1,s2)", "*", "f1,f2", "*", "f1,f2")

	// all the needed fields, unneeded fields intersect with src
	f("sort by(s1,s2)", "*", "s1,f1,f2", "*", "f1,f2")

	// needed fields do not intersect with src
	f("sort by(s1,s2)", "f1,f2", "", "s1,s2,f1,f2", "")

	// needed fields intersect with src
	f("sort by(s1,s2)", "s1,f1,f2", "", "s1,s2,f1,f2", "")
}
