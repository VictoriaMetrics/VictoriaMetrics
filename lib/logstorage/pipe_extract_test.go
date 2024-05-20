package logstorage

import (
	"testing"
)

func TestParsePipeExtractSuccess(t *testing.T) {
	f := func(pipeStr string) {
		t.Helper()
		expectParsePipeSuccess(t, pipeStr)
	}

	f(`extract "foo<bar>"`)
	f(`extract from x "foo<bar>"`)
	f(`extract from x "foo<bar>" if (y:in(a:foo bar | uniq by (qwe) limit 10))`)
}

func TestParsePipeExtractFailure(t *testing.T) {
	f := func(pipeStr string) {
		t.Helper()
		expectParsePipeFailure(t, pipeStr)
	}

	f(`extract`)
	f(`extract from`)
	f(`extract if (x:y)`)
	f(`extract if (x:y) "a<b>"`)
	f(`extract "a<b>" if`)
	f(`extract "a<b>" if (foo`)
	f(`extract "a<b>" if "foo"`)
	f(`extract "a"`)
	f(`extract "<a><b>"`)
	f(`extract "<*>foo<_>bar"`)
}

func TestPipeExtract(t *testing.T) {
	f := func(pipeStr string, rows, rowsExpected [][]Field) {
		t.Helper()
		expectPipeResults(t, pipeStr, rows, rowsExpected)
	}

	// single row, extract from _msg
	f(`extract "baz=<abc> a=<aa>"`, [][]Field{
		{
			{"_msg", `foo=bar baz="x y=z" a=b`},
		},
	}, [][]Field{
		{
			{"_msg", `foo=bar baz="x y=z" a=b`},
			{"abc", "x y=z"},
			{"aa", "b"},
		},
	})

	// single row, extract from _msg into _msg
	f(`extract "msg=<_msg>"`, [][]Field{
		{
			{"_msg", `msg=bar`},
		},
	}, [][]Field{
		{
			{"_msg", "bar"},
		},
	})

	// single row, extract from non-existing field
	f(`extract from x "foo=<bar>"`, [][]Field{
		{
			{"_msg", `foo=bar`},
		},
	}, [][]Field{
		{
			{"_msg", `foo=bar`},
			{"bar", ""},
		},
	})

	// single row, pattern mismatch
	f(`extract from x "foo=<bar>"`, [][]Field{
		{
			{"x", `foobar`},
		},
	}, [][]Field{
		{
			{"x", `foobar`},
			{"bar", ""},
		},
	})

	// single row, partial partern match
	f(`extract from x "foo=<bar> baz=<xx>"`, [][]Field{
		{
			{"x", `a foo="a\"b\\c" cde baz=aa`},
		},
	}, [][]Field{
		{
			{"x", `a foo="a\"b\\c" cde baz=aa`},
			{"bar", `a"b\c`},
			{"xx", ""},
		},
	})

	// single row, overwirte existing column
	f(`extract from x "foo=<bar> baz=<xx>"`, [][]Field{
		{
			{"x", `a foo=cc baz=aa b`},
			{"bar", "abc"},
		},
	}, [][]Field{
		{
			{"x", `a foo=cc baz=aa b`},
			{"bar", `cc`},
			{"xx", `aa b`},
		},
	})

	// single row, if match
	f(`extract from x "foo=<bar> baz=<xx>" if (x:baz)`, [][]Field{
		{
			{"x", `a foo=cc baz=aa b`},
			{"bar", "abc"},
		},
	}, [][]Field{
		{
			{"x", `a foo=cc baz=aa b`},
			{"bar", `cc`},
			{"xx", `aa b`},
		},
	})

	// single row, if mismatch
	f(`extract from x "foo=<bar> baz=<xx>" if (bar:"")`, [][]Field{
		{
			{"x", `a foo=cc baz=aa b`},
			{"bar", "abc"},
		},
	}, [][]Field{
		{
			{"x", `a foo=cc baz=aa b`},
			{"bar", `abc`},
		},
	})

	// multiple rows with distinct set of labels
	f(`extract "ip=<ip> " if (!ip:keep)`, [][]Field{
		{
			{"foo", "bar"},
			{"_msg", "request from ip=1.2.3.4 xxx"},
			{"f3", "y"},
		},
		{
			{"foo", "aaa"},
			{"_msg", "ip=5.4.3.1 abcd"},
			{"ip", "keep"},
			{"a", "b"},
		},
		{
			{"foo", "aaa"},
			{"_msg", "ip=34.32.11.94 abcd"},
			{"ip", "ppp"},
			{"a", "b"},
		},
		{
			{"foo", "klkfs"},
			{"_msg", "sdfdsfds dsf fd fdsa ip=123 abcd"},
			{"ip", "bbbsd"},
			{"a", "klo2i"},
		},
	}, [][]Field{
		{
			{"foo", "bar"},
			{"_msg", "request from ip=1.2.3.4 xxx"},
			{"f3", "y"},
			{"ip", "1.2.3.4"},
		},
		{
			{"foo", "aaa"},
			{"_msg", "ip=5.4.3.1 abcd"},
			{"ip", "keep"},
			{"a", "b"},
		},
		{
			{"foo", "aaa"},
			{"_msg", "ip=34.32.11.94 abcd"},
			{"ip", "34.32.11.94"},
			{"a", "b"},
		},
		{
			{"foo", "klkfs"},
			{"_msg", "sdfdsfds dsf fd fdsa ip=123 abcd"},
			{"ip", "123"},
			{"a", "klo2i"},
		},
	})
}

func TestPipeExtractUpdateNeededFields(t *testing.T) {
	f := func(s string, neededFields, unneededFields, neededFieldsExpected, unneededFieldsExpected string) {
		t.Helper()
		expectPipeNeededFields(t, s, neededFields, unneededFields, neededFieldsExpected, unneededFieldsExpected)
	}

	// all the needed fields
	f("extract from x '<foo>'", "*", "", "*", "foo")
	f("extract from x '<foo>' if (foo:bar)", "*", "", "*", "")

	// unneeded fields do not intersect with pattern and output fields
	f("extract from x '<foo>'", "*", "f1,f2", "*", "f1,f2,foo")
	f("extract from x '<foo>' if (f1:x)", "*", "f1,f2", "*", "f2,foo")
	f("extract from x '<foo>' if (foo:bar f1:x)", "*", "f1,f2", "*", "f2")

	// unneeded fields intersect with pattern
	f("extract from x '<foo>'", "*", "f2,x", "*", "f2,foo")
	f("extract from x '<foo>' if (f1:abc)", "*", "f2,x", "*", "f2,foo")
	f("extract from x '<foo>' if (f2:abc)", "*", "f2,x", "*", "foo")

	// unneeded fields intersect with output fields
	f("extract from x '<foo>x<bar>'", "*", "f2,foo", "*", "bar,f2,foo")
	f("extract from x '<foo>x<bar>' if (f1:abc)", "*", "f2,foo", "*", "bar,f2,foo")
	f("extract from x '<foo>x<bar>' if (f2:abc foo:w)", "*", "f2,foo", "*", "bar")

	// unneeded fields intersect with all the output fields
	f("extract from x '<foo>x<bar>'", "*", "f2,foo,bar", "*", "bar,f2,foo,x")
	f("extract from x '<foo>x<bar> if (a:b f2:q x:y foo:w)'", "*", "f2,foo,bar", "*", "bar,f2,foo,x")

	// needed fields do not intersect with pattern and output fields
	f("extract from x '<foo>x<bar>'", "f1,f2", "", "f1,f2", "")
	f("extract from x '<foo>x<bar>' if (a:b)", "f1,f2", "", "f1,f2", "")
	f("extract from x '<foo>x<bar>' if (f1:b)", "f1,f2", "", "f1,f2", "")

	// needed fields intersect with pattern field
	f("extract from x '<foo>x<bar>'", "f2,x", "", "f2,x", "")
	f("extract from x '<foo>x<bar>' if (a:b)", "f2,x", "", "f2,x", "")

	// needed fields intersect with output fields
	f("extract from x '<foo>x<bar>'", "f2,foo", "", "f2,x", "")
	f("extract from x '<foo>x<bar>' if (a:b)", "f2,foo", "", "a,f2,x", "")

	// needed fields intersect with pattern and output fields
	f("extract from x '<foo>x<bar>'", "f2,foo,x,y", "", "f2,x,y", "")
	f("extract from x '<foo>x<bar>' if (a:b foo:q)", "f2,foo,x,y", "", "a,f2,foo,x,y", "")
}

func expectParsePipeFailure(t *testing.T, pipeStr string) {
	t.Helper()

	lex := newLexer(pipeStr)
	p, err := parsePipe(lex)
	if err == nil && lex.isEnd() {
		t.Fatalf("expecting error when parsing [%s]; parsed result: [%s]", pipeStr, p)
	}
}

func expectParsePipeSuccess(t *testing.T, pipeStr string) {
	t.Helper()

	lex := newLexer(pipeStr)
	p, err := parsePipe(lex)
	if err != nil {
		t.Fatalf("cannot parse [%s]: %s", pipeStr, err)
	}
	if !lex.isEnd() {
		t.Fatalf("unexpected tail after parsing [%s]: [%s]", pipeStr, lex.s)
	}

	pipeStrResult := p.String()
	if pipeStrResult != pipeStr {
		t.Fatalf("unexpected string representation of pipe; got\n%s\nwant\n%s", pipeStrResult, pipeStr)
	}
}
