package regexutil

import (
	"regexp"
	"testing"
)

func TestPromRegexParseFailure(t *testing.T) {
	f := func(expr string) {
		t.Helper()

		pr, err := NewPromRegex(expr)
		if err == nil {
			t.Fatalf("expecting non-nil error for expr=%s", expr)
		}
		if pr != nil {
			t.Fatalf("expecting nil pr for expr=%s", expr)
		}
	}
	f("fo[bar")
	f("foo(bar")
}

func TestPromRegex(t *testing.T) {
	f := func(expr, s string, resultExpected bool) {
		t.Helper()

		pr, err := NewPromRegex(expr)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		exprResult := pr.String()
		if exprResult != expr {
			t.Fatalf("unexpected string representation for %q: %q", expr, exprResult)
		}
		result := pr.MatchString(s)
		if result != resultExpected {
			t.Fatalf("unexpected result when matching %q against %q; got %v; want %v", expr, s, result, resultExpected)
		}

		// Make sure the result is the same for regular regexp
		exprAnchored := "^(?:" + expr + ")$"
		re := regexp.MustCompile(exprAnchored)
		result = re.MatchString(s)
		if result != resultExpected {
			t.Fatalf("unexpected result when matching %q against %q during sanity check; got %v; want %v", exprAnchored, s, result, resultExpected)
		}
	}

	f("", "", true)
	f("", "foo", false)
	f("foo", "", false)
	f(".*", "", true)
	f(".*", "foo", true)
	f(".+", "", false)
	f(".+", "foo", true)
	f("foo.*", "bar", false)
	f("foo.*", "foo", true)
	f("foo.*", "foobar", true)
	f("foo.+", "bar", false)
	f("foo.+", "foo", false)
	f("foo.+", "foobar", true)
	f("foo|bar", "", false)
	f("foo|bar", "a", false)
	f("foo|bar", "foo", true)
	f("foo|bar", "bar", true)
	f("foo|bar", "foobar", false)
	f("foo(bar|baz)", "a", false)
	f("foo(bar|baz)", "foobar", true)
	f("foo(bar|baz)", "foobaz", true)
	f("foo(bar|baz)", "foobaza", false)
	f("foo(bar|baz)", "foobal", false)
	f("^foo|b(ar)$", "foo", true)
	f("^foo|b(ar)$", "bar", true)
	f("^foo|b(ar)$", "ar", false)
	f(".*foo.*", "foo", true)
	f(".*foo.*", "afoobar", true)
	f(".*foo.*", "abc", false)
	f("foo.*bar.*", "foobar", true)
	f("foo.*bar.*", "foo_bar_", true)
	f("foo.*bar.*", "foobaz", false)
	f(".+foo.+", "foo", false)
	f(".+foo.+", "afoobar", true)
	f(".+foo.+", "afoo", false)
	f(".+foo.+", "abc", false)
	f("foo.+bar.+", "foobar", false)
	f("foo.+bar.+", "foo_bar_", true)
	f("foo.+bar.+", "foobaz", false)
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

	// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/5297
	f(".+;|;.+", ";", false)
	f(".+;|;.+", "foo", false)
	f(".+;|;.+", "foo;bar", false)
	f(".+;|;.+", "foo;", true)
	f(".+;|;.+", ";foo", true)
	f(".+foo|bar|baz.+", "foo", false)
	f(".+foo|bar|baz.+", "afoo", true)
	f(".+foo|bar|baz.+", "fooa", false)
	f(".+foo|bar|baz.+", "afooa", false)
	f(".+foo|bar|baz.+", "bar", true)
	f(".+foo|bar|baz.+", "abar", false)
	f(".+foo|bar|baz.+", "abara", false)
	f(".+foo|bar|baz.+", "bara", false)
	f(".+foo|bar|baz.+", "baz", false)
	f(".+foo|bar|baz.+", "baza", true)
	f(".+foo|bar|baz.+", "abaz", false)
	f(".+foo|bar|baz.+", "abaza", false)
	f(".+foo|bar|baz.+", "afoo|bar|baza", false)
	f(".+(foo|bar|baz).+", "abara", true)
	f(".+(foo|bar|baz).+", "afooa", true)
	f(".+(foo|bar|baz).+", "abaza", true)

	f(".*;|;.*", ";", true)
	f(".*;|;.*", "foo", false)
	f(".*;|;.*", "foo;bar", false)
	f(".*;|;.*", "foo;", true)
	f(".*;|;.*", ";foo", true)

	f(".*foo(bar|baz)", "fooxfoobaz", true)
	f(".*foo(bar|baz)", "fooxfooban", false)
	f(".*foo(bar|baz)", "fooxfooban foobar", true)
}
