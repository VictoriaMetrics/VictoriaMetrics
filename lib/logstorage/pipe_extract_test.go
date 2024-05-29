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
	f(`extract "foo<bar>" skip_empty_results`)
	f(`extract "foo<bar>" keep_original_fields`)
	f(`extract "foo<bar>" from x`)
	f(`extract "foo<bar>" from x skip_empty_results`)
	f(`extract "foo<bar>" from x keep_original_fields`)
	f(`extract if (x:y) "foo<bar>" from baz`)
	f(`extract if (x:y) "foo<bar>" from baz skip_empty_results`)
	f(`extract if (x:y) "foo<bar>" from baz keep_original_fields`)
}

func TestParsePipeExtractFailure(t *testing.T) {
	f := func(pipeStr string) {
		t.Helper()
		expectParsePipeFailure(t, pipeStr)
	}

	f(`extract`)
	f(`extract keep_original_fields`)
	f(`extract skip_empty_results`)
	f(`extract from`)
	f(`extract from x`)
	f(`extract from x "y<foo>"`)
	f(`extract if (x:y)`)
	f(`extract "a<b>" if (x:y)`)
	f(`extract "a"`)
	f(`extract "<a><b>"`)
	f(`extract "<*>foo<_>bar"`)
}

func TestPipeExtract(t *testing.T) {
	f := func(pipeStr string, rows, rowsExpected [][]Field) {
		t.Helper()
		expectPipeResults(t, pipeStr, rows, rowsExpected)
	}

	// skip empty results
	f(`extract "baz=<abc> a=<aa>" skip_empty_results`, [][]Field{
		{
			{"_msg", `foo=bar baz="x y=z" `},
			{"aa", "foobar"},
			{"abc", "ippl"},
		},
	}, [][]Field{
		{
			{"_msg", `foo=bar baz="x y=z" `},
			{"aa", "foobar"},
			{"abc", "x y=z"},
		},
	})

	// no skip empty results
	f(`extract "baz=<abc> a=<aa>"`, [][]Field{
		{
			{"_msg", `foo=bar baz="x y=z" `},
			{"aa", "foobar"},
			{"abc", "ippl"},
		},
	}, [][]Field{
		{
			{"_msg", `foo=bar baz="x y=z" `},
			{"aa", ""},
			{"abc", "x y=z"},
		},
	})

	// keep original fields
	f(`extract "baz=<abc> a=<aa>" keep_original_fields`, [][]Field{
		{
			{"_msg", `foo=bar baz="x y=z" a=b`},
			{"aa", "foobar"},
			{"abc", ""},
		},
	}, [][]Field{
		{
			{"_msg", `foo=bar baz="x y=z" a=b`},
			{"abc", "x y=z"},
			{"aa", "foobar"},
		},
	})

	// no keep original fields
	f(`extract "baz=<abc> a=<aa>"`, [][]Field{
		{
			{"_msg", `foo=bar baz="x y=z" a=b`},
			{"aa", "foobar"},
			{"abc", ""},
		},
	}, [][]Field{
		{
			{"_msg", `foo=bar baz="x y=z" a=b`},
			{"abc", "x y=z"},
			{"aa", "b"},
		},
	})

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
	f(`extract "foo=<bar>" from x`, [][]Field{
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
	f(`extract "foo=<bar>" from x`, [][]Field{
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
	f(`extract "foo=<bar> baz=<xx>" from x`, [][]Field{
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

	// single row, disable unquoting
	f(`extract 'foo=[< plain : bar >]' from x`, [][]Field{
		{
			{"x", `a foo=["bc","de"]`},
		},
	}, [][]Field{
		{
			{"x", `a foo=["bc","de"]`},
			{"bar", `"bc","de"`},
		},
	})

	// single row, default unquoting
	f(`extract 'foo=[< bar >]' from x`, [][]Field{
		{
			{"x", `a foo=["bc","de"]`},
		},
	}, [][]Field{
		{
			{"x", `a foo=["bc","de"]`},
			{"bar", `bc`},
		},
	})

	// single row, overwirte existing column
	f(`extract "foo=<bar> baz=<xx>" from x`, [][]Field{
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
	f(`extract if (x:baz) "foo=<bar> baz=<xx>" from "x"`, [][]Field{
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
	f(`extract if (bar:"") "foo=<bar> baz=<xx>" from 'x'`, [][]Field{
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
	f(`extract if (!ip:keep) "ip=<ip> "`, [][]Field{
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
	f("extract '<foo>' from x", "*", "", "*", "foo")
	f("extract if (foo:bar) '<foo>' from x", "*", "", "*", "")
	f("extract if (foo:bar) '<foo>' from x keep_original_fields", "*", "", "*", "")
	f("extract if (foo:bar) '<foo>' from x skip_empty_results", "*", "", "*", "")

	// unneeded fields do not intersect with pattern and output fields
	f("extract '<foo>' from x", "*", "f1,f2", "*", "f1,f2,foo")
	f("extract '<foo>' from x keep_original_fields", "*", "f1,f2", "*", "f1,f2")
	f("extract '<foo>' from x skip_empty_results", "*", "f1,f2", "*", "f1,f2")
	f("extract if (f1:x) '<foo>' from x", "*", "f1,f2", "*", "f2,foo")
	f("extract if (f1:x) '<foo>' from x keep_original_fields", "*", "f1,f2", "*", "f2")
	f("extract if (f1:x) '<foo>' from x skip_empty_results", "*", "f1,f2", "*", "f2")
	f("extract if (foo:bar f1:x) '<foo>' from x", "*", "f1,f2", "*", "f2")

	// unneeded fields intersect with pattern
	f("extract '<foo>' from x", "*", "f2,x", "*", "f2,foo")
	f("extract '<foo>' from x keep_original_fields", "*", "f2,x", "*", "f2")
	f("extract '<foo>' from x skip_empty_results", "*", "f2,x", "*", "f2")
	f("extract if (f1:abc) '<foo>' from x", "*", "f2,x", "*", "f2,foo")
	f("extract if (f2:abc) '<foo>' from x", "*", "f2,x", "*", "foo")

	// unneeded fields intersect with output fields
	f("extract '<foo>x<bar>' from x", "*", "f2,foo", "*", "bar,f2,foo")
	f("extract '<foo>x<bar>' from x keep_original_fields", "*", "f2,foo", "*", "f2,foo")
	f("extract '<foo>x<bar>' from x skip_empty_results", "*", "f2,foo", "*", "f2,foo")
	f("extract if (f1:abc) '<foo>x<bar>' from x", "*", "f2,foo", "*", "bar,f2,foo")
	f("extract if (f2:abc foo:w) '<foo>x<bar>' from x", "*", "f2,foo", "*", "bar")
	f("extract if (f2:abc foo:w) '<foo>x<bar>' from x keep_original_fields", "*", "f2,foo", "*", "")
	f("extract if (f2:abc foo:w) '<foo>x<bar>' from x skip_empty_results", "*", "f2,foo", "*", "")

	// unneeded fields intersect with all the output fields
	f("extract '<foo>x<bar>' from x", "*", "f2,foo,bar", "*", "bar,f2,foo,x")
	f("extract if (a:b f2:q x:y foo:w) '<foo>x<bar>' from x", "*", "f2,foo,bar", "*", "bar,f2,foo,x")
	f("extract if (a:b f2:q x:y foo:w) '<foo>x<bar>' from x keep_original_fields", "*", "f2,foo,bar", "*", "bar,f2,foo,x")
	f("extract if (a:b f2:q x:y foo:w) '<foo>x<bar>' from x skip_empty_results", "*", "f2,foo,bar", "*", "bar,f2,foo,x")

	// needed fields do not intersect with pattern and output fields
	f("extract '<foo>x<bar>' from x", "f1,f2", "", "f1,f2", "")
	f("extract '<foo>x<bar>' from x keep_original_fields", "f1,f2", "", "f1,f2", "")
	f("extract '<foo>x<bar>' from x skip_empty_results", "f1,f2", "", "f1,f2", "")
	f("extract if (a:b) '<foo>x<bar>' from x", "f1,f2", "", "f1,f2", "")
	f("extract if (f1:b) '<foo>x<bar>' from x", "f1,f2", "", "f1,f2", "")

	// needed fields intersect with pattern field
	f("extract '<foo>x<bar>' from x", "f2,x", "", "f2,x", "")
	f("extract '<foo>x<bar>' from x keep_original_fields", "f2,x", "", "f2,x", "")
	f("extract '<foo>x<bar>' from x skip_empty_results", "f2,x", "", "f2,x", "")
	f("extract if (a:b) '<foo>x<bar>' from x", "f2,x", "", "f2,x", "")

	// needed fields intersect with output fields
	f("extract '<foo>x<bar>' from x", "f2,foo", "", "f2,x", "")
	f("extract '<foo>x<bar>' from x keep_original_fields", "f2,foo", "", "foo,f2,x", "")
	f("extract '<foo>x<bar>' from x skip_empty_results", "f2,foo", "", "foo,f2,x", "")
	f("extract if (a:b) '<foo>x<bar>' from x", "f2,foo", "", "a,f2,x", "")

	// needed fields intersect with pattern and output fields
	f("extract '<foo>x<bar>' from x", "f2,foo,x,y", "", "f2,x,y", "")
	f("extract '<foo>x<bar>' from x keep_original_fields", "f2,foo,x,y", "", "foo,f2,x,y", "")
	f("extract '<foo>x<bar>' from x skip_empty_results", "f2,foo,x,y", "", "foo,f2,x,y", "")
	f("extract if (a:b foo:q) '<foo>x<bar>' from x", "f2,foo,x,y", "", "a,f2,foo,x,y", "")
}
