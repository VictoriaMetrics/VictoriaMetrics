package flagutil

import (
	"flag"
	"testing"
)

var (
	fooFlagDynamicDefault       = flag.Int("fooFlagDynamicDefault", 42, "test")
	fooFlagDynamicDefaultCapped = flag.Int("fooFlagDynamicDefaultCapped", 256, "test")
)

func TestSetDynamicDefaultSuccess(t *testing.T) {
	f := func(flagName, s string, value *int, expectedValue int) {
		t.Helper()
		SetDynamicDefault(flagName, s)
		result := flag.Lookup(flagName).DefValue
		if result != s {
			t.Fatalf("unexpected DefValue; got %q; want %q", result, s)
		}
		// the flag value itself must be left intact, since only the -help output changes
		if *value != expectedValue {
			t.Fatalf("unexpected value for -%s; got %d; want %d", flagName, *value, expectedValue)
		}
	}

	f("fooFlagDynamicDefault", "2 * availableCPUs", fooFlagDynamicDefault, 42)
	f("fooFlagDynamicDefaultCapped", "min(16 * availableCPUs, 256)", fooFlagDynamicDefaultCapped, 256)
}

func TestSetDynamicDefaultFailure(t *testing.T) {
	f := func(flagName string) {
		t.Helper()
		defer func() {
			if r := recover(); r == nil {
				t.Fatalf("expecting panic for unknown flag %q", flagName)
			}
		}()
		SetDynamicDefault(flagName, "2 * availableCPUs")
	}

	f("fooFlagNonExisting")
	f("")
}
