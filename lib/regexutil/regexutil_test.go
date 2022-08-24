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
	// This is incorrect conversion, because the regexp matches nothing.
	// It is OK for now, since such regexps are uncommon in practice.
	// TODO: properly handle this case.
	f("^a(^foo|bar$)z$", []string{"abarz", "afooz"})
}
