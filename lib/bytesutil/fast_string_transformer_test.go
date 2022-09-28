package bytesutil

import (
	"strings"
	"testing"
)

func TestFastStringTransformer(t *testing.T) {
	fst := NewFastStringTransformer(strings.ToUpper)
	f := func(s, resultExpected string) {
		t.Helper()
		for i := 0; i < 10; i++ {
			result := fst.Transform(s)
			if result != resultExpected {
				t.Fatalf("unexpected result for Transform(%q) at iteration %d; got %q; want %q", s, i, result, resultExpected)
			}
		}
	}
	f("", "")
	f("foo", "FOO")
	f("a_b-C", "A_B-C")
}
