package graphite

import (
	"testing"
)

func TestNaturalLess(t *testing.T) {
	f := func(a, b string, okExpected bool) {
		t.Helper()
		ok := naturalLess(a, b)
		if ok != okExpected {
			t.Fatalf("unexpected result for naturalLess(%q, %q); got %v; want %v", a, b, ok, okExpected)
		}
	}
	f("", "", false)
	f("a", "b", true)
	f("", "foo", true)
	f("foo", "", false)
	f("foo", "foo", false)
	f("b", "a", false)
	f("1", "2", true)
	f("10", "2", false)
	f("foo100", "foo12", false)
	f("foo12", "foo100", true)
	f("10foo2", "10foo10", true)
	f("10foo10", "10foo2", false)
	f("foo1bar10", "foo1bar2aa", false)
	f("foo1bar2aa", "foo1bar10aa", true)
}
