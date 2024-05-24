package logstorage

import (
	"strings"
	"testing"
)

func TestParseStatsUniqValuesSuccess(t *testing.T) {
	f := func(pipeStr string) {
		t.Helper()
		expectParseStatsFuncSuccess(t, pipeStr)
	}

	f(`uniq_values(*)`)
	f(`uniq_values(a)`)
	f(`uniq_values(a, b)`)
	f(`uniq_values(a, b) limit 10`)
}

func TestParseStatsUniqValuesFailure(t *testing.T) {
	f := func(pipeStr string) {
		t.Helper()
		expectParseStatsFuncFailure(t, pipeStr)
	}

	f(`uniq_values`)
	f(`uniq_values(a b)`)
	f(`uniq_values(x) y`)
	f(`uniq_values(x) limit`)
	f(`uniq_values(x) limit N`)
}

func TestStatsUniqValues(t *testing.T) {
	f := func(pipeStr string, rows, rowsExpected [][]Field) {
		t.Helper()
		expectPipeResults(t, pipeStr, rows, rowsExpected)
	}

	f("stats uniq_values(*) as x", [][]Field{
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
			{"a", `-3`},
			{"b", `54`},
		},
	}, [][]Field{
		{
			{"x", `["-3","1","2","3","54","abc","def"]`},
		},
	})

	f("stats uniq_values(*) limit 1999 as x", [][]Field{
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
			{"a", `-3`},
			{"b", `54`},
		},
	}, [][]Field{
		{
			{"x", `["-3","1","2","3","54","abc","def"]`},
		},
	})

	f("stats uniq_values(a) as x", [][]Field{
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
			{"a", `3`},
			{"b", `54`},
		},
	}, [][]Field{
		{
			{"x", `["1","2","3"]`},
		},
	})

	f("stats uniq_values(a, b) as x", [][]Field{
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
			{"a", `3`},
			{"b", `54`},
		},
	}, [][]Field{
		{
			{"x", `["1","2","3","54"]`},
		},
	})

	f("stats uniq_values(b) as x", [][]Field{
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
			{"a", `3`},
			{"b", `54`},
		},
	}, [][]Field{
		{
			{"x", `["3","54"]`},
		},
	})

	f("stats uniq_values(c) as x", [][]Field{
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
			{"a", `3`},
			{"b", `54`},
		},
	}, [][]Field{
		{
			{"x", `[]`},
		},
	})

	f("stats uniq_values(a) if (b:*) as x", [][]Field{
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
			{"a", `3`},
			{"b", `54`},
		},
	}, [][]Field{
		{
			{"x", `["2","3"]`},
		},
	})

	f("stats by (b) uniq_values(a) if (b:*) as x", [][]Field{
		{
			{"_msg", `abc`},
			{"a", `2`},
			{"b", `3`},
		},
		{
			{"_msg", `def`},
			{"a", `1`},
			{"b", "3"},
		},
		{
			{"a", `3`},
			{"c", `54`},
		},
	}, [][]Field{
		{
			{"b", "3"},
			{"x", `["1","2"]`},
		},
		{
			{"b", ""},
			{"x", `[]`},
		},
	})

	f("stats by (a) uniq_values(b) as x", [][]Field{
		{
			{"_msg", `abc`},
			{"a", `1`},
			{"b", `3`},
		},
		{
			{"_msg", `def`},
			{"a", `1`},
		},
		{
			{"a", `3`},
			{"b", `5`},
		},
		{
			{"a", `3`},
			{"b", `7`},
		},
	}, [][]Field{
		{
			{"a", "1"},
			{"x", `["3"]`},
		},
		{
			{"a", "3"},
			{"x", `["5","7"]`},
		},
	})

	f("stats by (a) uniq_values(*) as x", [][]Field{
		{
			{"_msg", `abc`},
			{"a", `1`},
			{"b", `3`},
		},
		{
			{"_msg", `def`},
			{"a", `1`},
			{"c", "3"},
		},
		{
			{"a", `3`},
			{"b", `5`},
		},
		{
			{"a", `3`},
			{"b", `7`},
		},
	}, [][]Field{
		{
			{"a", "1"},
			{"x", `["1","3","abc","def"]`},
		},
		{
			{"a", "3"},
			{"x", `["3","5","7"]`},
		},
	})

	f("stats by (a) uniq_values(*) limit 100 as x", [][]Field{
		{
			{"_msg", `abc`},
			{"a", `1`},
			{"b", `3`},
		},
		{
			{"_msg", `def`},
			{"a", `1`},
			{"c", "3"},
		},
		{
			{"a", `3`},
			{"b", `5`},
		},
		{
			{"a", `3`},
			{"b", `7`},
		},
	}, [][]Field{
		{
			{"a", "1"},
			{"x", `["1","3","abc","def"]`},
		},
		{
			{"a", "3"},
			{"x", `["3","5","7"]`},
		},
	})

	f("stats by (a) uniq_values(c) as x", [][]Field{
		{
			{"_msg", `abc`},
			{"a", `1`},
			{"b", `3`},
		},
		{
			{"_msg", `def`},
			{"a", `1`},
		},
		{
			{"a", `3`},
			{"c", `5`},
		},
		{
			{"a", `3`},
			{"b", `7`},
		},
	}, [][]Field{
		{
			{"a", "1"},
			{"x", `[]`},
		},
		{
			{"a", "3"},
			{"x", `["5"]`},
		},
	})

	f("stats by (a) uniq_values(a, b, c) as x", [][]Field{
		{
			{"_msg", `abc`},
			{"a", `1`},
			{"b", `3`},
		},
		{
			{"_msg", `def`},
			{"a", `1`},
			{"c", "3"},
		},
		{
			{"a", `3`},
			{"b", `5`},
		},
		{
			{"a", `3`},
			{"b", `7`},
		},
	}, [][]Field{
		{
			{"a", "1"},
			{"x", `["1","3"]`},
		},
		{
			{"a", "3"},
			{"x", `["3","5","7"]`},
		},
	})

	f("stats by (a, b) uniq_values(a) as x", [][]Field{
		{
			{"_msg", `abc`},
			{"a", `1`},
			{"b", `3`},
		},
		{
			{"_msg", `def`},
			{"a", `1`},
			{"c", "3"},
		},
		{
			{"a", `3`},
			{"b", `5`},
		},
	}, [][]Field{
		{
			{"a", "1"},
			{"b", "3"},
			{"x", `["1"]`},
		},
		{
			{"a", "1"},
			{"b", ""},
			{"x", `["1"]`},
		},
		{
			{"a", "3"},
			{"b", "5"},
			{"x", `["3"]`},
		},
	})

	f("stats by (a, b) uniq_values(c) as x", [][]Field{
		{
			{"_msg", `abc`},
			{"a", `1`},
			{"b", `3`},
		},
		{
			{"_msg", `def`},
			{"a", `1`},
			{"c", "3"},
		},
		{
			{"a", `3`},
			{"b", `5`},
		},
	}, [][]Field{
		{
			{"a", "1"},
			{"b", "3"},
			{"x", `[]`},
		},
		{
			{"a", "1"},
			{"b", ""},
			{"x", `["3"]`},
		},
		{
			{"a", "3"},
			{"b", "5"},
			{"x", `[]`},
		},
	})
}

func TestSortStrings(t *testing.T) {
	f := func(s, resultExpected string) {
		t.Helper()

		a := strings.Split(s, ",")
		sortStrings(a)
		result := strings.Join(a, ",")
		if result != resultExpected {
			t.Fatalf("unexpected sort result\ngot\n%q\nwant\n%q", a, resultExpected)
		}
	}

	f("", "")
	f("1", "1")
	f("foo,bar,baz", "bar,baz,foo")
	f("100ms,1.5s,1.23s", "100ms,1.23s,1.5s")
	f("10KiB,10KB,5.34K", "5.34K,10KB,10KiB")
	f("v1.10.9,v1.10.10,v1.9.0", "v1.9.0,v1.10.9,v1.10.10")
	f("10s,123,100M", "123,100M,10s")
}
