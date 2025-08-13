package envtemplate

import (
	"reflect"
	"sort"
	"testing"
)

func TestExpandTemplates(t *testing.T) {
	f := func(envs, resultExpected []string) {
		t.Helper()
		m := parseEnvVars(envs)
		mExpanded := expandTemplates(m)
		result := make([]string, 0, len(mExpanded))
		for k, v := range mExpanded {
			result = append(result, k+"="+v)
		}
		sort.Strings(result)
		if !reflect.DeepEqual(result, resultExpected) {
			t.Fatalf("unexpected result;\ngot\n%q\nwant\n%q", result, resultExpected)
		}
	}
	f(nil, []string{})
	f([]string{"foo=%{bar}", "bar=x"}, []string{"bar=x", "foo=x"})
	f([]string{"a=x%{b}", "b=y%{c}z%{d}", "c=123", "d=qwe"}, []string{"a=xy123zqwe", "b=y123zqwe", "c=123", "d=qwe"})
	f([]string{"a=x%{b}y", "b=z%{a}q", "c"}, []string{"a=xzxzxzxz%{a}qyqyqyqy", "b=zxzxzxzx%{b}yqyqyqyq", "c="})
	f([]string{"a=%{x.y}", "x.y=test"}, []string{"a=test", "x.y=test"})
	f([]string{"a=%{x y}"}, []string{"a=%{x y}"})
	f([]string{"a=%{123}"}, []string{"a=%{123}"})
}

func TestLookupEnv(t *testing.T) {
	envVars = map[string]string{
		"foo": "bar",
	}
	result, ok := LookupEnv("foo")
	if result != "bar" {
		t.Fatalf("unexpected result; got %q; want %q", result, "bar")
	}
	if !ok {
		t.Fatalf("unexpected ok=false")
	}
	result, ok = LookupEnv("bar")
	if result != "" {
		t.Fatalf("unexpected non-empty result: %q", result)
	}
	if ok {
		t.Fatalf("unexpected ok=true")
	}
}

func TestReplaceSuccess(t *testing.T) {
	envVars = map[string]string{
		"foo":       "bar",
		"foo.bar_1": "baz",
		"foo-bar_2": "test",
	}
	f := func(s, resultExpected string) {
		t.Helper()
		result := ReplaceBytes([]byte(s))
		if string(result) != resultExpected {
			t.Fatalf("unexpected result for ReplaceBytes(%q);\ngot\n%q\nwant\n%q", s, result, resultExpected)
		}
		resultS := ReplaceString(s)
		if resultS != resultExpected {
			t.Fatalf("unexpected result for ReplaceString(%q);\ngot\n%q\nwant\n%q", s, result, resultExpected)
		}
	}

	// Zero placeholders
	f("", "")
	f("foo", "foo")

	// Matching placeholders
	f("a %{foo}-x", "a bar-x")
	f("%{foo.bar_1}", "baz")
	f("qq.%{foo-bar_2}.ww", "qq.test.ww")

	// Missing placeholders
	f("foo %{bar} %{baz} %{foo}", "foo %{bar} %{baz} bar")
}
