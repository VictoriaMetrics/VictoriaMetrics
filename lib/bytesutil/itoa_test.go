package bytesutil

import (
	"testing"
)

func TestItoa(t *testing.T) {
	f := func(n int, resultExpected string) {
		t.Helper()
		for i := 0; i < 5; i++ {
			result := Itoa(n)
			if result != resultExpected {
				t.Fatalf("unexpected result for Itoa(%d); got %q; want %q", n, result, resultExpected)
			}
		}
	}
	f(0, "0")
	f(1, "1")
	f(-123, "-123")
	f(343432, "343432")
}
