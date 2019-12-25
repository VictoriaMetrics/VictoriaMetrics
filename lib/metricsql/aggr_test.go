package metricsql

import (
	"testing"
)

func TestIsAggrFuncModifierSuccess(t *testing.T) {
	f := func(s string) {
		t.Helper()
		if !isAggrFuncModifier(s) {
			t.Fatalf("expecting valid funcModifier: %q", s)
		}
	}
	f("by")
	f("BY")
	f("without")
	f("Without")
}

func TestIsAggrFuncModifierError(t *testing.T) {
	f := func(s string) {
		t.Helper()
		if isAggrFuncModifier(s) {
			t.Fatalf("unexpected valid funcModifier: %q", s)
		}
	}
	f("byfix")
	f("on")
	f("ignoring")
}
