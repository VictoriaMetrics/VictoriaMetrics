package remotewrite

import (
	"net/http"
	"testing"
	"time"
)

func TestCalculateRetryDuration(t *testing.T) {
	// testFunc evaluate if the result of calculateRetryDuration is
	// 1. >= expectMinDuration
	// 2. <= expectMinDuration + 10% (see timeutil.AddJitterToDuration)
	testFunc := func(name string, retryAfterString string, retryDuration, expectMinDuration time.Duration) {
		t.Run(name, func(t *testing.T) {
			result := calculateRetryDuration(retryAfterString, retryDuration, time.Minute)

			expectMaxDuration := helper(expectMinDuration)
			expectMinDuration = expectMinDuration - (1000 * time.Millisecond) // Avoid edge case when calculating time.Until(now)

			if !(result >= expectMinDuration && result <= expectMaxDuration) {
				t.Fatalf(
					"incorrect retry duration, want (ms): [%d, %d], got (ms): %d",
					expectMinDuration.Milliseconds(), expectMaxDuration.Milliseconds(),
					result.Milliseconds(),
				)
			}
		})
	}

	// default timezone

	// default backoff policy test cases
	testFunc("default backoff policy", "", time.Second, 2*time.Second)
	testFunc("default backoff policy exceed max limit", "", 10*time.Minute, time.Minute)

	// retry after header test cases
	testFunc("retry after header in seconds", "10", time.Second, 10*time.Second)
	testFunc("retry after header in date time", time.Now().Add(10*time.Second).UTC().Format(http.TimeFormat), time.Second, 10*time.Second)
	testFunc("retry after header < default backoff policy", "1", 10*time.Second, 20*time.Second)
	testFunc("retry after header invalid", "in-correct-header", time.Second, 2*time.Second)
	testFunc("retry after header not in GMT", time.Now().Add(10*time.Second).Format("Mon, 02 Jan 2006 15:04:05 FAKETZ"), time.Second, 2*time.Second)

}

// helper calculate the max possible time duration calculated by timeutil.AddJitterToDuration.
func helper(d time.Duration) time.Duration {
	dv := d / 10
	if dv > 10*time.Second {
		dv = 10 * time.Second
	}

	return d + dv
}
