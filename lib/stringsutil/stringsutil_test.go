package stringsutil

import (
	"testing"
)

func TestLimitStringLen(t *testing.T) {
	f := func(s string, maxLen int, resultExpected string) {
		t.Helper()

		result := LimitStringLen(s, maxLen)
		if result != resultExpected {
			t.Fatalf("unexpected result; got %q; want %q", result, resultExpected)
		}
	}

	f("", 1, "")
	f("a", 10, "a")
	f("abc", 2, "abc")
	f("abcd", 3, "abcd")
	f("abcde", 3, "a..e")
	f("abcde", 4, "a..e")
	f("abcde", 5, "abcde")
}

func TestAppendLowercase(t *testing.T) {
	f := func(s, resultExpected string) {
		t.Helper()

		result := AppendLowercase(nil, s)
		if string(result) != resultExpected {
			t.Fatalf("unexpected result; got %q; want %q", result, resultExpected)
		}
	}

	f("", "")
	f("foo", "foo")
	f("FOO", "foo")
	f("foo БаР baz 123", "foo бар baz 123")
}
