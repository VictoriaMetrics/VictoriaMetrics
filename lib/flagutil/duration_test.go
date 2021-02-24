package flagutil

import (
	"strings"
	"testing"
)

func TestDurationSetFailure(t *testing.T) {
	f := func(value string) {
		t.Helper()
		var d Duration
		if err := d.Set(value); err == nil {
			t.Fatalf("expecting non-nil error in d.Set(%q)", value)
		}
	}
	f("")
	f("foobar")
	f("5foobar")
	f("ah")
	f("134xd")
	f("2.43sdfw")

	// Too big value in months
	f("12345")

	// Too big duration
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
		if d.Msecs != expectedMsecs {
			t.Fatalf("unexpected result; got %d; want %d", d.Msecs, expectedMsecs)
		}
		valueString := d.String()
		valueExpected := strings.ToLower(value)
		if valueString != valueExpected {
			t.Fatalf("unexpected valueString; got %q; want %q", valueString, valueExpected)
		}
	}
	f("0", 0)
	f("1", msecsPerMonth)
	f("123.456", 123.456*msecsPerMonth)
	f("1h", 3600*1000)
	f("1.5d", 1.5*24*3600*1000)
	f("2.3W", 2.3*7*24*3600*1000)
	f("0.25y", 0.25*365*24*3600*1000)
}
