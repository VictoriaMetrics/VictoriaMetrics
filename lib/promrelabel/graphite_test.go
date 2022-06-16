package promrelabel

import (
	"reflect"
	"testing"
)

func TestGraphiteTemplateMatchExpand(t *testing.T) {
	f := func(matchTpl, s, replaceTpl, resultExpected string) {
		t.Helper()
		gmt := newGraphiteMatchTemplate(matchTpl)
		matches, ok := gmt.Match(nil, s)
		if !ok {
			matches = nil
		}
		grt := newGraphiteReplaceTemplate(replaceTpl)
		result := grt.Expand(nil, matches)
		if string(result) != resultExpected {
			t.Fatalf("unexpected result; got %q; want %q", result, resultExpected)
		}
	}
	f("", "", "", "")
	f("test.*.*.counter", "test.foo.bar.counter", "${2}_total", "bar_total")
	f("test.*.*.counter", "test.foo.bar.counter", "$1_total", "foo_total")
	f("test.*.*.counter", "test.foo.bar.counter", "total_$0", "total_test.foo.bar.counter")
	f("test.dispatcher.*.*.*", "test.dispatcher.foo.bar.baz", "$3-$2-$1", "baz-bar-foo")
	f("*.signup.*.*", "foo.signup.bar.baz", "$1-${3}_$2_total", "foo-baz_bar_total")
}

func TestGraphiteMatchTemplateMatch(t *testing.T) {
	f := func(tpl, s string, matchesExpected []string, okExpected bool) {
		t.Helper()
		gmt := newGraphiteMatchTemplate(tpl)
		tplGot := gmt.String()
		if tplGot != tpl {
			t.Fatalf("unexpected template; got %q; want %q", tplGot, tpl)
		}
		matches, ok := gmt.Match(nil, s)
		if ok != okExpected {
			t.Fatalf("unexpected ok result for tpl=%q, s=%q; got %v; want %v", tpl, s, ok, okExpected)
		}
		if okExpected {
			if !reflect.DeepEqual(matches, matchesExpected) {
				t.Fatalf("unexpected matches for tpl=%q, s=%q; got\n%q\nwant\n%q\ngraphiteMatchTemplate=%v", tpl, s, matches, matchesExpected, gmt)
			}
		}
	}
	f("", "", []string{""}, true)
	f("", "foobar", nil, false)
	f("foo", "foo", []string{"foo"}, true)
	f("foo", "", nil, false)
	f("foo.bar.baz", "foo.bar.baz", []string{"foo.bar.baz"}, true)
	f("*", "foobar", []string{"foobar", "foobar"}, true)
	f("**", "foobar", nil, false)
	f("*", "foo.bar", nil, false)
	f("*foo", "barfoo", []string{"barfoo", "bar"}, true)
	f("*foo", "foo", []string{"foo", ""}, true)
	f("*foo", "bar.foo", nil, false)
	f("foo*", "foobar", []string{"foobar", "bar"}, true)
	f("foo*", "foo", []string{"foo", ""}, true)
	f("foo*", "foo.bar", nil, false)
	f("foo.*", "foobar", nil, false)
	f("foo.*", "foo.bar", []string{"foo.bar", "bar"}, true)
	f("foo.*", "foo.bar.baz", nil, false)
	f("*.*.baz", "foo.bar.baz", []string{"foo.bar.baz", "foo", "bar"}, true)
	f("*.bar", "foo.bar.baz", nil, false)
	f("*.bar", "foo.baz", nil, false)
}

func TestGraphiteReplaceTemplateExpand(t *testing.T) {
	f := func(tpl string, matches []string, resultExpected string) {
		t.Helper()
		grt := newGraphiteReplaceTemplate(tpl)
		tplGot := grt.String()
		if tplGot != tpl {
			t.Fatalf("unexpected template; got %q; want %q", tplGot, tpl)
		}
		result := grt.Expand(nil, matches)
		if string(result) != resultExpected {
			t.Fatalf("unexpected result for tpl=%q; got\n%q\nwant\n%q\ngraphiteReplaceTemplate=%v", tpl, result, resultExpected, grt)
		}
	}
	f("", nil, "")
	f("foo", nil, "foo")
	f("$", nil, "$")
	f("$1", nil, "$1")
	f("${123", nil, "${123")
	f("${123}", nil, "${123}")
	f("${foo}45$sdf$3", nil, "${foo}45$sdf$3")
	f("$1", []string{"foo", "bar"}, "bar")
	f("$0-$1", []string{"foo", "bar"}, "foo-bar")
	f("x-${0}-$1", []string{"foo", "bar"}, "x-foo-bar")
}
