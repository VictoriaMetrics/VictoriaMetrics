package logstorage

import (
	"testing"
)

func TestParsePipeStatsSuccess(t *testing.T) {
	f := func(pipeStr string) {
		t.Helper()
		expectParsePipeSuccess(t, pipeStr)
	}

	f(`stats count(*) as rows`)
	f(`stats by (x) count(*) as rows, count_uniq(x) as uniqs`)
	f(`stats by (_time:month offset 6.5h, y) count(*) as rows, count_uniq(x) as uniqs`)
	f(`stats by (_time:month offset 6.5h, y) count(*) if (q:w) as rows, count_uniq(x) as uniqs`)
}

func TestParsePipeStatsFailure(t *testing.T) {
	f := func(pipeStr string) {
		t.Helper()
		expectParsePipeFailure(t, pipeStr)
	}

	f(`stats`)
	f(`stats by`)
	f(`stats foo`)
	f(`stats count`)
	f(`stats if (x:y)`)
	f(`stats by(x) foo`)
	f(`stats by(x:abc) count() rows`)
	f(`stats by(x:1h offset) count () rows`)
	f(`stats by(x:1h offset foo) count() rows`)
}

func TestPipeStats(t *testing.T) {
	f := func(pipeStr string, rows, rowsExpected [][]Field) {
		t.Helper()
		expectPipeResults(t, pipeStr, rows, rowsExpected)
	}

	f("stats count(*) as rows", [][]Field{
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
			{"rows", "3"},
		},
	})

	f("stats by (a) count(*) as rows", [][]Field{
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
			{"a", "1"},
			{"rows", "1"},
		},
		{
			{"a", "2"},
			{"rows", "2"},
		},
	})
}

func TestPipeStatsUpdateNeededFields(t *testing.T) {
	f := func(s, neededFields, unneededFields, neededFieldsExpected, unneededFieldsExpected string) {
		t.Helper()
		expectPipeNeededFields(t, s, neededFields, unneededFields, neededFieldsExpected, unneededFieldsExpected)
	}

	// all the needed fields
	f("stats count() r1", "*", "", "", "")
	f("stats count(*) r1", "*", "", "", "")
	f("stats count(f1,f2) r1", "*", "", "f1,f2", "")
	f("stats count(f1,f2) r1, sum(f3,f4) r2", "*", "", "f1,f2,f3,f4", "")
	f("stats by (b1,b2) count(f1,f2) r1", "*", "", "b1,b2,f1,f2", "")
	f("stats by (b1,b2) count(f1,f2) r1, count(f1,f3) r2", "*", "", "b1,b2,f1,f2,f3", "")

	// all the needed fields, unneeded fields do not intersect with stats fields
	f("stats count() r1", "*", "f1,f2", "", "")
	f("stats count(*) r1", "*", "f1,f2", "", "")
	f("stats count(f1,f2) r1", "*", "f3,f4", "f1,f2", "")
	f("stats count(f1,f2) r1, sum(f3,f4) r2", "*", "f5,f6", "f1,f2,f3,f4", "")
	f("stats by (b1,b2) count(f1,f2) r1", "*", "f3,f4", "b1,b2,f1,f2", "")
	f("stats by (b1,b2) count(f1,f2) r1, count(f1,f3) r2", "*", "f4,f5", "b1,b2,f1,f2,f3", "")

	// all the needed fields, unneeded fields intersect with stats fields
	f("stats count() r1", "*", "r1,r2", "", "")
	f("stats count(*) r1", "*", "r1,r2", "", "")
	f("stats count(f1,f2) r1", "*", "r1,r2", "", "")
	f("stats count(f1,f2) r1, sum(f3,f4) r2", "*", "r1,r3", "f3,f4", "")
	f("stats by (b1,b2) count(f1,f2) r1", "*", "r1,r2", "b1,b2", "")
	f("stats by (b1,b2) count(f1,f2) r1", "*", "r1,r2,b1", "b1,b2", "")
	f("stats by (b1,b2) count(f1,f2) r1", "*", "r1,r2,b1,b2", "b1,b2", "")
	f("stats by (b1,b2) count(f1,f2) r1, count(f1,f3) r2", "*", "r1,r3", "b1,b2,f1,f3", "")

	// needed fields do not intersect with stats fields
	f("stats count() r1", "r2", "", "", "")
	f("stats count(*) r1", "r2", "", "", "")
	f("stats count(f1,f2) r1", "r2", "", "", "")
	f("stats count(f1,f2) r1, sum(f3,f4) r2", "r3", "", "", "")
	f("stats by (b1,b2) count(f1,f2) r1", "r2", "", "b1,b2", "")
	f("stats by (b1,b2) count(f1,f2) r1, count(f1,f3) r2", "r3", "", "b1,b2", "")

	// needed fields intersect with stats fields
	f("stats count() r1", "r1,r2", "", "", "")
	f("stats count(*) r1", "r1,r2", "", "", "")
	f("stats count(f1,f2) r1", "r1,r2", "", "f1,f2", "")
	f("stats count(f1,f2) r1, sum(f3,f4) r2", "r1,r3", "", "f1,f2", "")
	f("stats by (b1,b2) count(f1,f2) r1", "r1,r2", "", "b1,b2,f1,f2", "")
	f("stats by (b1,b2) count(f1,f2) r1, count(f1,f3) r2", "r1,r3", "", "b1,b2,f1,f2", "")
}
