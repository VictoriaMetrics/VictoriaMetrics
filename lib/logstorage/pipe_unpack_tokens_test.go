package logstorage

import (
	"testing"
)

func TestParsePipeUnpackTokensSuccess(t *testing.T) {
	f := func(pipeStr string) {
		t.Helper()
		expectParsePipeSuccess(t, pipeStr)
	}

	f(`unpack_tokens`)
	f(`unpack_tokens as bar`)
	f(`unpack_tokens from foo`)
	f(`unpack_tokens from foo as bar`)
}

func TestParsePipeUrollTokensFailure(t *testing.T) {
	f := func(pipeStr string) {
		t.Helper()
		expectParsePipeFailure(t, pipeStr)
	}

	f(`unpack_tokens as`)
	f(`unpack_tokens from`)
	f(`unpack_tokens foo bar baz`)
	f(`unpack_tokens foo, bar`)
}

func TestPipeUnpackTokens(t *testing.T) {
	f := func(pipeStr string, rows, rowsExpected [][]Field) {
		t.Helper()
		expectPipeResults(t, pipeStr, rows, rowsExpected)
	}

	// unpack_tokens by missing field
	f("unpack_tokens x", [][]Field{
		{
			{"a", `["foo",1,{"baz":"x"},[1,2],null,NaN]`},
			{"q", "w"},
		},
	}, [][]Field{
		{
			{"a", `["foo",1,{"baz":"x"},[1,2],null,NaN]`},
			{"q", "w"},
			{"x", "[]"},
		},
	})

	// unpack_tokens by a field without tokens
	f("unpack_tokens q", [][]Field{
		{
			{"a", `["foo",1,{"baz":"x"},[1,2],null,NaN]`},
			{"q", "!#$%,"},
		},
	}, [][]Field{
		{
			{"a", `["foo",1,{"baz":"x"},[1,2],null,NaN]`},
			{"q", "[]"},
		},
	})

	// unpack_tokens by a field with tokens
	f("unpack_tokens a", [][]Field{
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
			{"a", `["foo","bar","baz"]`},
			{"q", "w"},
		},
		{
			{"a", `["b"]`},
			{"c", "d"},
		},
	})

	// unpack_tokens by a field with tokens into another field
	f("unpack_tokens from a as b", [][]Field{
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
			{"b", `["foo","bar","baz"]`},
			{"q", "w"},
		},
		{
			{"a", "b"},
			{"b", `["b"]`},
			{"c", "d"},
		},
	})

	// unpack_tokens from _msg inplace
	f("unpack_tokens", [][]Field{
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
			{"_msg", `["foo","bar","baz"]`},
			{"q", "w"},
		},
		{
			{"_msg", `["b"]`},
			{"c", "d"},
		},
	})

	// unpack_tokens from _msg into other field
	f("unpack_tokens as b", [][]Field{
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
			{"b", `["foo","bar","foo"]`},
			{"q", "w"},
		},
		{
			{"_msg", "b"},
			{"b", `["b"]`},
			{"c", "d"},
		},
	})
}

func TestPipeUnpackTokensUpdateNeededFields(t *testing.T) {
	f := func(s string, neededFields, unneededFields, neededFieldsExpected, unneededFieldsExpected string) {
		t.Helper()
		expectPipeNeededFields(t, s, neededFields, unneededFields, neededFieldsExpected, unneededFieldsExpected)
	}

	// all the needed fields
	f("unpack_tokens x", "*", "", "*", "")
	f("unpack_tokens x y", "*", "", "*", "y")

	// all the needed fields, unneeded fields do not intersect with src
	f("unpack_tokens x", "*", "f1,f2", "*", "f1,f2")
	f("unpack_tokens x as y", "*", "f1,f2", "*", "f1,f2,y")

	// all the needed fields, unneeded fields intersect with src
	f("unpack_tokens x", "*", "f2,x", "*", "f2,x")
	f("unpack_tokens x y", "*", "f2,x", "*", "f2,y")
	f("unpack_tokens x y", "*", "f2,y", "*", "f2,y")

	// needed fields do not intersect with src
	f("unpack_tokens x", "f1,f2", "", "f1,f2", "")
	f("unpack_tokens x y", "f1,f2", "", "f1,f2", "")

	// needed fields intersect with src
	f("unpack_tokens x", "f2,x", "", "f2,x", "")
	f("unpack_tokens x y", "f2,x", "", "f2,x", "")
	f("unpack_tokens x y", "f2,y", "", "f2,x", "")
}
