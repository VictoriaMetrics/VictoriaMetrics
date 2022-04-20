package flagutil

import (
	"testing"
	"time"
)

func TestPromDuration(t *testing.T) {
	f := func(value string, duration time.Duration) {
		t.Helper()
		var d PromDuration
		if err := d.Set(value); err != nil {
			t.Fatalf("unexpected error in d.Set(%q): %s", value, err)
		}
		if d.Duration() != duration {
			t.Fatalf("unexpected result; got %s; want %s", d.Duration(), duration)
		}
	}
	f("0", 0)
	f("1w", 7*24*time.Hour)
	f("1h", time.Hour)
	f("-1h", -time.Hour)
	f("1.5d", 36*time.Hour)
	f("1.5d", 36*time.Hour)
}
