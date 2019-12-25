package metricsql

import (
	"testing"
)

func TestExpandWithExprsSuccess(t *testing.T) {
	f := func(q, qExpected string) {
		t.Helper()
		for i := 0; i < 3; i++ {
			qExpanded, err := ExpandWithExprs(q)
			if err != nil {
				t.Fatalf("unexpected error when expanding %q: %s", q, err)
			}
			if qExpanded != qExpected {
				t.Fatalf("unexpected expanded expression for %q;\ngot\n%q\nwant\n%q", q, qExpanded, qExpected)
			}
		}
	}

	f(`1`, `1`)
	f(`foobar`, `foobar`)
	f(`with (x = 1) x+x`, `2`)
	f(`with (f(x) = x*x) 3+f(2)+2`, `9`)
}

func TestExpandWithExprsError(t *testing.T) {
	f := func(q string) {
		t.Helper()
		for i := 0; i < 3; i++ {
			qExpanded, err := ExpandWithExprs(q)
			if err == nil {
				t.Fatalf("expecting non-nil error when expanding %q", q)
			}
			if qExpanded != "" {
				t.Fatalf("unexpected non-empty qExpanded=%q", qExpanded)
			}
		}
	}

	f(``)
	f(`  with (`)
}
