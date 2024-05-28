package logstorage

import (
	"testing"
)

func TestParsePipeMathSuccess(t *testing.T) {
	f := func(pipeStr string) {
		t.Helper()
		expectParsePipeSuccess(t, pipeStr)
	}

	f(`math b as a`)
	f(`math -123 as a`)
	f(`math 12.345KB as a`)
	f(`math (-2 + 2) as a`)
	f(`math min(3, foo, (1 + bar) / baz) as a, max(a, b) as b, (abs(c) + 5) as d`)
	f(`math x as a, z as y`)
	f(`math (foo / bar + baz * abc % -45ms) as a`)
	f(`math (foo / (bar + baz) * abc ^ 2) as a`)
	f(`math (foo / ((bar + baz) * abc) ^ -2) as a`)
	f(`math (foo + bar / baz - abc) as a`)
}

func TestParsePipeMathFailure(t *testing.T) {
	f := func(pipeStr string) {
		t.Helper()
		expectParsePipeFailure(t, pipeStr)
	}

	f(`math`)
	f(`math x`)
	f(`math x y`)
	f(`math x as`)
	f(`math abs() as x`)
	f(`math abs(a, b) as x`)
	f(`math min() as x`)
	f(`math min(a) as x`)
	f(`math max() as x`)
	f(`math max(a) as x`)
}

func TestPipeMath(t *testing.T) {
	f := func(pipeStr string, rows, rowsExpected [][]Field) {
		t.Helper()
		expectPipeResults(t, pipeStr, rows, rowsExpected)
	}

	f("math 1 as a", [][]Field{
		{
			{"a", "v1"},
			{"b", "2"},
			{"c", "3"},
		},
	}, [][]Field{
		{
			{"a", "1"},
			{"b", "2"},
			{"c", "3"},
		},
	})

	f("math 10 * 5 - 3 as a", [][]Field{
		{
			{"a", "v1"},
			{"b", "2"},
			{"c", "3"},
		},
	}, [][]Field{
		{
			{"a", "47"},
			{"b", "2"},
			{"c", "3"},
		},
	})

	f("math -1.5K as a", [][]Field{
		{
			{"a", "v1"},
			{"b", "2"},
			{"c", "3"},
		},
	}, [][]Field{
		{
			{"a", "-1500"},
			{"b", "2"},
			{"c", "3"},
		},
	})

	f("math b as a", [][]Field{
		{
			{"a", "v1"},
			{"b", "2"},
			{"c", "3"},
		},
	}, [][]Field{
		{
			{"a", "2"},
			{"b", "2"},
			{"c", "3"},
		},
	})

	f("math a as a", [][]Field{
		{
			{"a", "v1"},
			{"b", "2"},
			{"c", "3"},
		},
	}, [][]Field{
		{
			{"a", "NaN"},
			{"b", "2"},
			{"c", "3"},
		},
	})

	f("math 2*c + b as x", [][]Field{
		{
			{"a", "v1"},
			{"b", "2"},
			{"c", "3"},
		},
	}, [][]Field{
		{
			{"a", "v1"},
			{"b", "2"},
			{"c", "3"},
			{"x", "8"},
		},
	})

	f("math (2*c + (b%c))/(c-b)^(b-1) as a", [][]Field{
		{
			{"a", "v"},
			{"b", "2"},
			{"c", "3"},
		},
		{
			{"a", "x"},
			{"b", "3"},
			{"c", "5"},
		},
		{
			{"b", "3"},
			{"c", "6"},
		},
	}, [][]Field{
		{
			{"a", "8"},
			{"b", "2"},
			{"c", "3"},
		},
		{
			{"a", "42.25"},
			{"b", "3"},
			{"c", "5"},
		},
		{
			{"a", "25"},
			{"b", "3"},
			{"c", "6"},
		},
	})
}
