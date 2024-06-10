package bytesutil

import (
	"strings"
	"sync/atomic"
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
		var lct atomic.Uint64
		lct.Store(lastCleanupTime)
		result := needCleanup(&lct, currentTime)
		if result != resultExpected {
			t.Fatalf("unexpected result for needCleanup(%d, %d); got %v; want %v", lastCleanupTime, currentTime, result, resultExpected)
		}
		if result {
			if n := lct.Load(); n != currentTime {
				t.Fatalf("unexpected value for lct; got %d; want currentTime=%d", n, currentTime)
			}
		} else {
			if n := lct.Load(); n != lastCleanupTime {
				t.Fatalf("unexpected value for lct; got %d; want lastCleanupTime=%d", n, lastCleanupTime)
			}
		}
	}
	f(0, 0, false)
	f(0, 61, false)
	f(0, 62, true)
	f(10, 100, true)
}
