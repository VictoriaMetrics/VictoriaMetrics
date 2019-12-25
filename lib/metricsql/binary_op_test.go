package metricsql

import (
	"testing"
)

func TestIsBinaryOpSuccess(t *testing.T) {
	f := func(s string) {
		t.Helper()
		if !isBinaryOp(s) {
			t.Fatalf("expecting valid binaryOp: %q", s)
		}
	}
	f("and")
	f("AND")
	f("unless")
	f("unleSS")
	f("==")
	f("!=")
	f(">=")
	f("<=")
	f("or")
	f("Or")
	f("+")
	f("-")
	f("*")
	f("/")
	f("%")
	f("^")
	f(">")
	f("<")
}

func TestIsBinaryOpError(t *testing.T) {
	f := func(s string) {
		t.Helper()
		if isBinaryOp(s) {
			t.Fatalf("unexpected valid binaryOp: %q", s)
		}
	}
	f("foobar")
	f("=~")
	f("!~")
	f("=")
	f("<==")
	f("234")
}

func TestIsBinaryOpGroupModifierSuccess(t *testing.T) {
	f := func(s string) {
		t.Helper()
		if !isBinaryOpGroupModifier(s) {
			t.Fatalf("expecting valid binaryOpGroupModifier: %q", s)
		}
	}
	f("on")
	f("ON")
	f("oN")
	f("ignoring")
	f("IGnoring")
}

func TestIsBinaryOpGroupModifierError(t *testing.T) {
	f := func(s string) {
		t.Helper()
		if isBinaryOpGroupModifier(s) {
			t.Fatalf("unexpected valid binaryOpGroupModifier: %q", s)
		}
	}
	f("off")
	f("by")
	f("without")
	f("123")
}

func TestIsBinaryOpJoinModifierSuccess(t *testing.T) {
	f := func(s string) {
		t.Helper()
		if !isBinaryOpJoinModifier(s) {
			t.Fatalf("expecting valid binaryOpJoinModifier: %q", s)
		}
	}
	f("group_left")
	f("group_right")
	f("group_LEft")
	f("GRoup_RighT")
}

func TestIsBinaryOpJoinModifierError(t *testing.T) {
	f := func(s string) {
		t.Helper()
		if isBinaryOpJoinModifier(s) {
			t.Fatalf("unexpected valid binaryOpJoinModifier: %q", s)
		}
	}
	f("on")
	f("by")
	f("without")
	f("123")
}

func TestIsBinaryOpBoolModifierSuccess(t *testing.T) {
	f := func(s string) {
		t.Helper()
		if !isBinaryOpBoolModifier(s) {
			t.Fatalf("expecting valid binaryOpBoolModifier: %q", s)
		}
	}
	f("bool")
	f("bOOL")
	f("BOOL")
}

func TestIsBinaryOpBoolModifierError(t *testing.T) {
	f := func(s string) {
		t.Helper()
		if isBinaryOpBoolModifier(s) {
			t.Fatalf("unexpected valid binaryOpBoolModifier: %q", s)
		}
	}
	f("on")
	f("by")
	f("without")
	f("123")
}
