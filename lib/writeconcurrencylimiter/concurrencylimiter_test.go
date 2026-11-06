package writeconcurrencylimiter

import (
	"testing"
	"time"
)

func TestRetryAfterSeconds(t *testing.T) {
	f := func(d time.Duration, base int) {
		t.Helper()
		for range 50 {
			got := retryAfterSeconds(d)
			if got < base || got > 2*base {
				t.Fatalf("retryAfterSeconds(%s) = %d; want in [%d, %d]", d, got, base, 2*base)
			}
		}
	}

	f(0, 0)
	f(500*time.Millisecond, 1)
	f(15*time.Second, 15)
	f(time.Minute, 60)
}
