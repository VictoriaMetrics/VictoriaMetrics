package timeutil

import (
	"testing"
	"time"
)

func TestAddJitterToDuration(t *testing.T) {
	f := func(d time.Duration) {
		t.Helper()
		result := AddJitterToDuration(d)
		if result < d {
			t.Fatalf("unexpected negative jitter")
		}
		variance := (float64(result) - float64(d)) / float64(d)
		if variance > 0.1 {
			t.Fatalf("too big variance=%.2f for result=%s, d=%s; mustn't exceed 0.1", variance, result, d)
		}
	}

	f(time.Nanosecond)
	f(time.Microsecond)
	f(time.Millisecond)
	f(time.Second)
	f(time.Hour)
	f(24 * time.Hour)
}
