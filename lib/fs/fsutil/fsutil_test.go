package fsutil

import (
	"os"
	"testing"
)

func TestIsFsyncDisabledInternal(t *testing.T) {
	f := func(envVarValue string, resultExpected bool) {
		t.Helper()

		os.Setenv("DISABLE_FSYNC_FOR_TESTING", envVarValue)
		defer os.Unsetenv("DISABLE_FSYNC_FOR_TESTING")

		result := isFsyncDisabledInternal()
		if result != resultExpected {
			t.Errorf("unexpected value for DISABLE_FSYNC_FOR_TESTING=%q; got %v; want %v", envVarValue, result, resultExpected)
		}
	}

	// fsync must be unconditionally disabled in tests
	f("", true)

	f("TRUE", true)
	f("True", true)
	f("true", true)
	f("T", true)
	f("t", true)
	f("1", true)
	f("FALSE", false)
	f("False", false)
	f("false", false)
	f("F", false)
	f("f", false)
	f("0", false)

	f("unsupported", false)
	f("tRuE", false)
}
