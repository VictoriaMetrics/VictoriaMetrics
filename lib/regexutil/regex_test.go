package regexutil

import (
	"reflect"
	"testing"
)

func TestNewRegexFailure(t *testing.T) {
	f := func(expr string) {
		t.Helper()

		r, err := NewRegex(expr)
		if err == nil {
			t.Fatalf("expecting non-nil error when parsing %q; got %q", expr, r)
		}
	}

	f("[foo")
	f("(foo")
}

func TestRegexMatchString(t *testing.T) {
	f := func(expr, s string, resultExpected bool) {
		t.Helper()

		r, err := NewRegex(expr)
		if err != nil {
			t.Fatalf("cannot parse %q: %s", expr, err)
		}
		exprResult := r.String()
		if exprResult != expr {
			t.Fatalf("unexpected string representation for %q: %q", expr, exprResult)
		}
		result := r.MatchString(s)
		if result != resultExpected {
			t.Fatalf("unexpected result when matching %q against regex=%q; got %v; want %v", s, expr, result, resultExpected)
		}
	}

	f("", "", true)
	f("", "foo", true)
	f("foo", "", false)
	f(".*", "", true)
	f(".*", "foo", true)
	f(".+", "", false)
	f(".+", "foo", true)
	f("foo.*", "bar", false)
	f("foo.*", "foo", true)
	f("foo.*", "a foo", true)
	f("foo.*", "a foo a", true)
	f("foo.*", "foobar", true)
	f("foo.*", "a foobar", true)
	f("foo.+", "bar", false)
	f("foo.+", "foo", false)
	f("foo.+", "a foo", false)
	f("foo.+", "foobar", true)
	f("foo.+", "a foobar", true)
	f("foo|bar", "", false)
	f("foo|bar", "a", false)
	f("foo|bar", "foo", true)
	f("foo|bar", "a foo", true)
	f("foo|bar", "foo a", true)
	f("foo|bar", "a foo a", true)
	f("foo|bar", "bar", true)
	f("foo|bar", "foobar", true)
	f("foo(bar|baz)", "a", false)
	f("foo(bar|baz)", "foobar", true)
	f("foo(bar|baz)", "foobaz", true)
	f("foo(bar|baz)", "foobaza", true)
	f("foo(bar|baz)", "a foobaz a", true)
	f("foo(bar|baz)", "foobal", false)
	f("^foo|b(ar)$", "foo", true)
	f("^foo|b(ar)$", "foo a", true)
	f("^foo|b(ar)$", "a foo", false)
	f("^foo|b(ar)$", "bar", true)
	f("^foo|b(ar)$", "a bar", true)
	f("^foo|b(ar)$", "barz", false)
	f("^foo|b(ar)$", "ar", false)
	f(".*foo.*", "foo", true)
	f(".*foo.*", "afoobar", true)
	f(".*foo.*", "abc", false)
	f("foo.*bar.*", "foobar", true)
	f("foo.*bar.*", "foo_bar_", true)
	f("foo.*bar.*", "a foo bar baz", true)
	f("foo.*bar.*", "foobaz", false)
	f("foo.*bar.*", "baz foo", false)
	f(".+foo.+", "foo", false)
	f(".+foo.+", "afoobar", true)
	f(".+foo.+", "afoo", false)
	f(".+foo.+", "abc", false)
	f("foo.+bar.+", "foobar", false)
	f("foo.+bar.+", "foo_bar_", true)
	f("foo.+bar.+", "a foo_bar_", true)
	f("foo.+bar.+", "foobaz", false)
	f("foo.+bar.+", "abc", false)
	f(".+foo.*", "foo", false)
	f(".+foo.*", "afoo", true)
	f(".+foo.*", "afoobar", true)
	f(".*(a|b).*", "a", true)
	f(".*(a|b).*", "ax", true)
	f(".*(a|b).*", "xa", true)
	f(".*(a|b).*", "xay", true)
	f(".*(a|b).*", "xzy", false)
	f("^(?:true)$", "true", true)
	f("^(?:true)$", "false", false)

	f(".+;|;.+", ";", false)
	f(".+;|;.+", "foo", false)
	f(".+;|;.+", "foo;bar", true)
	f(".+;|;.+", "foo;", true)
	f(".+;|;.+", ";foo", true)
	f(".+foo|bar|baz.+", "foo", false)
	f(".+foo|bar|baz.+", "afoo", true)
	f(".+foo|bar|baz.+", "fooa", false)
	f(".+foo|bar|baz.+", "afooa", true)
	f(".+foo|bar|baz.+", "bar", true)
	f(".+foo|bar|baz.+", "abar", true)
	f(".+foo|bar|baz.+", "abara", true)
	f(".+foo|bar|baz.+", "bara", true)
	f(".+foo|bar|baz.+", "baz", false)
	f(".+foo|bar|baz.+", "baza", true)
	f(".+foo|bar|baz.+", "abaz", false)
	f(".+foo|bar|baz.+", "abaza", true)
	f(".+foo|bar|baz.+", "afoo|bar|baza", true)
	f(".+(foo|bar|baz).+", "bar", false)
	f(".+(foo|bar|baz).+", "bara", false)
	f(".+(foo|bar|baz).+", "abar", false)
	f(".+(foo|bar|baz).+", "abara", true)
	f(".+(foo|bar|baz).+", "afooa", true)
	f(".+(foo|bar|baz).+", "abaza", true)

	f(".*;|;.*", ";", true)
	f(".*;|;.*", "foo", false)
	f(".*;|;.*", "foo;bar", true)
	f(".*;|;.*", "foo;", true)
	f(".*;|;.*", ";foo", true)

	f("^bar", "foobarbaz", false)
	f("^foo", "foobarbaz", true)
	f("bar$", "foobarbaz", false)
	f("baz$", "foobarbaz", true)
	f("(bar$|^foo)", "foobarbaz", true)
	f("(bar$^boo)", "foobarbaz", false)
	f("foo(bar|baz)", "a fooxfoobaz a", true)
	f("foo(bar|baz)", "a fooxfooban a", false)
	f("foo(bar|baz)", "a fooxfooban foobar a", true)
}

func TestGetLiterals(t *testing.T) {
	f := func(expr string, literalsExpected []string) {
		t.Helper()

		r, err := NewRegex(expr)
		if err != nil {
			t.Fatalf("cannot parse %q: %s", expr, err)
		}
		literals := r.GetLiterals()
		if !reflect.DeepEqual(literals, literalsExpected) {
			t.Fatalf("unexpected literals; got %q; want %q", literals, literalsExpected)
		}
	}

	f("", nil)
	f("foo bar baz", []string{"foo bar baz"})
	f("foo.*bar(a|b)baz.+", []string{"foo", "bar", "baz"})
	f("(foo[ab](?:bar))", []string{"foo", "bar"})
	f("foo|bar", nil)
	f("(?i)foo", nil)
	f("foo((?i)bar)baz", []string{"foo", "baz"})
	f("((foo|bar)baz xxx(?:yzabc))", []string{"baz xxxyzabc"})
	f("((foo|bar)baz xxx(?:yzabc)*)", []string{"baz xxx"})
	f("((foo|bar)baz? xxx(?:yzabc)*)", []string{"ba", " xxx"})
}
