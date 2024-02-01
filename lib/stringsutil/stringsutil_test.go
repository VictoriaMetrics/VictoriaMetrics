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
