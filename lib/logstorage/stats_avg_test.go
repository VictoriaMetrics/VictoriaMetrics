package logstorage

import (
	"testing"
)

func TestParseStatsAvgSuccess(t *testing.T) {
	f := func(pipeStr string) {
		t.Helper()
		expectParseStatsFuncSuccess(t, pipeStr)
	}

	f(`avg(*)`)
	f(`avg(a)`)
	f(`avg(a, b)`)
}

func TestParseStatsAvgFailure(t *testing.T) {
	f := func(pipeStr string) {
		t.Helper()
		expectParseStatsFuncFailure(t, pipeStr)
	}

	f(`avg`)
	f(`avg(a b)`)
	f(`avg(x) y`)
}

func TestStatsAvg(t *testing.T) {
	f := func(pipeStr string, rows, rowsExpected [][]Field) {
		t.Helper()
		expectPipeResults(t, pipeStr, rows, rowsExpected)
	}

	f("stats avg(*) as x", [][]Field{
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
			{"x", "12.6"},
		},
	})

	f("stats avg(a) as x", [][]Field{
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
			{"x", "2"},
		},
	})

	f("stats avg(a) as a", [][]Field{
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
			{"a", "2"},
		},
	})

	f("stats avg(a, b) as x", [][]Field{
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
			{"x", "12.6"},
		},
	})

	f("stats avg(b) as x", [][]Field{
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
			{"x", "28.5"},
		},
	})

	f("stats avg(c) as x", [][]Field{
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
			{"x", "NaN"},
		},
	})

	f("stats avg(a) if (b:*) as x", [][]Field{
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
			{"x", "2.5"},
		},
	})

	f("stats by (b) avg(a) if (b:*) as x", [][]Field{
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
			{"x", "1.5"},
		},
		{
			{"b", ""},
			{"x", "NaN"},
		},
	})

	f("stats by (a) avg(b) as x", [][]Field{
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
			{"x", "3"},
		},
		{
			{"a", "3"},
			{"x", "6"},
		},
	})

	f("stats by (a) avg(*) as x", [][]Field{
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
			{"x", "2"},
		},
		{
			{"a", "3"},
			{"x", "4.5"},
		},
	})

	f("stats by (a) avg(c) as x", [][]Field{
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
			{"x", "NaN"},
		},
		{
			{"a", "3"},
			{"x", "5"},
		},
	})

	f("stats by (a) avg(a, b, c) as x", [][]Field{
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
			{"x", "2"},
		},
		{
			{"a", "3"},
			{"x", "4.5"},
		},
	})

	f("stats by (a, b) avg(a) as x", [][]Field{
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
			{"x", "1"},
		},
		{
			{"a", "1"},
			{"b", ""},
			{"x", "1"},
		},
		{
			{"a", "3"},
			{"b", "5"},
			{"x", "3"},
		},
	})

	f("stats by (a, b) avg(c) as x", [][]Field{
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
			{"x", "NaN"},
		},
		{
			{"a", "1"},
			{"b", ""},
			{"x", "3"},
		},
		{
			{"a", "3"},
			{"b", "5"},
			{"x", "NaN"},
		},
	})
}

func expectParseStatsFuncFailure(t *testing.T, s string) {
	t.Helper()

	lex := newLexer(s)
	sf, err := parseStatsFunc(lex)
	if err == nil && lex.isEnd() {
		t.Fatalf("expecting error when parsing [%s]; parsed result: [%s]", s, sf)
	}
}

func expectParseStatsFuncSuccess(t *testing.T, s string) {
	t.Helper()

	lex := newLexer(s)
	p, err := parseStatsFunc(lex)
	if err != nil {
		t.Fatalf("cannot parse [%s]: %s", s, err)
	}
	if !lex.isEnd() {
		t.Fatalf("unexpected tail after parsing [%s]: [%s]", s, lex.s)
	}

	sResult := p.String()
	if sResult != s {
		t.Fatalf("unexpected string representation of stats func; got\n%s\nwant\n%s", sResult, s)
	}
}
