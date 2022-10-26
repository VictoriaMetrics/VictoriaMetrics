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
		"foo": "bar",
	}
	f := func(s, resultExpected string) {
		t.Helper()
		result, err := ReplaceBytes([]byte(s))
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		if string(result) != resultExpected {
			t.Fatalf("unexpected result for ReplaceBytes(%q);\ngot\n%q\nwant\n%q", s, result, resultExpected)
		}
		resultS, err := ReplaceString(s)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		if resultS != resultExpected {
			t.Fatalf("unexpected result for ReplaceString(%q);\ngot\n%q\nwant\n%q", s, result, resultExpected)
		}
	}
	f("", "")
	f("foo", "foo")
	f("a %{foo}-x", "a bar-x")
}

func TestReplaceFailure(t *testing.T) {
	f := func(s string) {
		t.Helper()
		if _, err := ReplaceBytes([]byte(s)); err == nil {
			t.Fatalf("expecting non-nil error for ReplaceBytes(%q)", s)
		}
		if _, err := ReplaceString(s); err == nil {
			t.Fatalf("expecting non-nil error for ReplaceString(%q)", s)
		}
	}
	f("foo %{bar} %{baz}")
}
