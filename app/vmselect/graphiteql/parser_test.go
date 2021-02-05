package graphiteql

import (
	"testing"
)

func TestQuoteString(t *testing.T) {
	f := func(s, qExpected string) {
		t.Helper()
		q := QuoteString(s)
		if q != qExpected {
			t.Fatalf("unexpected result from QuoteString(%q); got %s; want %s", s, q, qExpected)
		}
	}
	f(``, `''`)
	f(`foo`, `'foo'`)
	f(`f'o\ba"r`, `'f\'o\\ba"r'`)
}

func TestParseSuccess(t *testing.T) {
	another := func(s, resultExpected string) {
		t.Helper()
		expr, err := Parse(s)
		if err != nil {
			t.Fatalf("unexpected error when parsing %s: %s", s, err)
		}
		result := expr.AppendString(nil)
		if string(result) != resultExpected {
			t.Fatalf("unexpected result when marshaling %s;\ngot\n%s\nwant\n%s", s, result, resultExpected)
		}
	}
	same := func(s string) {
		t.Helper()
		another(s, s)
	}
	// Metric expressions
	same("a")
	same("foo.bar.baz")
	same("foo.bar.baz:aa:bb")
	same("fOO.*.b[a-z]R{aa*,bb}s_s.$aaa")
	same("*")
	same("*.foo")
	same("{x,y}.z")
	same("[x-zaBc]DeF")
	another(`\f\ oo`, `f\ oo`)
	another(`f\x1B\x3a`, `f\x1b:`)

	// booleans
	same("True")
	same("False")
	another("true", "True")
	another("faLSe", "False")

	// Numbers
	same("123")
	same("-123")
	another("+123", "123")
	same("12.3")
	same("-1.23")
	another("+1.23", "1.23")
	another("123e5", "1.23e+07")
	another("-123e5", "-1.23e+07")
	another("+123e5", "1.23e+07")
	another("1.23E5", "123000")
	another("-1.23e5", "-123000")
	another("+1.23e5", "123000")
	another("0xab", "171")
	another("0b1011101", "93")
	another("0O12345", "5349")

	// strings
	another(`"foo'"`, `'foo\''`)
	same(`'fo\'o"b\\ar'`)
	another(`"f\\oo\.bar\1"`, `'f\\oo\\.bar\\1'`)

	// function calls
	same("foo()")
	another("foo(bar,)", "foo(bar)")
	same("foo(bar,123,'baz')")
	another("foo(foo(bar), BAZ = xx ( 123, x))", `foo(foo(bar),BAZ=xx(123,x))`)

	// chained functions
	another("foo | bar", "foo|bar")
	same("foo|bar|baz")
	same("foo(sss)|bar(aa)|xxx.ss")
	another(`foo|bar(1,"sdf")`, `foo|bar(1,'sdf')`)

	// mix
	same(`f(a,xx=b|c|aa(124,'456'),aa=bb)`)
}

func TestParseFailure(t *testing.T) {
	f := func(s string) {
		t.Helper()
		expr, err := Parse(s)
		if err == nil {
			t.Fatalf("expecting error when parsing %s", s)
		}
		if expr != nil {
			t.Fatalf("expecting nil expr when parsing %s; got %s", s, expr.AppendString(nil))
		}
	}
	f("")
	f("'asdf")
	f("foo bar")
	f("f(a")
	f("f(1.2.3")
	f("foo|bar(")
	f("+foo")
	f("-bar")
	f("123 '")
	f("f '")
	f("f|")
	f("f|'")
	f("f|123")
	f("f('")
	f("f(f()=123)")
	f("f(a=')")
	f("f(a=foo(")
	f("f()'")
}
