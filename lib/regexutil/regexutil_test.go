package regexutil

import (
	"reflect"
	"testing"
)

func TestGetOrValues(t *testing.T) {
	f := func(s string, valuesExpected []string) {
		t.Helper()
		values := GetOrValues(s)
		if !reflect.DeepEqual(values, valuesExpected) {
			t.Fatalf("unexpected values for s=%q; got %q; want %q", s, values, valuesExpected)
		}
	}

	f("", []string{""})
	f("foo", []string{"foo"})
	f("^foo$", []string{"foo"})
	f("|foo", []string{"", "foo"})
	f("|foo|", []string{"", "", "foo"})
	f("foo.+", nil)
	f("foo.*", nil)
	f(".*", nil)
	f("foo|.*", nil)
	f("(fo((o)))|(bar)", []string{"bar", "foo"})
	f("foobar", []string{"foobar"})
	f("z|x|c", []string{"c", "x", "z"})
	f("foo|bar", []string{"bar", "foo"})
	f("(foo|bar)", []string{"bar", "foo"})
	f("(foo|bar)baz", []string{"barbaz", "foobaz"})
	f("[a-z][a-z]", nil)
	f("[a-d]", []string{"a", "b", "c", "d"})
	f("x[a-d]we", []string{"xawe", "xbwe", "xcwe", "xdwe"})
	f("foo(bar|baz)", []string{"foobar", "foobaz"})
	f("foo(ba[rz]|(xx|o))", []string{"foobar", "foobaz", "fooo", "fooxx"})
	f("foo(?:bar|baz)x(qwe|rt)", []string{"foobarxqwe", "foobarxrt", "foobazxqwe", "foobazxrt"})
	f("foo(bar||baz)", []string{"foo", "foobar", "foobaz"})
	f("(a|b|c)(d|e|f|0|1|2)(g|h|k|x|y|z)", nil)
	f("(?i)foo", nil)
	f("(?i)(foo|bar)", nil)
	f("^foo|bar$", []string{"bar", "foo"})
	f("^(foo|bar)$", []string{"bar", "foo"})
	f("^a(foo|b(?:a|r))$", []string{"aba", "abr", "afoo"})
	f("^a(foo$|b(?:a$|r))$", []string{"aba", "abr", "afoo"})
	f("^a(^foo|bar$)z$", nil)
}

func TestSimplify(t *testing.T) {
	f := func(s, expectedPrefix, expectedSuffix string) {
		t.Helper()
		prefix, suffix := Simplify(s)
		if prefix != expectedPrefix {
			t.Fatalf("unexpected prefix for s=%q; got %q; want %q", s, prefix, expectedPrefix)
		}
		if suffix != expectedSuffix {
			t.Fatalf("unexpected suffix for s=%q; got %q; want %q", s, suffix, expectedSuffix)
		}
	}

	f("", "", "")
	f("^", "", "")
	f("$", "", "")
	f("^()$", "", "")
	f("^(?:)$", "", "")
	f("^foo|^bar$|baz", "", "foo|ba[rz]")
	f("^(foo$|^bar)$", "", "foo|bar")
	f("^a(foo$|bar)$", "a", "foo|bar")
	f("^a(^foo|bar$)z$", "a", "(?-m:(?:\\Afoo|bar$)z)")
	f("foobar", "foobar", "")
	f("foo$|^foobar", "foo", "|bar")
	f("^(foo$|^foobar)$", "foo", "|bar")
	f("foobar|foobaz", "fooba", "[rz]")
	f("(fo|(zar|bazz)|x)", "", "fo|zar|bazz|x")
	f("(тестЧЧ|тест)", "тест", "ЧЧ|")
	f("foo(bar|baz|bana)", "fooba", "[rz]|na")
	f("^foobar|foobaz", "fooba", "[rz]")
	f("^foobar|^foobaz$", "fooba", "[rz]")
	f("foobar|foobaz", "fooba", "[rz]")
	f("(?:^foobar|^foobaz)aa.*", "fooba", "(?-s:[rz]aa.*)")
	f("foo[bar]+", "foo", "[abr]+")
	f("foo[a-z]+", "foo", "[a-z]+")
	f("foo[bar]*", "foo", "[abr]*")
	f("foo[a-z]*", "foo", "[a-z]*")
	f("foo[x]+", "foo", "x+")
	f("foo[^x]+", "foo", "[^x]+")
	f("foo[x]*", "foo", "x*")
	f("foo[^x]*", "foo", "[^x]*")
	f("foo[x]*bar", "foo", "x*bar")
	f("fo\\Bo[x]*bar?", "fo", "\\Box*bar?")
	f("foo.+bar", "foo", "(?-s:.+bar)")
	f("a(b|c.*).+", "a", "(?-s:(?:b|c.*).+)")
	f("ab|ac", "a", "[bc]")
	f("(?i)xyz", "", "(?i:XYZ)")
	f("(?i)foo|bar", "", "(?i:FOO|BAR)")
	f("(?i)up.+x", "", "(?i-s:UP.+X)")
	f("(?smi)xy.*z$", "", "(?ims:XY.*Z$)")

	// test invalid regexps
	f("a(", "a(", "")
	f("a[", "a[", "")
	f("a[]", "a[]", "")
	f("a{", "a{", "")
	f("a{}", "a{}", "")
	f("invalid(regexp", "invalid(regexp", "")

	// The transformed regexp mustn't match aba
	f("a?(^ba|c)", "", "a?(?:\\Aba|c)")

	// The transformed regexp mustn't match barx
	f("(foo|bar$)x*", "", "(?-m:(?:foo|bar$)x*)")

	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/5297
	f(".+;|;.+", "", "(?-s:.+;|;.+)")
	f("^(.+);|;(.+)$", "", "(?-s:.+;|;.+)")
	f("^(.+);$|^;(.+)$", "", "(?-s:.+;|;.+)")
	f(".*;|;.*", "", "(?-s:.*;|;.*)")
	f("^(.*);|;(.*)$", "", "(?-s:.*;|;.*)")
	f("^(.*);$|^;(.*)$", "", "(?-s:.*;|;.*)")
}

func TestRemoveStartEndAnchors(t *testing.T) {
	f := func(s, resultExpected string) {
		t.Helper()
		result := RemoveStartEndAnchors(s)
		if result != resultExpected {
			t.Fatalf("unexpected result for RemoveStartEndAnchors(%q); got %q; want %q", s, result, resultExpected)
		}
	}
	f("", "")
	f("a", "a")
	f("^^abc", "abc")
	f("a^b$c", "a^b$c")
	f("$$abc^", "$$abc^")
	f("^abc|de$", "abc|de")
	f("abc\\$", "abc\\$")
	f("^abc\\$$$", "abc\\$")
	f("^a\\$b\\$$", "a\\$b\\$")
}
