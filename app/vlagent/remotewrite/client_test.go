package remotewrite

import (
	"math"
	"net/http"
	"testing"
	"time"
)

func TestCalculateRetryDuration(t *testing.T) {
	// `testFunc` call `calculateRetryDuration` for `n` times
	// and evaluate if the result of `calculateRetryDuration` is
	// 1. >= expectMinDuration
	// 2. <= expectMinDuration + 10% (see timeutil.AddJitterToDuration)
	f := func(retryAfterDuration, retryDuration time.Duration, n int, expectMinDuration time.Duration) {
		t.Helper()

		for i := 0; i < n; i++ {
			retryDuration = getRetryDuration(retryAfterDuration, retryDuration, time.Minute)
		}

		expectMaxDuration := helper(expectMinDuration)
		expectMinDuration = expectMinDuration - (1000 * time.Millisecond) // Avoid edge case when calculating time.Until(now)

		if retryDuration < expectMinDuration || retryDuration > expectMaxDuration {
			t.Fatalf(
				"incorrect retry duration, want (ms): [%d, %d], got (ms): %d",
				expectMinDuration.Milliseconds(), expectMaxDuration.Milliseconds(),
				retryDuration.Milliseconds(),
			)
		}
	}

	// Call calculateRetryDuration for 1 time.
	{
		// default backoff policy
		f(0, time.Second, 1, 2*time.Second)
		// default backoff policy exceed max limit"
		f(0, 10*time.Minute, 1, time.Minute)

		// retry after > default backoff policy
		f(10*time.Second, 1*time.Second, 1, 10*time.Second)
		// retry after < default backoff policy
		f(1*time.Second, 10*time.Second, 1, 1*time.Second)
		// retry after invalid and < default backoff policy
		f(0, time.Second, 1, 2*time.Second)

	}

	// Call calculateRetryDuration for multiple times.
	{
		// default backoff policy 2 times
		f(0, time.Second, 2, 4*time.Second)
		// default backoff policy 3 times
		f(0, time.Second, 3, 8*time.Second)
		// default backoff policy N times exceed max limit
		f(0, time.Second, 10, time.Minute)

		// retry after 120s 1 times
		f(120*time.Second, time.Second, 1, 120*time.Second)
		// retry after 120s 2 times
		f(120*time.Second, time.Second, 2, 120*time.Second)
	}
}

func TestParseRetryAfterHeader(t *testing.T) {
	f := func(retryAfterString string, expectResult time.Duration) {
		t.Helper()

		result := parseRetryAfterHeader(retryAfterString)
		// expect `expectResult == result` when retryAfterString is in seconds or invalid
		// expect the difference between result and expectResult to be lower than 10%
		if !(expectResult == result || math.Abs(float64(expectResult-result))/float64(expectResult) < 0.10) {
			t.Fatalf(
				"incorrect retry after duration, want (ms): %d, got (ms): %d",
				expectResult.Milliseconds(), result.Milliseconds(),
			)
		}
	}

	// retry after header in seconds
	f("10", 10*time.Second)
	// retry after header in date time
	f(time.Now().Add(30*time.Second).UTC().Format(http.TimeFormat), 30*time.Second)
	// retry after header invalid
	f("invalid-retry-after", 0)
	// retry after header not in GMT
	f(time.Now().Add(10*time.Second).Format("Mon, 02 Jan 2006 15:04:05 FAKETZ"), 0)
}

// helper calculate the max possible time duration calculated by timeutil.AddJitterToDuration.
func helper(d time.Duration) time.Duration {
	dv := d / 10
	if dv > 10*time.Second {
		dv = 10 * time.Second
	}

	return d + dv
}
