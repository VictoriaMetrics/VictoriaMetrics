package bytesutil

import (
	"strings"
	"testing"
)

func TestFastStringMatcher(t *testing.T) {
	fsm := NewFastStringMatcher(func(s string) bool {
		return strings.HasPrefix(s, "foo")
	})
	f := func(s string, resultExpected bool) {
		t.Helper()
		for i := 0; i < 10; i++ {
			result := fsm.Match(s)
			if result != resultExpected {
				t.Fatalf("unexpected result for Match(%q) at iteration %d; got %v; want %v", s, i, result, resultExpected)
			}
		}
	}
	f("", false)
	f("foo", true)
	f("a_b-C", false)
	f("foobar", true)
}
