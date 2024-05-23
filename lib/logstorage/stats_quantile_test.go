package logstorage

import (
	"math"
	"testing"
)

func TestParseStatsQuantileSuccess(t *testing.T) {
	f := func(pipeStr string) {
		t.Helper()
		expectParseStatsFuncSuccess(t, pipeStr)
	}

	f(`quantile(0.3)`)
	f(`quantile(1, a)`)
	f(`quantile(0.99, a, b)`)
}

func TestParseStatsQuantileFailure(t *testing.T) {
	f := func(pipeStr string) {
		t.Helper()
		expectParseStatsFuncFailure(t, pipeStr)
	}

	f(`quantile`)
	f(`quantile(a)`)
	f(`quantile(a, b)`)
	f(`quantile(10, b)`)
	f(`quantile(-1, b)`)
	f(`quantile(0.5, b) c`)
}

func TestStatsQuantile(t *testing.T) {
	f := func(pipeStr string, rows, rowsExpected [][]Field) {
		t.Helper()
		expectPipeResults(t, pipeStr, rows, rowsExpected)
	}

	f("stats quantile(0.9) as x", [][]Field{
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
			{"x", "54"},
		},
	})

	f("stats quantile(0.9, a) as x", [][]Field{
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
			{"x", "3"},
		},
	})

	f("stats quantile(0.9, a, b) as x", [][]Field{
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
			{"x", "54"},
		},
	})

	f("stats quantile(0.9, b) as x", [][]Field{
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
			{"x", "54"},
		},
	})

	f("stats quantile(0.9, c) as x", [][]Field{
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

	f("stats quantile(0.9, a) if (b:*) as x", [][]Field{
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
			{"x", "3"},
		},
	})

	f("stats by (b) quantile(0.9, a) if (b:*) as x", [][]Field{
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
			{"x", "2"},
		},
		{
			{"b", ""},
			{"x", "NaN"},
		},
	})

	f("stats by (a) quantile(0.9, b) as x", [][]Field{
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
			{"x", "7"},
		},
	})

	f("stats by (a) quantile(0.9) as x", [][]Field{
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
			{"x", "3"},
		},
		{
			{"a", "3"},
			{"x", "7"},
		},
	})

	f("stats by (a) quantile(0.9, c) as x", [][]Field{
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

	f("stats by (a) quantile(0.9, a, b, c) as x", [][]Field{
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
			{"x", "3"},
		},
		{
			{"a", "3"},
			{"x", "7"},
		},
	})

	f("stats by (a, b) quantile(0.9, a) as x", [][]Field{
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

	f("stats by (a, b) quantile(0.9, c) as x", [][]Field{
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

func TestHistogramQuantile(t *testing.T) {
	f := func(a []float64, phi, qExpected float64) {
		t.Helper()

		var h histogram
		for _, f := range a {
			h.update(f)
		}
		q := h.quantile(phi)

		if math.IsNaN(qExpected) {
			if !math.IsNaN(q) {
				t.Fatalf("unexpected result for q=%v, phi=%v; got %v; want %v", a, phi, q, qExpected)
			}
		} else if q != qExpected {
			t.Fatalf("unexpected result for q=%v, phi=%v; got %v; want %v", a, phi, q, qExpected)
		}
	}

	f(nil, -1, nan)
	f(nil, 0, nan)
	f(nil, 0.5, nan)
	f(nil, 1, nan)
	f(nil, 10, nan)

	f([]float64{123}, -1, 123)
	f([]float64{123}, 0, 123)
	f([]float64{123}, 0.5, 123)
	f([]float64{123}, 1, 123)
	f([]float64{123}, 10, 123)

	f([]float64{5, 1}, -1, 1)
	f([]float64{5, 1}, 0, 1)
	f([]float64{5, 1}, 0.5-1e-5, 1)
	f([]float64{5, 1}, 0.5, 5)
	f([]float64{5, 1}, 1, 5)
	f([]float64{5, 1}, 10, 5)

	f([]float64{5, 1, 3}, -1, 1)
	f([]float64{5, 1, 3}, 0, 1)
	f([]float64{5, 1, 3}, 1.0/3-1e-5, 1)
	f([]float64{5, 1, 3}, 1.0/3, 3)
	f([]float64{5, 1, 3}, 2.0/3-1e-5, 3)
	f([]float64{5, 1, 3}, 2.0/3, 5)
	f([]float64{5, 1, 3}, 1-1e-5, 5)
	f([]float64{5, 1, 3}, 1, 5)
	f([]float64{5, 1, 3}, 10, 5)
}
