package logstorage

import (
	"reflect"
	"testing"
)

func TestExtractFormatApply(t *testing.T) {
	f := func(format, s string, resultsExpected []string) {
		t.Helper()

		ef, err := parseExtractFormat(format)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		ef.apply(s)

		if !reflect.DeepEqual(ef.results, resultsExpected) {
			t.Fatalf("unexpected results; got %q; want %q", ef.results, resultsExpected)
		}
	}

	f("<foo>", "", []string{""})
	f("<foo>", "abc", []string{"abc"})
	f("<foo>bar", "", []string{"", ""})
	f("<foo>bar", "bar", []string{"", ""})
	f("<foo>bar", "bazbar", []string{"baz", ""})
	f("<foo>bar", "a bazbar xdsf", []string{"a baz", " xdsf"})
	f("foo<bar>", "", []string{""})
	f("foo<bar>", "foo", []string{""})
	f("foo<bar>", "a foo xdf sdf", []string{" xdf sdf"})
	f("foo<bar>", "a foo foobar", []string{" foobar"})
	f("foo<bar>baz", "a foo foobar", []string{"", ""})
	f("foo<bar>baz", "a foo foobar baz", []string{" foobar ", ""})
	f("foo<bar>baz", "a foo foobar bazabc", []string{" foobar ", "abc"})
	f("ip=<ip> <> path=<path> ", "x=a, ip=1.2.3.4 method=GET host='abc' path=/foo/bar some tail here", []string{"1.2.3.4", "method=GET host='abc'", "/foo/bar", "some tail here"})
	f("ip=&lt;<ip>&gt;", "foo ip=<1.2.3.4> bar", []string{"1.2.3.4", " bar"})

	// quoted fields
	f(`"msg":<msg>,`, `{"foo":"bar","msg":"foo,b\"ar\n\t","baz":"x"}`, []string{`foo,b"ar`+"\n\t", `"baz":"x"}`})
	f(`foo=<bar>`, "foo=`bar baz,abc` def", []string{"bar baz,abc"})
	f(`foo=<bar> `, "foo=`bar baz,abc` def", []string{"bar baz,abc", "def"})
}

func TestParseExtractFormatStepsSuccess(t *testing.T) {
	f := func(s string, stepsExpected []*extractFormatStep) {
		t.Helper()

		steps, err := parseExtractFormatSteps(s)
		if err != nil {
			t.Fatalf("unexpected error when parsing %q: %s", s, err)
		}
		if !reflect.DeepEqual(steps, stepsExpected) {
			t.Fatalf("unexpected steps for [%s]; got %v; want %v", s, steps, stepsExpected)
		}
	}

	f("<foo>", []*extractFormatStep{
		{
			field: "foo",
		},
	})
	f("<_>", []*extractFormatStep{
		{},
	})
	f("<*>", []*extractFormatStep{
		{},
	})
	f("<>", []*extractFormatStep{
		{},
	})
	f("<foo>bar", []*extractFormatStep{
		{
			field: "foo",
		},
		{
			prefix: "bar",
		},
	})
	f("<>bar<foo>", []*extractFormatStep{
		{},
		{
			prefix: "bar",
			field:     "foo",
		},
	})
	f("bar<foo>", []*extractFormatStep{
		{
			prefix: "bar",
			field:     "foo",
		},
	})
	f("bar<foo>abc", []*extractFormatStep{
		{
			prefix: "bar",
			field:     "foo",
		},
		{
			prefix: "abc",
		},
	})
	f("bar<foo>abc<_>", []*extractFormatStep{
		{
			prefix: "bar",
			field:     "foo",
		},
		{
			prefix: "abc",
		},
	})
	f("<foo>bar<baz>", []*extractFormatStep{
		{
			field: "foo",
		},
		{
			prefix: "bar",
			field:     "baz",
		},
	})
	f("bar<foo>baz", []*extractFormatStep{
		{
			prefix: "bar",
			field:     "foo",
		},
		{
			prefix: "baz",
		},
	})
	f("&lt;<foo>&amp;gt;", []*extractFormatStep{
		{
			prefix: "<",
			field:     "foo",
		},
		{
			prefix: "&gt;",
		},
	})
}

func TestParseExtractFormatStepFailure(t *testing.T) {
	f := func(s string) {
		t.Helper()

		_, err := parseExtractFormatSteps(s)
		if err == nil {
			t.Fatalf("expecting non-nil error when parsing %q", s)
		}
	}

	// empty string
	f("")

	// zero fields
	f("foobar")

	// missing delimiter between fields
	f("<foo><bar>")
	f("aa<foo><bar>")
	f("aa<foo><bar>bb")

	// missing >
	f("<foo")
	f("foo<bar")
}
