package envutil

import (
	"os"
	"testing"
)

func TestGetenvBool(t *testing.T) {
	f := func(value string, want bool) {
		t.Helper()

		key := "VM_LIB_ENVUTIL_TEST_GETENV_BOOL"
		os.Setenv(key, value)
		defer os.Unsetenv(key)

		if got := GetenvBool(key); got != want {
			t.Errorf("GetenvBool(%s=%s) unexpected return value: got %t, want %t", key, value, got, want)
		}
	}

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

	f("", false)
	f("unsupported", false)
	f("tRuE", false)
}
