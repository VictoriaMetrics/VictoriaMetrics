package flagutil

import (
	"flag"
	"strings"
	"testing"
)

// The flags are registered at package level, since flag registration panics when it repeats.
var (
	fooFlagIntDynamicDefault      = NewIntWithDynamicDefault("fooFlagIntDynamicDefault", 42, "2 * availableCPUs", "test")
	fooFlagArrayIntDynamicDefault = NewArrayIntWithDynamicDefault("fooFlagArrayIntDynamicDefault", 42, "2 * availableCPUs", "test")
	fooFlagArrayIntPlainDefault   = NewArrayInt("fooFlagArrayIntPlainDefault", 42, "test")
)

func TestNewIntWithDynamicDefaultSuccess(t *testing.T) {
	// -help must show the value together with the hint.
	f := flag.Lookup("fooFlagIntDynamicDefault")
	if f.DefValue != "42 = 2 * availableCPUs" {
		t.Fatalf("unexpected DefValue; got %q; want %q", f.DefValue, "42 = 2 * availableCPUs")
	}

	// the flag value must stay the calculated one.
	if *fooFlagIntDynamicDefault != 42 {
		t.Fatalf("unexpected flag value; got %d; want %d", *fooFlagIntDynamicDefault, 42)
	}
}

func TestNewArrayIntWithDynamicDefaultSuccess(t *testing.T) {
	// array flags keep the default in the description, so the hint must go there.
	f := flag.Lookup("fooFlagArrayIntDynamicDefault")
	if !strings.Contains(f.Usage, "(default 42 = 2 * availableCPUs)") {
		t.Fatalf("missing the hint in the flag description; got %q", f.Usage)
	}

	// the default value must stay the calculated one.
	if n := fooFlagArrayIntDynamicDefault.GetOptionalArg(0); n != 42 {
		t.Fatalf("unexpected default value; got %d; want %d", n, 42)
	}
}

func TestNewArrayIntKeepsPlainDefault(t *testing.T) {
	// NewArrayInt must keep showing a plain number, since it shares the body with the dynamic one.
	f := flag.Lookup("fooFlagArrayIntPlainDefault")
	if !strings.Contains(f.Usage, "(default 42)") {
		t.Fatalf("unexpected flag description; got %q", f.Usage)
	}
	if n := fooFlagArrayIntPlainDefault.GetOptionalArg(0); n != 42 {
		t.Fatalf("unexpected default value; got %d; want %d", n, 42)
	}
}
