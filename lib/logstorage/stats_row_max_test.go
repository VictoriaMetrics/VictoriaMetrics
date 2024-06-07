package logstorage

import (
	"testing"
)

func TestParseStatsRowMaxSuccess(t *testing.T) {
	f := func(pipeStr string) {
		t.Helper()
		expectParseStatsFuncSuccess(t, pipeStr)
	}

	f(`row_max(foo)`)
	f(`row_max(foo, bar)`)
	f(`row_max(foo, bar, baz)`)
}

func TestParseStatsRowMaxFailure(t *testing.T) {
	f := func(pipeStr string) {
		t.Helper()
		expectParseStatsFuncFailure(t, pipeStr)
	}

	f(`row_max`)
	f(`row_max()`)
	f(`row_max(x) bar`)
}

func TestStatsRowMax(t *testing.T) {
	f := func(pipeStr string, rows, rowsExpected [][]Field) {
		t.Helper()
		expectPipeResults(t, pipeStr, rows, rowsExpected)
	}

	f("stats row_max(a) as x", [][]Field{
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
			{"x", `{"a":"3","b":"54"}`},
		},
	})

	f("stats row_max(foo) as x", [][]Field{
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
			{"x", `{}`},
		},
	})

	f("stats row_max(b, a) as x", [][]Field{
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
			{"c", "1232"},
		},
	}, [][]Field{
		{
			{"x", `{"a":"3"}`},
		},
	})

	f("stats row_max(b, a, x, b) as x", [][]Field{
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
			{"c", "1232"},
		},
	}, [][]Field{
		{
			{"x", `{"a":"3","x":"","b":"54"}`},
		},
	})

	f("stats row_max(a) if (b:*) as x", [][]Field{
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
			{"x", `{"a":"3","b":"54"}`},
		},
	})

	f("stats by (b) row_max(a) if (b:*) as x", [][]Field{
		{
			{"_msg", `abc`},
			{"a", `2`},
			{"b", `3`},
		},
		{
			{"_msg", `def`},
			{"a", `-12.34`},
			{"b", "3"},
		},
		{
			{"a", `3`},
			{"c", `54`},
		},
	}, [][]Field{
		{
			{"b", "3"},
			{"x", `{"_msg":"abc","a":"2","b":"3"}`},
		},
		{
			{"b", ""},
			{"x", `{}`},
		},
	})

	f("stats by (a) row_max(b) as x", [][]Field{
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
			{"x", `{"_msg":"abc","a":"1","b":"3"}`},
		},
		{
			{"a", "3"},
			{"x", `{"a":"3","b":"7"}`},
		},
	})

	f("stats by (a) row_max(c) as x", [][]Field{
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
			{"c", `foo`},
		},
		{
			{"a", `3`},
			{"b", `7`},
		},
	}, [][]Field{
		{
			{"a", "1"},
			{"x", `{}`},
		},
		{
			{"a", "3"},
			{"x", `{"a":"3","c":"foo"}`},
		},
	})

	f("stats by (a) row_max(b, c) as x", [][]Field{
		{
			{"_msg", `abc`},
			{"a", `1`},
			{"b", `34`},
		},
		{
			{"_msg", `def`},
			{"a", `1`},
			{"c", "3"},
		},
		{
			{"a", `3`},
			{"b", `5`},
			{"c", "foo"},
		},
		{
			{"a", `3`},
			{"b", `7`},
			{"c", "bar"},
		},
	}, [][]Field{
		{
			{"a", "1"},
			{"x", `{"c":""}`},
		},
		{
			{"a", "3"},
			{"x", `{"c":"bar"}`},
		},
	})

	f("stats by (a, b) row_max(c) as x", [][]Field{
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
			{"x", `{}`},
		},
		{
			{"a", "1"},
			{"b", ""},
			{"x", `{"_msg":"def","a":"1","c":"foo"}`},
		},
		{
			{"a", "3"},
			{"b", "5"},
			{"x", `{"a":"3","b":"5","c":"4"}`},
		},
	})
}
