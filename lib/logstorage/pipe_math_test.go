package logstorage

import (
	"testing"
)

func TestParsePipeMathSuccess(t *testing.T) {
	f := func(pipeStr string) {
		t.Helper()
		expectParsePipeSuccess(t, pipeStr)
	}

	f(`math a = b`)
	f(`math a = -123`)
	f(`math a = 12.345KB`)
	f(`math a = -2 + 2`)
	f(`math a = x, y = z`)
	f(`math a = foo / bar + baz * abc % -45ms`)
	f(`math a = foo / (bar + baz) * abc ^ 2`)
	f(`math a = foo / ((bar + baz) * abc) ^ -2`)
	f(`math a = foo + bar / baz - abc`)
}

func TestParsePipeMathFailure(t *testing.T) {
	f := func(pipeStr string) {
		t.Helper()
		expectParsePipeFailure(t, pipeStr)
	}

	f(`math`)
	f(`math x`)
	f(`math x y`)
	f(`math x =`)
	f(`math x = (`)
	f(`math x = a +`)
	f(`math x = a + (`)
	f(`math x = a + )`)
}

func TestPipeMath(t *testing.T) {
	f := func(pipeStr string, rows, rowsExpected [][]Field) {
		t.Helper()
		expectPipeResults(t, pipeStr, rows, rowsExpected)
	}

	f("math a = 1", [][]Field{
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

	f("math a = 10 * 5 - 3", [][]Field{
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

	f("math a = -1.5K", [][]Field{
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

	f("math a = b", [][]Field{
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

	f("math a = a", [][]Field{
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

	f("math x = 2*c + b", [][]Field{
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

	f("math a = (2*c + (b%c))/(c-b)^(b-1)", [][]Field{
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
