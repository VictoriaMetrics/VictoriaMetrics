package flagutil

import (
	"strings"
	"testing"
	"time"
)

func TestDurationSetFailure(t *testing.T) {
	f := func(value string) {
		t.Helper()
		var d RetentionDuration
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

	f("1mM")

	// RetentionDuration in minutes is confused with duration in months
	f("1m")
}

func TestDurationSetSuccess(t *testing.T) {
	f := func(value string, expectedMsecs int64, expectedValueString string) {
		t.Helper()
		var d RetentionDuration
		if err := d.Set(value); err != nil {
			t.Fatalf("unexpected error in d.Set(%q): %s", value, err)
		}
		if d.Milliseconds() != expectedMsecs {
			t.Fatalf("unexpected result; got %d; want %d", d.Milliseconds(), expectedMsecs)
		}
		valueString := d.String()
		if valueString != expectedValueString {
			t.Fatalf("unexpected valueString; got %q; want %q", valueString, expectedValueString)
		}
	}
	f("", 0, "")
	f("0", 0, "0M")
	f("1", msecsPer31Days, "1M")
	f("123.456", 123.456*msecsPer31Days, "123.456M")
	f("1h", 3600*1000, "1h")
	f("1.5d", 1.5*24*3600*1000, "1.5d")
	f("2.3W", 2.3*7*24*3600*1000, "2.3w")
	f("1w", 7*24*3600*1000, "1w")
	f("0.25y", 0.25*365*24*3600*1000, "0.25y")
	f("3M", 93*24*3600*1000, "3M")
	f("100y", 100*365*24*3600*1000, "100y")
}

func TestDurationDuration(t *testing.T) {
	f := func(value string, expected time.Duration) {
		t.Helper()
		var d RetentionDuration
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

func TestExtendedDurationSetFailure(t *testing.T) {
	f := func(value string) {
		t.Helper()
		var d ExtendedDuration
		if err := d.Set(value); err == nil {
			t.Fatalf("expecting non-nil error in d.Set(%q)", value)
		}
	}
	// Invalid format
	f("foobar")
	f("5foobar")
	f("ah")
	f("134xd")
	f("2.43sdfw")

	// Bare numbers are not allowed (except 0)
	f("1")
	f("5")
	f("123.456")

	// Negative duration
	f("-1h")
	f("-34d")

	// Invalid duration syntax
	f("1x")
	f("abc5d")
}

func TestExtendedDurationSetSuccess(t *testing.T) {
	f := func(value string, expectedMsecs int64) {
		t.Helper()
		var d ExtendedDuration
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
	// Empty and zero values
	f("", 0)
	f("0", 0)

	// Time units
	f("1s", 1000)
	f("30s", 30*1000)
	f("1h", 3600*1000)
	f("2h", 2*3600*1000)
	f("1.5h", 1.5*3600*1000)

	// Extended units
	f("1d", 24*3600*1000)
	f("1.5d", 1.5*24*3600*1000)
	f("7d", 7*24*3600*1000)
	f("1w", 7*24*3600*1000)
	f("2w", 2*7*24*3600*1000)
	f("1y", 365*24*3600*1000)
	f("0.25y", 0.25*365*24*3600*1000)

	// Case insensitive
	f("1D", 24*3600*1000)
	f("1W", 7*24*3600*1000)
	f("1Y", 365*24*3600*1000)

	// Minutes are allowed (no ambiguity like in RetentionDuration)
	f("1m", 60*1000)
	f("30m", 30*60*1000)
}

func TestExtendedDurationDuration(t *testing.T) {
	f := func(value string, expected time.Duration) {
		t.Helper()
		var d ExtendedDuration
		if err := d.Set(value); err != nil {
			t.Fatalf("unexpected error in d.Set(%q): %s", value, err)
		}
		if d.Duration() != expected {
			t.Fatalf("unexpected result; got %v; want %v", d.Duration().String(), expected.String())
		}
	}
	f("0", 0)
	f("1s", time.Second)
	f("1m", time.Minute)
	f("1h", time.Hour)
	f("1d", 24*time.Hour)
	f("1w", 7*24*time.Hour)
	f("1y", 365*24*time.Hour)
	f("1.5d", 1.5*24*time.Hour)
}

func TestExtendedDurationJSON(t *testing.T) {
	f := func(value string) {
		t.Helper()
		var d ExtendedDuration
		if err := d.Set(value); err != nil {
			t.Fatalf("unexpected error in d.Set(%q): %s", value, err)
		}

		// Test MarshalJSON
		data, err := d.MarshalJSON()
		if err != nil {
			t.Fatalf("unexpected error in MarshalJSON(): %s", err)
		}

		// Test UnmarshalJSON
		var d2 ExtendedDuration
		if err := d2.UnmarshalJSON(data); err != nil {
			t.Fatalf("unexpected error in UnmarshalJSON(): %s", err)
		}

		if d.Milliseconds() != d2.Milliseconds() {
			t.Fatalf("unexpected result after JSON roundtrip; got %d; want %d", d2.Milliseconds(), d.Milliseconds())
		}
		if d.String() != d2.String() {
			t.Fatalf("unexpected string after JSON roundtrip; got %q; want %q", d2.String(), d.String())
		}
	}
	f("0")
	f("1h")
	f("1d")
	f("1w")
	f("1y")
}
