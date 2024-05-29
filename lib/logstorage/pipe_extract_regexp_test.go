package logstorage

import (
	"testing"
)

func TestParsePipeExtractRegexpSuccess(t *testing.T) {
	f := func(pipeStr string) {
		t.Helper()
		expectParsePipeSuccess(t, pipeStr)
	}

	f(`extract_regexp "foo(?P<bar>.*)"`)
	f(`extract_regexp "foo(?P<bar>.*)" skip_empty_results`)
	f(`extract_regexp "foo(?P<bar>.*)" keep_original_fields`)
	f(`extract_regexp "foo(?P<bar>.*)" from x`)
	f(`extract_regexp "foo(?P<bar>.*)" from x skip_empty_results`)
	f(`extract_regexp "foo(?P<bar>.*)" from x keep_original_fields`)
	f(`extract_regexp if (x:y) "foo(?P<bar>.*)" from baz`)
	f(`extract_regexp if (x:y) "foo(?P<bar>.*)" from baz skip_empty_results`)
	f(`extract_regexp if (x:y) "foo(?P<bar>.*)" from baz keep_original_fields`)
}

func TestParsePipeExtractRegexpFailure(t *testing.T) {
	f := func(pipeStr string) {
		t.Helper()
		expectParsePipeFailure(t, pipeStr)
	}

	f(`extract_regexp`)
	f(`extract_regexp keep_original_fields`)
	f(`extract_regexp skip_empty_results`)
	f(`extract_regexp from`)
	f(`extract_regexp from x`)
	f(`extract_regexp from x "y(?P<foo>.*)"`)
	f(`extract_regexp if (x:y)`)
	f(`extract_regexp "a(?P<b>.*)" if (x:y)`)
	f(`extract_regexp "a"`)
	f(`extract_regexp "(foo)"`)
}

func TestPipeExtractRegexp(t *testing.T) {
	f := func(pipeStr string, rows, rowsExpected [][]Field) {
		t.Helper()
		expectPipeResults(t, pipeStr, rows, rowsExpected)
	}

	// skip empty results
	f(`extract_regexp "baz=(?P<abc>.*) a=(?P<aa>.*)" skip_empty_results`, [][]Field{
		{
			{"_msg", `foo=bar baz="x y=z" a=`},
			{"aa", "foobar"},
			{"abc", "ippl"},
		},
	}, [][]Field{
		{
			{"_msg", `foo=bar baz="x y=z" a=`},
			{"aa", "foobar"},
			{"abc", `"x y=z"`},
		},
	})

	// no skip empty results
	f(`extract_regexp "baz=(?P<abc>.*) a=(?P<aa>.*)"`, [][]Field{
		{
			{"_msg", `foo=bar baz="x y=z" a=`},
			{"aa", "foobar"},
			{"abc", "ippl"},
		},
	}, [][]Field{
		{
			{"_msg", `foo=bar baz="x y=z" a=`},
			{"aa", ""},
			{"abc", `"x y=z"`},
		},
	})

	// keep original fields
	f(`extract_regexp "baz=(?P<abc>.*) a=(?P<aa>.*)" keep_original_fields`, [][]Field{
		{
			{"_msg", `foo=bar baz="x y=z" a=b`},
			{"aa", "foobar"},
			{"abc", ""},
		},
	}, [][]Field{
		{
			{"_msg", `foo=bar baz="x y=z" a=b`},
			{"abc", `"x y=z"`},
			{"aa", "foobar"},
		},
	})

	// no keep original fields
	f(`extract_regexp "baz=(?P<abc>.*) a=(?P<aa>.*)"`, [][]Field{
		{
			{"_msg", `foo=bar baz="x y=z" a=b`},
			{"aa", "foobar"},
			{"abc", ""},
		},
	}, [][]Field{
		{
			{"_msg", `foo=bar baz="x y=z" a=b`},
			{"abc", `"x y=z"`},
			{"aa", "b"},
		},
	})

	// single row, extract from _msg
	f(`extract_regexp "baz=(?P<abc>.*) a=(?P<aa>.*)"`, [][]Field{
		{
			{"_msg", `foo=bar baz="x y=z" a=b`},
		},
	}, [][]Field{
		{
			{"_msg", `foo=bar baz="x y=z" a=b`},
			{"abc", `"x y=z"`},
			{"aa", "b"},
		},
	})

	// single row, extract from _msg into _msg
	f(`extract_regexp "msg=(?P<_msg>.*)"`, [][]Field{
		{
			{"_msg", `msg=bar`},
		},
	}, [][]Field{
		{
			{"_msg", "bar"},
		},
	})

	// single row, extract from non-existing field
	f(`extract_regexp "foo=(?P<bar>.*)" from x`, [][]Field{
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
	f(`extract_regexp "foo=(?P<bar>.*)" from x`, [][]Field{
		{
			{"x", `foobar`},
		},
	}, [][]Field{
		{
			{"x", `foobar`},
			{"bar", ""},
		},
	})

	f(`extract_regexp "foo=(?P<bar>.*) baz=(?P<xx>.*)" from x`, [][]Field{
		{
			{"x", `a foo="a\"b\\c" cde baz=aa`},
		},
	}, [][]Field{
		{
			{"x", `a foo="a\"b\\c" cde baz=aa`},
			{"bar", `"a\"b\\c" cde`},
			{"xx", "aa"},
		},
	})

	// single row, overwirte existing column
	f(`extract_regexp "foo=(?P<bar>.*) baz=(?P<xx>.*)" from x`, [][]Field{
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
	f(`extract_regexp if (x:baz) "foo=(?P<bar>.*) baz=(?P<xx>.*)" from "x"`, [][]Field{
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
	f(`extract_regexp if (bar:"") "foo=(?P<bar>.*) baz=(?P<xx>.*)" from 'x'`, [][]Field{
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
	f(`extract_regexp if (!ip:keep) "ip=(?P<ip>([0-9]+[.]){3}[0-9]+) "`, [][]Field{
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
			{"ip", ""},
			{"a", "klo2i"},
		},
	})
}

func TestPipeExtractRegexpUpdateNeededFields(t *testing.T) {
	f := func(s string, neededFields, unneededFields, neededFieldsExpected, unneededFieldsExpected string) {
		t.Helper()
		expectPipeNeededFields(t, s, neededFields, unneededFields, neededFieldsExpected, unneededFieldsExpected)
	}

	// all the needed fields
	f("extract_regexp '(?P<foo>.*)' from x", "*", "", "*", "foo")
	f("extract_regexp if (foo:bar) '(?P<foo>.*)' from x", "*", "", "*", "")
	f("extract_regexp if (foo:bar) '(?P<foo>.*)' from x keep_original_fields", "*", "", "*", "")
	f("extract_regexp if (foo:bar) '(?P<foo>.*)' from x skip_empty_results", "*", "", "*", "")

	// unneeded fields do not intersect with pattern and output fields
	f("extract_regexp '(?P<foo>.*)' from x", "*", "f1,f2", "*", "f1,f2,foo")
	f("extract_regexp '(?P<foo>.*)' from x keep_original_fields", "*", "f1,f2", "*", "f1,f2")
	f("extract_regexp '(?P<foo>.*)' from x skip_empty_results", "*", "f1,f2", "*", "f1,f2")
	f("extract_regexp if (f1:x) '(?P<foo>.*)' from x", "*", "f1,f2", "*", "f2,foo")
	f("extract_regexp if (f1:x) '(?P<foo>.*)' from x keep_original_fields", "*", "f1,f2", "*", "f2")
	f("extract_regexp if (f1:x) '(?P<foo>.*)' from x skip_empty_results", "*", "f1,f2", "*", "f2")
	f("extract_regexp if (foo:bar f1:x) '(?P<foo>.*)' from x", "*", "f1,f2", "*", "f2")

	// unneeded fields intersect with pattern
	f("extract_regexp '(?P<foo>.*)' from x", "*", "f2,x", "*", "f2,foo")
	f("extract_regexp '(?P<foo>.*)' from x keep_original_fields", "*", "f2,x", "*", "f2")
	f("extract_regexp '(?P<foo>.*)' from x skip_empty_results", "*", "f2,x", "*", "f2")
	f("extract_regexp if (f1:abc) '(?P<foo>.*)' from x", "*", "f2,x", "*", "f2,foo")
	f("extract_regexp if (f2:abc) '(?P<foo>.*)' from x", "*", "f2,x", "*", "foo")

	// unneeded fields intersect with output fields
	f("extract_regexp '(?P<foo>.*)x(?P<bar>.*)' from x", "*", "f2,foo", "*", "bar,f2,foo")
	f("extract_regexp '(?P<foo>.*)x(?P<bar>.*)' from x keep_original_fields", "*", "f2,foo", "*", "f2,foo")
	f("extract_regexp '(?P<foo>.*)x(?P<bar>.*)' from x skip_empty_results", "*", "f2,foo", "*", "f2,foo")
	f("extract_regexp if (f1:abc) '(?P<foo>.*)x(?P<bar>.*)' from x", "*", "f2,foo", "*", "bar,f2,foo")
	f("extract_regexp if (f2:abc foo:w) '(?P<foo>.*)x(?P<bar>.*)' from x", "*", "f2,foo", "*", "bar")
	f("extract_regexp if (f2:abc foo:w) '(?P<foo>.*)x(?P<bar>.*)' from x keep_original_fields", "*", "f2,foo", "*", "")
	f("extract_regexp if (f2:abc foo:w) '(?P<foo>.*)x(?P<bar>.*)' from x skip_empty_results", "*", "f2,foo", "*", "")

	// unneeded fields intersect with all the output fields
	f("extract_regexp '(?P<foo>.*)x(?P<bar>.*)' from x", "*", "f2,foo,bar", "*", "bar,f2,foo,x")
	f("extract_regexp if (a:b f2:q x:y foo:w) '(?P<foo>.*)x(?P<bar>.*)' from x", "*", "f2,foo,bar", "*", "bar,f2,foo,x")
	f("extract_regexp if (a:b f2:q x:y foo:w) '(?P<foo>.*)x(?P<bar>.*)' from x keep_original_fields", "*", "f2,foo,bar", "*", "bar,f2,foo,x")
	f("extract_regexp if (a:b f2:q x:y foo:w) '(?P<foo>.*)x(?P<bar>.*)' from x skip_empty_results", "*", "f2,foo,bar", "*", "bar,f2,foo,x")

	// needed fields do not intersect with pattern and output fields
	f("extract_regexp '(?P<foo>.*)x(?P<bar>.*)' from x", "f1,f2", "", "f1,f2", "")
	f("extract_regexp '(?P<foo>.*)x(?P<bar>.*)' from x keep_original_fields", "f1,f2", "", "f1,f2", "")
	f("extract_regexp '(?P<foo>.*)x(?P<bar>.*)' from x skip_empty_results", "f1,f2", "", "f1,f2", "")
	f("extract_regexp if (a:b) '(?P<foo>.*)x(?P<bar>.*)' from x", "f1,f2", "", "f1,f2", "")
	f("extract_regexp if (f1:b) '(?P<foo>.*)x(?P<bar>.*)' from x", "f1,f2", "", "f1,f2", "")

	// needed fields intersect with pattern field
	f("extract_regexp '(?P<foo>.*)x(?P<bar>.*)' from x", "f2,x", "", "f2,x", "")
	f("extract_regexp '(?P<foo>.*)x(?P<bar>.*)' from x keep_original_fields", "f2,x", "", "f2,x", "")
	f("extract_regexp '(?P<foo>.*)x(?P<bar>.*)' from x skip_empty_results", "f2,x", "", "f2,x", "")
	f("extract_regexp if (a:b) '(?P<foo>.*)x(?P<bar>.*)' from x", "f2,x", "", "f2,x", "")

	// needed fields intersect with output fields
	f("extract_regexp '(?P<foo>.*)x(?P<bar>.*)' from x", "f2,foo", "", "f2,x", "")
	f("extract_regexp '(?P<foo>.*)x(?P<bar>.*)' from x keep_original_fields", "f2,foo", "", "foo,f2,x", "")
	f("extract_regexp '(?P<foo>.*)x(?P<bar>.*)' from x skip_empty_results", "f2,foo", "", "foo,f2,x", "")
	f("extract_regexp if (a:b) '(?P<foo>.*)x(?P<bar>.*)' from x", "f2,foo", "", "a,f2,x", "")

	// needed fields intersect with pattern and output fields
	f("extract_regexp '(?P<foo>.*)x(?P<bar>.*)' from x", "f2,foo,x,y", "", "f2,x,y", "")
	f("extract_regexp '(?P<foo>.*)x(?P<bar>.*)' from x keep_original_fields", "f2,foo,x,y", "", "foo,f2,x,y", "")
	f("extract_regexp '(?P<foo>.*)x(?P<bar>.*)' from x skip_empty_results", "f2,foo,x,y", "", "foo,f2,x,y", "")
	f("extract_regexp if (a:b foo:q) '(?P<foo>.*)x(?P<bar>.*)' from x", "f2,foo,x,y", "", "a,f2,foo,x,y", "")
}
