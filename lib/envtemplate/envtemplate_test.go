package envtemplate

import (
	"testing"
)

func TestReplace(t *testing.T) {
	f := func(s, resultExpected string) {
		t.Helper()
		result := Replace([]byte(s))
		if string(result) != resultExpected {
			t.Fatalf("unexpected result;\ngot\n%q\nwant\n%q", result, resultExpected)
		}
	}
	f("", "")
	f("foo", "foo")
	f("%{foo}", "%{foo}")
	f("foo %{bar} %{baz}", "foo %{bar} %{baz}")
}
