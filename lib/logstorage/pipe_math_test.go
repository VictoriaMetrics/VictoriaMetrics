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
	f(`math x as a, z as y`)
	f(`math (foo / bar + baz * abc % -45ms) as a`)
	f(`math (foo / (bar + baz) * abc ^ 2) as a`)
	f(`math (foo / ((bar + baz) * abc) ^ -2) as a`)
	f(`math (foo + bar / baz - abc) as a`)
	f(`math min(3, foo, (1 + bar) / baz) as a, max(a, b) as b, (abs(c) + 5) as d`)
	f(`math round(foo) as x`)
	f(`math round(foo, 0.1) as y`)
	f(`math (a / b default 10) as z`)
	f(`math (ln(a) + exp(b)) as x`)
}

func TestParsePipeMathFailure(t *testing.T) {
	f := func(pipeStr string) {
		t.Helper()
		expectParsePipeFailure(t, pipeStr)
	}

	f(`math`)
	f(`math x as`)
	f(`math abs() as x`)
	f(`math abs(a, b) as x`)
	f(`math min() as x`)
	f(`math min(a) as x`)
	f(`math max() as x`)
	f(`math max(a) as x`)
	f(`math round() as x`)
	f(`math round(a, b, c) as x`)
}

func TestPipeMath(t *testing.T) {
	f := func(pipeStr string, rows, rowsExpected [][]Field) {
		t.Helper()
		expectPipeResults(t, pipeStr, rows, rowsExpected)
	}

	f("math b+1 as a, a*2 as b, b-10.5+c as c", [][]Field{
		{
			{"a", "v1"},
			{"b", "2"},
			{"c", "3"},
		},
	}, [][]Field{
		{
			{"a", "3"},
			{"b", "6"},
			{"c", "-1.5"},
		},
	})

	f("math a / b default c", [][]Field{
		{
			{"a", "v1"},
			{"b", "2"},
			{"c", "3"},
		},
		{
			{"a", "0"},
			{"b", "0"},
			{"c", "3"},
		},
		{
			{"a", "3"},
			{"b", "2"},
		},
		{
			{"a", "3"},
			{"b", "foo"},
		},
	}, [][]Field{
		{
			{"a", "v1"},
			{"b", "2"},
			{"c", "3"},
			{"a / b default c", "3"},
		},
		{
			{"a", "0"},
			{"b", "0"},
			{"c", "3"},
			{"a / b default c", "3"},
		},
		{
			{"a", "3"},
			{"b", "2"},
			{"a / b default c", "1.5"},
		},
		{
			{"a", "3"},
			{"b", "foo"},
			{"a / b default c", "NaN"},
		},
	})

	f("math round(exp(a), 0.01), round(ln(a), 0.01)", [][]Field{
		{
			{"a", "v1"},
		},
		{
			{"a", "0"},
		},
		{
			{"a", "1"},
		},
		{
			{"a", "2"},
		},
		{
			{"a", "3"},
		},
	}, [][]Field{
		{
			{"a", "v1"},
			{"round(exp(a), 0.01)", "NaN"},
			{"round(ln(a), 0.01)", "NaN"},
		},
		{
			{"a", "0"},
			{"round(exp(a), 0.01)", "1"},
			{"round(ln(a), 0.01)", "NaN"},
		},
		{
			{"a", "1"},
			{"round(exp(a), 0.01)", "2.72"},
			{"round(ln(a), 0.01)", "0"},
		},
		{
			{"a", "2"},
			{"round(exp(a), 0.01)", "7.39"},
			{"round(ln(a), 0.01)", "0.69"},
		},
		{
			{"a", "3"},
			{"round(exp(a), 0.01)", "20.09"},
			{"round(ln(a), 0.01)", "1.1"},
		},
	})

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

	f("math 10 * 5 - 3 a", [][]Field{
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

	f("math abs(-min(a,b)) as min, round(max(40*b/30,c)) as max", [][]Field{
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
			{"min", "2"},
			{"max", "3"},
		},
	})

	f("math round((2*c + (b%c))/(c-b)^(b-1), -0.001) as a", [][]Field{
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
			{"a", "3.25"},
			{"b", "3"},
			{"c", "5"},
		},
		{
			{"a", "1.667"},
			{"b", "3"},
			{"c", "6"},
		},
	})
}

func TestPipeMathUpdateNeededFields(t *testing.T) {
	f := func(s string, neededFields, unneededFields, neededFieldsExpected, unneededFieldsExpected string) {
		t.Helper()
		expectPipeNeededFields(t, s, neededFields, unneededFields, neededFieldsExpected, unneededFieldsExpected)
	}

	// all the needed fields
	f("math (x + 1) as y", "*", "", "*", "y")

	// all the needed fields, unneeded fields do not intersect with src and dst
	f("math (x + 1) as y", "*", "f1,f2", "*", "f1,f2,y")

	// all the needed fields, unneeded fields intersect with src
	f("math (x + 1) as y", "*", "f1,x", "*", "f1,y")

	// all the needed fields, unneeded fields intersect with dst
	f("math (x + 1) as y", "*", "f1,y", "*", "f1,y")

	// all the needed fields, unneeded fields intersect with src and dst
	f("math (x + 1) as y", "*", "f1,x,y", "*", "f1,x,y")

	// needed fields do not intersect with src and dst
	f("math (x + 1) as y", "f1,f2", "", "f1,f2", "")

	// needed fields intersect with src
	f("math (x + 1) as y", "f1,x", "", "f1,x", "")

	// needed fields intersect with dst
	f("math (x + 1) as y", "f1,y", "", "f1,x", "")

	// needed fields intersect with src and dst
	f("math (x + 1) as y", "f1,x,y", "", "f1,x", "")
}
