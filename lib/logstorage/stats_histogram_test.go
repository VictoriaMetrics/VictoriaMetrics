package logstorage

import (
	"testing"
)

func TestParseStatsHistogramSuccess(t *testing.T) {
	f := func(pipeStr string) {
		t.Helper()
		expectParseStatsFuncSuccess(t, pipeStr)
	}

	f(`histogram(foo)`)
}

func TestParseStatsHistogramFailure(t *testing.T) {
	f := func(pipeStr string) {
		t.Helper()
		expectParseStatsFuncFailure(t, pipeStr)
	}

	f(`histogram`)
	f(`histogram(a, b)`)
	f(`histogram(a) abc`)
}

func TestStatsHistogram(t *testing.T) {
	f := func(pipeStr string, rows, rowsExpected [][]Field) {
		t.Helper()
		expectPipeResults(t, pipeStr, rows, rowsExpected)
	}

	f("stats histogram(a) as x", [][]Field{
		{
			{"_msg", `abc`},
			{"a", `2`},
			{"b", `3`},
		},
		{
			{"_msg", `def`},
			{"a", `1.9`},
		},
		{
			{"a", `3.05`},
			{"b", `54`},
		},
	}, [][]Field{
		{
			{"x", `[{"vmrange":"1.896e+00...2.154e+00","hits":2},{"vmrange":"2.783e+00...3.162e+00","hits":1}]`},
		},
	})
}
