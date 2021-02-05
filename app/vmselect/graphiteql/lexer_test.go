package graphiteql

import (
	"reflect"
	"strings"
	"testing"
)

func TestScanStringSuccess(t *testing.T) {
	f := func(s, sExpected string) {
		t.Helper()
		result, err := scanString(s)
		if err != nil {
			t.Fatalf("unexpected error in scanString(%s): %s", s, err)
		}
		if result != sExpected {
			t.Fatalf("unexpected string scanned from %s; got %s; want %s", s, result, sExpected)
		}
		if !strings.HasPrefix(s, result) {
			t.Fatalf("invalid prefix for scanne string %s: %s", s, result)
		}
	}
	f(`""`, `""`)
	f(`''`, `''`)
	f(`""tail`, `""`)
	f(`''tail`, `''`)
	f(`"foo", bar`, `"foo"`)
	f(`'foo', bar`, `'foo'`)
	f(`"foo\.bar"`, `"foo\.bar"`)
	f(`"foo\"bar\1"\"`, `"foo\"bar\1"`)
	f(`"foo\\"bar\1"\"`, `"foo\\"`)
	f(`'foo\\'bar\1"\"`, `'foo\\'`)
}

func TestScanStringFailure(t *testing.T) {
	f := func(s string) {
		t.Helper()
		result, err := scanString(s)
		if err == nil {
			t.Fatalf("expecting non-nil error for scanString(%s)", s)
		}
		if result != "" {
			t.Fatalf("expecting empty result for scanString(%s); got %s", s, result)
		}
	}
	f(``)
	f(`"foo`)
	f(`'bar`)
}

func TestAppendEscapedIdent(t *testing.T) {
	f := func(s, resultExpected string) {
		t.Helper()
		result := appendEscapedIdent(nil, s)
		if string(result) != resultExpected {
			t.Fatalf("unexpected result; got\n%s\nwant\n%s", result, resultExpected)
		}
	}
	f("", "")
	f("a", "a")
	f("fo_o.$b-ar.b[a]*z{aa,bb}", "fo_o.$b-ar.b[a]*z{aa,bb}")
	f("a(b =C)", `a\(b\ \=C\)`)
}

func TestLexerSuccess(t *testing.T) {
	f := func(s string, tokensExpected []string) {
		t.Helper()
		var lex lexer
		var tokens []string
		lex.Init(s)
		for {
			if err := lex.Next(); err != nil {
				t.Fatalf("unexpected error: %s", err)
			}
			if isEOF(lex.Token) {
				break
			}
			tokens = append(tokens, lex.Token)
		}
		if !reflect.DeepEqual(tokens, tokensExpected) {
			t.Fatalf("unexpected tokens; got\n%q\nwant\n%q", tokens, tokensExpected)
		}
	}
	f("", nil)
	f("a", []string{"a"})
	f("*", []string{"*"})
	f("*.a", []string{"*.a"})
	f("[a-z]zx", []string{"[a-z]zx"})
	f("fo_o.$ba-r", []string{"fo_o.$ba-r"})
	f("{foo,bar}", []string{"{foo,bar}"})
	f("[a-z]", []string{"[a-z]"})
	f("fo*.bar10[s-z]as{aaa,bb}.ss{aa*ss,DS[c-D],ss}ss", []string{"fo*.bar10[s-z]as{aaa,bb}.ss{aa*ss,DS[c-D],ss}ss"})
	f("FOO.bar:avg", []string{"FOO.bar:avg"})
	f("FOO.bar\\:avg", []string{"FOO.bar\\:avg"})
	f(`foo.Bar|aaa(b,cd,e=aa)`, []string{"foo.Bar", "|", "aaa", "(", "b", ",", "cd", ",", "e", "=", "aa", ")"})
	f(`foo.Bar\|aaa\(b\,cd\,e\=aa\)`, []string{`foo.Bar\|aaa\(b\,cd\,e\=aa\)`})
	f(`123`, []string{`123`})
	f(`12.34`, []string{`12.34`})
	f(`12.34e4`, []string{`12.34e4`})
	f(`12.34e-4`, []string{`12.34e-4`})
	f(`12E+45`, []string{`12E+45`})
	f(`+12.34`, []string{`+`, `12.34`})
	f("0xABcd", []string{`0xABcd`})
	f("f(0o765,0b1101,0734,12.34)", []string{"f", "(", "0o765", ",", "0b1101", ",", "0734", ",", "12.34", ")"})
	f(`f ( foo, -.54e6,bar)`, []string{"f", "(", "foo", ",", "-", ".54e6", ",", "bar", ")"})
	f(`"foo(b'ar:baz)"`, []string{`"foo(b'ar:baz)"`})
	f(`'a"bc'`, []string{`'a"bc'`})
	f(`"f\"oo\\b"`, []string{`"f\"oo\\b"`})
	f(`a("b,c", 'de')`, []string{`a`, `(`, `"b,c"`, `,`, `'de'`, `)`})
}

func TestLexerError(t *testing.T) {
	f := func(s string) {
		t.Helper()
		var lex lexer
		lex.Init(s)
		for {
			if err := lex.Next(); err != nil {
				// Make sure lex.Next() consistently returns the error.
				if err1 := lex.Next(); err1 != err {
					t.Fatalf("unexpected error returned; got %v; want %v", err1, err)
				}
				return
			}
			if isEOF(lex.Token) {
				t.Fatalf("expecting non-nil error when parsing %q", s)
			}
		}
	}

	// Invalid identifier
	f("foo\\")
	f(`foo[bar`)
	f(`foo{bar`)
	f("~")
	f(",~")

	// Invalid string
	f(`"`)
	f(`"foo`)
	f(`'aa`)

	// Invalid number
	f(`0x`)
	f(`0o`)
	f(`-0b`)
	f(`13.`)
	f(`1e`)
	f(`1E+`)
	f(`1.3e`)
}
