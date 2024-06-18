package flagutil

import (
	"strings"
	"testing"
	"time"
)

func TestDurationSetFailure(t *testing.T) {
	f := func(value string) {
		t.Helper()
		var d Duration
		if err := d.Set(value); err == nil {
			t.Fatalf("expecting non-nil error in d.Set(%q)", value)
		}
	}
	f("foobar")
	f("5foobar")
	f("ah")
	f("134xd")
	f("2.43sdfw")

	// Too big value in months
	f("12345")

	// Too big duration
	f("999y")
	f("100000000000y")

	// Negative duration
	f("-1")
	f("-34h")

	// Duration in minutes is confused with duration in months
	f("1m")
}

func TestDurationSetSuccess(t *testing.T) {
	f := func(value string, expectedMsecs int64) {
		t.Helper()
		var d Duration
		if err := d.Set(value); err != nil {
			t.Fatalf("unexpected error in d.Set(%q): %s", value, err)
		}
		if d.Milliseconds() != expectedMsecs {
			t.Fatalf("unexpected result; got %d; want %d", d.Milliseconds(), expectedMsecs)
		}
		valueString := d.String()
		valueExpected := strings.ToLower(value)
		if valueString != valueExpected {
			t.Fatalf("unexpected valueString; got %q; want %q", valueString, valueExpected)
		}
	}
	f("", 0)
	f("0", 0)
	f("1", msecsPer31Days)
	f("123.456", 123.456*msecsPer31Days)
	f("1h", 3600*1000)
	f("1.5d", 1.5*24*3600*1000)
	f("2.3W", 2.3*7*24*3600*1000)
	f("1w", 7*24*3600*1000)
	f("0.25y", 0.25*365*24*3600*1000)
	f("100y", 100*365*24*3600*1000)
}

func TestDurationDuration(t *testing.T) {
	f := func(value string, expected time.Duration) {
		t.Helper()
		var d Duration
		if err := d.Set(value); err != nil {
			t.Fatalf("unexpected error in d.Set(%q): %s", value, err)
		}
		if d.Duration() != expected {
			t.Fatalf("unexpected result; got %v; want %v", d.Duration().String(), expected.String())
		}
	}
	f("0", 0)
	f("1", 31*24*time.Hour)
	f("1h", time.Hour)
	f("1.5d", 1.5*24*time.Hour)
	f("1w", 7*24*time.Hour)
	f("0.25y", 0.25*365*24*time.Hour)
}
