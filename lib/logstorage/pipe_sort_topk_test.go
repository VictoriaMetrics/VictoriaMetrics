package logstorage

import (
	"testing"
)

func TestLessString(t *testing.T) {
	f := func(a, b string, resultExpected bool) {
		t.Helper()

		result := lessString(a, b)
		if result != resultExpected {
			t.Fatalf("unexpected result for lessString(%q, %q); got %v; want %v", a, b, result, resultExpected)
		}
	}

	f("", "", false)
	f("a", "", false)
	f("", "a", true)
	f("foo", "bar", false)
	f("bar", "foo", true)
	f("foo", "foo", false)
	f("foo1", "foo", false)
	f("foo", "foo1", true)

	// integers
	f("123", "9", false)
	f("9", "123", true)
	f("-123", "9", true)
	f("9", "-123", false)

	// floating point numbers
	f("1e3", "5", false)
	f("5", "1e3", true)

	// timestamps
	f("2025-01-15T10:20:30.1", "2025-01-15T10:20:30.09", false)
	f("2025-01-15T10:20:30.09", "2025-01-15T10:20:30.1", true)

	// versions
	f("v1.23.4", "v1.23.10", true)
	f("v1.23.10", "v1.23.4", false)

	// durations
	f("1h", "5s", false)
	f("5s", "1h", true)

	// bytes
	f("1MB", "5KB", false)
	f("5KB", "1MB", true)

	f("1.5M", "5.1K", false)
	f("5.1K", "1.5M", true)
	f("1.5M", "1.5M", false)
}
