package timeutil

import (
	"testing"
	"time"
)

func TestParseDuration(t *testing.T) {
	f := func(s string, resultExpected time.Duration) {
		t.Helper()
		result, err := ParseDuration(s)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		if result != resultExpected {
			t.Fatalf("unexpected result; got %v; want %v", result, resultExpected)
		}
	}

	f("0", 0)
	f("1s", time.Second)
	f("1m", time.Minute)
	f("1h", time.Hour)
	f("1d", time.Hour*24)
	f("1w", time.Hour*24*7)
	f("1m30s", time.Minute+time.Second*30)
	f("-1m30s", -(time.Minute + time.Second*30))
	f("1d-4h", time.Hour*20)
}
