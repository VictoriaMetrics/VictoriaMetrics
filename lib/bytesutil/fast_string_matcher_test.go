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

func TestNeedCleanup(t *testing.T) {
	f := func(lastCleanupTime, currentTime uint64, resultExpected bool) {
		t.Helper()
		lct := lastCleanupTime
		result := needCleanup(&lct, currentTime)
		if result != resultExpected {
			t.Fatalf("unexpected result for needCleanup(%d, %d); got %v; want %v", lastCleanupTime, currentTime, result, resultExpected)
		}
		if result {
			if lct != currentTime {
				t.Fatalf("unexpected value for lct; got %d; want currentTime=%d", lct, currentTime)
			}
		} else {
			if lct != lastCleanupTime {
				t.Fatalf("unexpected value for lct; got %d; want lastCleanupTime=%d", lct, lastCleanupTime)
			}
		}
	}
	f(0, 0, false)
	f(0, 61, false)
	f(0, 62, true)
	f(10, 100, true)
}
