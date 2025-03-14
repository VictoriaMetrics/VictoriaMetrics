package logstorage

import (
	"testing"
)

func TestParsePipeUnpackWordsSuccess(t *testing.T) {
	f := func(pipeStr string) {
		t.Helper()
		expectParsePipeSuccess(t, pipeStr)
	}

	f(`unpack_words`)
	f(`unpack_words drop_duplicates`)
	f(`unpack_words as bar`)
	f(`unpack_words as bar drop_duplicates`)
	f(`unpack_words from foo`)
	f(`unpack_words from foo drop_duplicates`)
	f(`unpack_words from foo as bar`)
	f(`unpack_words from foo as bar drop_duplicates`)
}

func TestParsePipeUnpackWordsFailure(t *testing.T) {
	f := func(pipeStr string) {
		t.Helper()
		expectParsePipeFailure(t, pipeStr)
	}

	f(`unpack_words as`)
	f(`unpack_words drop_duplicates x`)
	f(`unpack_words from`)
	f(`unpack_words foo bar baz`)
	f(`unpack_words foo, bar`)
}

func TestPipeUnpackWords(t *testing.T) {
	f := func(pipeStr string, rows, rowsExpected [][]Field) {
		t.Helper()
		expectPipeResults(t, pipeStr, rows, rowsExpected)
	}

	// unpack_words by missing field
	f("unpack_words x", [][]Field{
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

	// unpack_words by a field without words
	f("unpack_words q", [][]Field{
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

	// unpack_words by a field with words
	f("unpack_words a", [][]Field{
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

	// unpack_words by a field with words into another field
	f("unpack_words from a as b", [][]Field{
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

	// unpack_words from _msg inplace
	f("unpack_words", [][]Field{
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

	// unpack_words from _msg into other field
	f("unpack_words as b", [][]Field{
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

	// unpack_words from _msg into other field with dropping duplicate words
	f("unpack_words as b drop_duplicates", [][]Field{
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
			{"b", `["foo","bar"]`},
			{"q", "w"},
		},
		{
			{"_msg", "b"},
			{"b", `["b"]`},
			{"c", "d"},
		},
	})
}

func TestPipeUnpackWordsUpdateNeededFields(t *testing.T) {
	f := func(s string, neededFields, unneededFields, neededFieldsExpected, unneededFieldsExpected string) {
		t.Helper()
		expectPipeNeededFields(t, s, neededFields, unneededFields, neededFieldsExpected, unneededFieldsExpected)
	}

	// all the needed fields
	f("unpack_words x", "*", "", "*", "")
	f("unpack_words x y", "*", "", "*", "y")

	// all the needed fields, unneeded fields do not intersect with src
	f("unpack_words x", "*", "f1,f2", "*", "f1,f2")
	f("unpack_words x as y", "*", "f1,f2", "*", "f1,f2,y")

	// all the needed fields, unneeded fields intersect with src
	f("unpack_words x", "*", "f2,x", "*", "f2,x")
	f("unpack_words x y", "*", "f2,x", "*", "f2,y")
	f("unpack_words x y", "*", "f2,y", "*", "f2,y")

	// needed fields do not intersect with src
	f("unpack_words x", "f1,f2", "", "f1,f2", "")
	f("unpack_words x y", "f1,f2", "", "f1,f2", "")

	// needed fields intersect with src
	f("unpack_words x", "f2,x", "", "f2,x", "")
	f("unpack_words x y", "f2,x", "", "f2,x", "")
	f("unpack_words x y", "f2,y", "", "f2,x", "")
}
