package logstorage

import (
	"reflect"
	"testing"
)

func TestPatternApply(t *testing.T) {
	f := func(patternStr, s string, resultsExpected []string) {
		t.Helper()

		checkFields := func(ptn *pattern) {
			t.Helper()
			if len(ptn.fields) != len(resultsExpected) {
				t.Fatalf("unexpected number of results; got %d; want %d", len(ptn.fields), len(resultsExpected))
			}
			for i, f := range ptn.fields {
				if v := *f.value; v != resultsExpected[i] {
					t.Fatalf("unexpected value for field %q; got %q; want %q", f.name, v, resultsExpected[i])
				}
			}
		}

		ptn, err := parsePattern(patternStr)
		if err != nil {
			t.Fatalf("cannot parse %q: %s", patternStr, err)
		}
		ptn.apply(s)
		checkFields(ptn)

		// clone pattern and check fields again
		ptnCopy := ptn.clone()
		ptnCopy.apply(s)
		checkFields(ptn)
	}

	f("<foo>", "", []string{""})
	f("<foo>", "abc", []string{"abc"})
	f("<foo>bar", "", []string{""})
	f("<foo>bar", "bar", []string{""})
	f("<foo>bar", "bazbar", []string{"baz"})
	f("<foo>bar", "a bazbar xdsf", []string{"a baz"})
	f("<foo>bar<>", "a bazbar xdsf", []string{"a baz"})
	f("<foo>bar<>x", "a bazbar xdsf", []string{"a baz"})
	f("foo<bar>", "", []string{""})
	f("foo<bar>", "foo", []string{""})
	f("foo<bar>", "a foo xdf sdf", []string{" xdf sdf"})
	f("foo<bar>", "a foo foobar", []string{" foobar"})
	f("foo<bar>baz", "a foo foobar", []string{""})
	f("foo<bar>baz", "a foobaz bar", []string{""})
	f("foo<bar>baz", "a foo foobar baz", []string{" foobar "})
	f("foo<bar>baz", "a foo foobar bazabc", []string{" foobar "})

	f("ip=<ip> <> path=<path> ", "x=a, ip=1.2.3.4 method=GET host='abc' path=/foo/bar some tail here", []string{"1.2.3.4", "/foo/bar"})

	// escaped pattern
	f("ip=&lt;<ip>&gt;", "foo ip=<1.2.3.4> bar", []string{"1.2.3.4"})
	f("ip=&lt;<ip>&gt;", "foo ip=<foo&amp;bar> bar", []string{"foo&amp;bar"})

	// quoted fields
	f(`"msg":<msg>,`, `{"foo":"bar","msg":"foo,b\"ar\n\t","baz":"x"}`, []string{`foo,b"ar` + "\n\t"})
	f(`foo=<bar>`, "foo=`bar baz,abc` def", []string{"bar baz,abc"})
	f(`foo=<bar> `, "foo=`bar baz,abc` def", []string{"bar baz,abc"})
	f(`<foo>`, `"foo,\"bar"`, []string{`foo,"bar`})
	f(`<foo>,"bar`, `"foo,\"bar"`, []string{`foo,"bar`})

	// disable automatic unquoting of quoted field
	f(`[<plain:foo>]`, `["foo","bar"]`, []string{`"foo","bar"`})
}

func TestParsePatternFailure(t *testing.T) {
	f := func(patternStr string) {
		t.Helper()

		ptn, err := parsePattern(patternStr)
		if err == nil {
			t.Fatalf("expecting error when parsing %q; got %v", patternStr, ptn)
		}
	}

	// Missing named fields
	f("")
	f("foobar")
	f("<>")
	f("<>foo<>bar")

	// Missing delimiter between fields
	f("<foo><bar>")
	f("abc<foo><bar>def")
	f("abc<foo><bar>")
	f("abc<foo><_>")
	f("abc<_><_>")
}

func TestParsePatternStepsSuccess(t *testing.T) {
	f := func(s string, stepsExpected []patternStep) {
		t.Helper()

		steps, err := parsePatternSteps(s)
		if err != nil {
			t.Fatalf("unexpected error when parsing %q: %s", s, err)
		}
		if !reflect.DeepEqual(steps, stepsExpected) {
			t.Fatalf("unexpected steps for [%s]; got %v; want %v", s, steps, stepsExpected)
		}
	}

	f("", nil)

	f("foobar", []patternStep{
		{
			prefix: "foobar",
		},
	})

	f("<>", []patternStep{
		{},
	})

	f("foo<>", []patternStep{
		{
			prefix: "foo",
		},
	})

	f("<foo><bar>", []patternStep{
		{
			field: "foo",
		},
		{
			field: "bar",
		},
	})

	f("<foo>", []patternStep{
		{
			field: "foo",
		},
	})
	f("<foo>bar", []patternStep{
		{
			field: "foo",
		},
		{
			prefix: "bar",
		},
	})
	f("<>bar<foo>", []patternStep{
		{},
		{
			prefix: "bar",
			field:  "foo",
		},
	})
	f("bar<foo>", []patternStep{
		{
			prefix: "bar",
			field:  "foo",
		},
	})
	f("bar<foo>abc", []patternStep{
		{
			prefix: "bar",
			field:  "foo",
		},
		{
			prefix: "abc",
		},
	})
	f("bar<foo>abc<_>", []patternStep{
		{
			prefix: "bar",
			field:  "foo",
		},
		{
			prefix: "abc",
		},
	})
	f("<foo>bar<baz>", []patternStep{
		{
			field: "foo",
		},
		{
			prefix: "bar",
			field:  "baz",
		},
	})
	f("bar<foo>baz", []patternStep{
		{
			prefix: "bar",
			field:  "foo",
		},
		{
			prefix: "baz",
		},
	})
	f("&lt;&amp;&gt;", []patternStep{
		{
			prefix: "<&>",
		},
	})
	f("&lt;< foo >&amp;gt;", []patternStep{
		{
			prefix: "<",
			field:  "foo",
		},
		{
			prefix: "&gt;",
		},
	})
	f("< q : foo >bar<plain : baz:c:y>f<:foo:bar:baz>", []patternStep{
		{
			field:    "foo",
			fieldOpt: "q",
		},
		{
			prefix:   "bar",
			field:    "baz:c:y",
			fieldOpt: "plain",
		},
		{
			prefix: "f",
			field:  "foo:bar:baz",
		},
	})

}

func TestParsePatternStepsFailure(t *testing.T) {
	f := func(s string) {
		t.Helper()

		steps, err := parsePatternSteps(s)
		if err == nil {
			t.Fatalf("expecting non-nil error when parsing %q; got steps: %v", s, steps)
		}
	}

	// missing >
	f("<foo")
	f("foo<bar")
}
