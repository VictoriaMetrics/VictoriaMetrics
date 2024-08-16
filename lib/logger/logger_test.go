package logger

import (
	"fmt"
	"testing"
)

func TestFormatLogMessage(t *testing.T) {
	f := func(format string, args []any, maxArgLen int, expectedResult string) {
		t.Helper()
		result := formatLogMessage(maxArgLen, format, args)
		if result != expectedResult {
			t.Fatalf("unexpected result; got\n%q\nwant\n%q", result, expectedResult)
		}
	}

	// Zero format args
	f("foobar", nil, 1, "foobar")

	// Format args not exceeding the maxArgLen
	f("foo: %d, %s, %s, %s", []any{123, "bar", []byte("baz"), fmt.Errorf("abc")}, 3, "foo: 123, bar, baz, abc")

	// Format args exceeding the maxArgLen
	f("foo: %s, %q, %s", []any{"abcde", fmt.Errorf("foo bar baz"), "xx"}, 4, `foo: a..e, "f..z", xx`)
}
