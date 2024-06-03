package logstorage

import (
	"testing"
)

func TestParseStatsRowAnySuccess(t *testing.T) {
	f := func(pipeStr string) {
		t.Helper()
		expectParseStatsFuncSuccess(t, pipeStr)
	}

	f(`row_any(*)`)
	f(`row_any(foo)`)
	f(`row_any(foo, bar)`)
}

func TestParseStatsRowAnyFailure(t *testing.T) {
	f := func(pipeStr string) {
		t.Helper()
		expectParseStatsFuncFailure(t, pipeStr)
	}

	f(`row_any`)
	f(`row_any(x) bar`)
}

func TestStatsRowAny(t *testing.T) {
	f := func(pipeStr string, rows, rowsExpected [][]Field) {
		t.Helper()
		expectPipeResults(t, pipeStr, rows, rowsExpected)
	}

	f("row_any()", [][]Field{
		{
			{"_msg", `abc`},
			{"a", `2`},
			{"b", `3`},
		},
	}, [][]Field{
		{
			{"row_any(*)", `{"_msg":"abc","a":"2","b":"3"}`},
		},
	})

	f("stats row_any(a) as x", [][]Field{
		{
			{"_msg", `abc`},
			{"a", `2`},
			{"b", `3`},
		},
	}, [][]Field{
		{
			{"x", `{"a":"2"}`},
		},
	})

	f("stats row_any(a, x, b) as x", [][]Field{
		{
			{"_msg", `abc`},
			{"a", `2`},
			{"b", `3`},
		},
	}, [][]Field{
		{
			{"x", `{"a":"2","x":"","b":"3"}`},
		},
	})

	f("stats row_any(a) if (b:'') as x", [][]Field{
		{
			{"_msg", `abc`},
			{"a", `2`},
			{"b", `3`},
		},
		{
			{"_msg", `def`},
			{"a", `1`},
		},
	}, [][]Field{
		{
			{"x", `{"a":"1"}`},
		},
	})

	f("stats by (b) row_any(a) if (b:*) as x", [][]Field{
		{
			{"_msg", `abc`},
			{"a", `2`},
			{"b", `3`},
		},
		{
			{"a", `3`},
			{"c", `54`},
		},
	}, [][]Field{
		{
			{"b", "3"},
			{"x", `{"a":"2"}`},
		},
		{
			{"b", ""},
			{"x", `{}`},
		},
	})

	f("stats by (a) row_any(b) as x", [][]Field{
		{
			{"_msg", `abc`},
			{"a", `1`},
			{"b", `3`},
		},
		{
			{"a", `3`},
			{"b", `5`},
		},
	}, [][]Field{
		{
			{"a", "1"},
			{"x", `{"b":"3"}`},
		},
		{
			{"a", "3"},
			{"x", `{"b":"5"}`},
		},
	})

	f("stats by (a) row_any(c) as x", [][]Field{
		{
			{"_msg", `abc`},
			{"a", `1`},
			{"b", `3`},
		},
		{
			{"a", `3`},
			{"c", `foo`},
		},
	}, [][]Field{
		{
			{"a", "1"},
			{"x", `{"c":""}`},
		},
		{
			{"a", "3"},
			{"x", `{"c":"foo"}`},
		},
	})

	f("stats by (a, b) row_any(c) as x", [][]Field{
		{
			{"_msg", `abc`},
			{"a", `1`},
			{"b", `3`},
		},
		{
			{"_msg", `def`},
			{"a", `1`},
			{"c", "foo"},
		},
		{
			{"a", `3`},
			{"b", `5`},
			{"c", "4"},
		},
	}, [][]Field{
		{
			{"a", "1"},
			{"b", "3"},
			{"x", `{"c":""}`},
		},
		{
			{"a", "1"},
			{"b", ""},
			{"x", `{"c":"foo"}`},
		},
		{
			{"a", "3"},
			{"b", "5"},
			{"x", `{"c":"4"}`},
		},
	})
}
