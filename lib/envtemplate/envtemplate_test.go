package envtemplate

import (
	"os"
	"testing"
)

func TestReplaceSuccess(t *testing.T) {
	if err := os.Setenv("foo", "bar"); err != nil {
		t.Fatalf("cannot set env var: %s", err)
	}
	f := func(s, resultExpected string) {
		t.Helper()
		result, err := Replace([]byte(s))
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		if string(result) != resultExpected {
			t.Fatalf("unexpected result;\ngot\n%q\nwant\n%q", result, resultExpected)
		}
	}
	f("", "")
	f("foo", "foo")
	f("a %{foo}-x", "a bar-x")
}

func TestReplaceFailure(t *testing.T) {
	f := func(s string) {
		t.Helper()
		_, err := Replace([]byte(s))
		if err == nil {
			t.Fatalf("expecting non-nil error")
		}
	}
	f("foo %{bar} %{baz}")
}
