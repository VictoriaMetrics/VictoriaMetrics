package query

import (
	"testing"
)

func TestCheckTraceIDList(t *testing.T) {
	f := func(traceID string, valid bool) {
		t.Helper()

		result := checkTraceIDList([]string{traceID})
		if valid != (len(result) == 1) {
			t.Fatalf("check trace id unexpected result, trace_id: %s, valid: %t", traceID, len(result) == 1)
		}
	}
	f("12345678", true)
	f("abcd1234567", true)
	f("asdf-asdf-1234-asdf", true)
	f("abcd1234:4321bcda:4321bacd", true)
	f("abcd.abcd.1234.4321", true)
	f("abcd bcad", false)
	f("abcd\"", false)
}
