package logstorage

import (
	"testing"
)

func TestParsePipeUnrollTokensSuccess(t *testing.T) {
	f := func(pipeStr string) {
		t.Helper()
		expectParsePipeSuccess(t, pipeStr)
	}

	f(`unroll_tokens`)
	f(`unroll_tokens as bar`)
	f(`unroll_tokens foo`)
	f(`unroll_tokens foo as bar`)
}

func TestParsePipeUrollTokensFailure(t *testing.T) {
	f := func(pipeStr string) {
		t.Helper()
		expectParsePipeFailure(t, pipeStr)
	}

	f(`unroll_tokens as`)
	f(`unroll_tokens foo bar baz`)
	f(`unroll_tokens foo, bar`)
}

func TestPipeUnrollTokens(t *testing.T) {
	f := func(pipeStr string, rows, rowsExpected [][]Field) {
		t.Helper()
		expectPipeResults(t, pipeStr, rows, rowsExpected)
	}

	// unroll_tokens by missing field
	f("unroll_tokens x", [][]Field{
		{
			{"a", `["foo",1,{"baz":"x"},[1,2],null,NaN]`},
			{"q", "w"},
		},
	}, nil)

	// unroll_tokens by a field without tokens
	f("unroll_tokens q", [][]Field{
		{
			{"a", `["foo",1,{"baz":"x"},[1,2],null,NaN]`},
			{"q", "!#$%,"},
		},
	}, nil)

	// unroll_tokens by a field with tokens
	f("unroll_tokens a", [][]Field{
		{
			{"a", `foo,bar baz`},
			{"q", "w"},
		},
		{
			{"a", "b"},
			{"c", "d"},
		},
	}, [][]Field{
		{
			{"a", "foo"},
			{"q", "w"},
		},
		{
			{"a", "bar"},
			{"q", "w"},
		},
		{
			{"a", `baz`},
			{"q", "w"},
		},
		{
			{"a", "b"},
			{"c", "d"},
		},
	})

	// unroll_tokens by a field with tokens into another field
	f("unroll_tokens a as b", [][]Field{
		{
			{"a", `foo,bar baz`},
			{"q", "w"},
		},
		{
			{"a", "b"},
			{"c", "d"},
		},
	}, [][]Field{
		{
			{"a", `foo,bar baz`},
			{"b", "foo"},
			{"q", "w"},
		},
		{
			{"a", `foo,bar baz`},
			{"b", "bar"},
			{"q", "w"},
		},
		{
			{"a", `foo,bar baz`},
			{"b", `baz`},
			{"q", "w"},
		},
		{
			{"a", "b"},
			{"b", "b"},
			{"c", "d"},
		},
	})

	// unroll_tokens from _msg inplace
	f("unroll_tokens", [][]Field{
		{
			{"_msg", `foo,bar baz`},
			{"q", "w"},
		},
		{
			{"_msg", "b"},
			{"c", "d"},
		},
	}, [][]Field{
		{
			{"_msg", "foo"},
			{"q", "w"},
		},
		{
			{"_msg", "bar"},
			{"q", "w"},
		},
		{
			{"_msg", `baz`},
			{"q", "w"},
		},
		{
			{"_msg", "b"},
			{"c", "d"},
		},
	})

	// unroll_tokens from _msg into other field
	f("unroll_tokens as b", [][]Field{
		{
			{"_msg", `foo,bar foo`},
			{"q", "w"},
		},
		{
			{"_msg", "b"},
			{"c", "d"},
		},
	}, [][]Field{
		{
			{"_msg", `foo,bar foo`},
			{"b", "foo"},
			{"q", "w"},
		},
		{
			{"_msg", `foo,bar foo`},
			{"b", "bar"},
			{"q", "w"},
		},
		{
			{"_msg", `foo,bar foo`},
			{"b", `foo`},
			{"q", "w"},
		},
		{
			{"_msg", "b"},
			{"b", "b"},
			{"c", "d"},
		},
	})
}

func TestPipeUnrollTokensUpdateNeededFields(t *testing.T) {
	f := func(s string, neededFields, unneededFields, neededFieldsExpected, unneededFieldsExpected string) {
		t.Helper()
		expectPipeNeededFields(t, s, neededFields, unneededFields, neededFieldsExpected, unneededFieldsExpected)
	}

	// all the needed fields
	f("unroll_tokens x", "*", "", "*", "")
	f("unroll_tokens x y", "*", "", "*", "y")

	// all the needed fields, unneeded fields do not intersect with src
	f("unroll_tokens x", "*", "f1,f2", "*", "f1,f2")
	f("unroll_tokens x as y", "*", "f1,f2", "*", "f1,f2,y")

	// all the needed fields, unneeded fields intersect with src
	f("unroll_tokens x", "*", "f2,x", "*", "f2")
	f("unroll_tokens x y", "*", "f2,x", "*", "f2,y")
	f("unroll_tokens x y", "*", "f2,y", "*", "f2,y")

	// needed fields do not intersect with src
	f("unroll_tokens x", "f1,f2", "", "f1,f2,x", "")
	f("unroll_tokens x y", "f1,f2", "", "f1,f2,x", "")

	// needed fields intersect with src
	f("unroll_tokens x", "f2,x", "", "f2,x", "")
	f("unroll_tokens x y", "f2,x", "", "f2,x", "")
	f("unroll_tokens x y", "f2,y", "", "f2,x", "")
}
