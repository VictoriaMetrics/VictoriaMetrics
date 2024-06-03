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

	// missing 'stats' keyword and resutl name
	f("count(*)", [][]Field{
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
			{`count(*)`, "3"},
		},
	})

	// missing 'stats' keyword
	f("count() as rows, count() if (a:2) rows2", [][]Field{
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
			{"rows2", "2"},
		},
	})

	f("stats count() as rows, count() if (a:2) rows2", [][]Field{
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
			{"rows2", "2"},
		},
	})

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
		{},
		{
			{"a", `2`},
			{"b", `54`},
		},
		{},
	}, [][]Field{
		{
			{"rows", "5"},
		},
	})

	f("stats count(b) as rows", [][]Field{
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
			{"rows", "2"},
		},
	})

	f("stats count(x) as rows", [][]Field{
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
			{"rows", "0"},
		},
	})

	f("stats count(x, _msg, b) as rows", [][]Field{
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

	// missing 'stats' keyword
	f("by (a) count(*) as rows", [][]Field{
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

	f("stats by (a) count(*) if (b:54) as rows", [][]Field{
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
			{"rows", "0"},
		},
		{
			{"a", "2"},
			{"rows", "1"},
		},
	})

	f("stats by (a, x) count(*) if (b:54) as rows_b54, count(*) as rows_total", [][]Field{
		{
			{"_msg", `abc`},
			{"a", `2`},
			{"b", `3`},
		},
		{
			{"_msg", `def`},
			{"a", `1`},
			{"x", "123"},
		},
		{
			{"a", `2`},
			{"b", `54`},
		},
	}, [][]Field{
		{
			{"a", "1"},
			{"x", "123"},
			{"rows_b54", "0"},
			{"rows_total", "1"},
		},
		{
			{"a", "2"},
			{"x", ""},
			{"rows_b54", "1"},
			{"rows_total", "2"},
		},
	})

	f("stats by (x:1KiB) count(*) as rows", [][]Field{
		{
			{"x", "1023"},
			{"_msg", "foo"},
		},
		{
			{"x", "1024"},
			{"_msg", "bar"},
		},
		{
			{"x", "2047"},
			{"_msg", "baz"},
		},
	}, [][]Field{
		{
			{"x", "0"},
			{"rows", "1"},
		},
		{
			{"x", "1024"},
			{"rows", "2"},
		},
	})

	f("stats by (ip:/24) count(*) as rows", [][]Field{
		{
			{"ip", "1.2.3.4"},
		},
		{
			{"ip", "1.2.3.255"},
		},
		{
			{"ip", "127.2.3.4"},
		},
		{
			{"ip", "1.2.4.0"},
		},
	}, [][]Field{
		{
			{"ip", "1.2.3.0"},
			{"rows", "2"},
		},
		{
			{"ip", "1.2.4.0"},
			{"rows", "1"},
		},
		{
			{"ip", "127.2.3.0"},
			{"rows", "1"},
		},
	})

	f("stats by (_time:1d) count(*) as rows", [][]Field{
		{
			{"_time", "2024-04-01T10:20:30Z"},
			{"a", `2`},
			{"b", `3`},
		},
		{
			{"_time", "2024-04-02T10:20:30Z"},
			{"a", "1"},
		},
		{
			{"_time", "2024-04-02T10:20:30Z"},
			{"a", "2"},
			{"b", `54`},
		},
		{
			{"_time", "2024-04-02T10:20:30Z"},
			{"a", "2"},
			{"c", `xyz`},
		},
	}, [][]Field{
		{
			{"_time", "2024-04-01T00:00:00Z"},
			{"rows", "1"},
		},
		{
			{"_time", "2024-04-02T00:00:00Z"},
			{"rows", "3"},
		},
	})

	f("stats by (_time:1d offset 2h) count(*) as rows", [][]Field{
		{
			{"_time", "2024-04-01T00:20:30Z"},
			{"a", `2`},
			{"b", `3`},
		},
		{
			{"_time", "2024-04-02T22:20:30Z"},
			{"a", "1"},
		},
		{
			{"_time", "2024-04-02T10:20:30Z"},
			{"a", "2"},
			{"b", `54`},
		},
		{
			{"_time", "2024-04-03T01:59:59.999999999Z"},
			{"a", "2"},
			{"c", `xyz`},
		},
	}, [][]Field{
		{
			{"_time", "2024-03-31T02:00:00Z"},
			{"rows", "1"},
		},
		{
			{"_time", "2024-04-02T02:00:00Z"},
			{"rows", "3"},
		},
	})

	f("stats by (a, _time:1d) count(*) as rows", [][]Field{
		{
			{"_time", "2024-04-01T10:20:30Z"},
			{"a", `2`},
			{"b", `3`},
		},
		{
			{"_time", "2024-04-02T10:20:30Z"},
			{"a", "1"},
		},
		{
			{"_time", "2024-04-02T10:20:30Z"},
			{"a", "2"},
			{"b", `54`},
		},
		{
			{"_time", "2024-04-02T10:20:30Z"},
			{"a", "2"},
			{"c", `xyz`},
		},
	}, [][]Field{
		{
			{"a", "2"},
			{"_time", "2024-04-01T00:00:00Z"},
			{"rows", "1"},
		},
		{
			{"a", "1"},
			{"_time", "2024-04-02T00:00:00Z"},
			{"rows", "1"},
		},
		{
			{"a", "2"},
			{"_time", "2024-04-02T00:00:00Z"},
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
