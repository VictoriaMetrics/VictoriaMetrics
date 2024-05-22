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
	testFunc := func(name string, retryAfterDuration, retryDuration time.Duration, n int, expectMinDuration time.Duration) {
		t.Run(name, func(t *testing.T) {
			for i := 0; i < n; i++ {
				retryDuration = calculateRetryDuration(retryAfterDuration, retryDuration, time.Minute)
			}

			expectMaxDuration := helper(expectMinDuration)
			expectMinDuration = expectMinDuration - (1000 * time.Millisecond) // Avoid edge case when calculating time.Until(now)

			if !(retryDuration >= expectMinDuration && retryDuration <= expectMaxDuration) {
				t.Fatalf(
					"incorrect retry duration, want (ms): [%d, %d], got (ms): %d",
					expectMinDuration.Milliseconds(), expectMaxDuration.Milliseconds(),
					retryDuration.Milliseconds(),
				)
			}
		})
	}

	// Call calculateRetryDuration for 1 time.
	{
		// default backoff policy test cases
		testFunc("default backoff policy", 0, time.Second, 1, 2*time.Second)
		testFunc("default backoff policy exceed max limit", 0, 10*time.Minute, 1, time.Minute)

		// retry after header test cases
		testFunc("retry after header > default backoff policy", 10*time.Second, 1*time.Second, 1, 10*time.Second)
		testFunc("retry after header < default backoff policy", 1*time.Second, 10*time.Second, 1, 20*time.Second)
		testFunc("retry after header invalid", 0, time.Second, 1, 2*time.Second)
	}

	// Call calculateRetryDuration for multiple times.
	{
		testFunc("default backoff policy 2 times", 0, time.Second, 2, 4*time.Second)
		testFunc("default backoff policy 3 times", 0, time.Second, 3, 8*time.Second)
		testFunc("default backoff policy N times exceed max limit", 0, time.Second, 10, time.Minute)

		testFunc("retry after header 10s 2 times", 10*time.Second, time.Second, 2, 20*time.Second)
		testFunc("retry after header 10s 3 times", 10*time.Second, time.Second, 3, 40*time.Second)
		testFunc("retry after header 10s 4 times exceed max limit", 10*time.Second, time.Second, 4, time.Minute)
		testFunc("retry after header 10s 10 times exceed max limit", 10*time.Second, time.Second, 10, time.Minute)

		testFunc("retry after header 120s 1 times", 120*time.Second, time.Second, 1, 120*time.Second)
		testFunc("retry after header 120s 2 times", 120*time.Second, time.Second, 2, 120*time.Second)
		testFunc("retry after header 120s 10 times", 120*time.Second, time.Second, 10, 120*time.Second)
	}
}

func TestParseRetryAfterHeader(t *testing.T) {
	testFunc := func(name string, retryAfterString string, expectResult time.Duration) {
		t.Run(name, func(t *testing.T) {

			result := parseRetryAfterHeader(retryAfterString)
			// expect `expectResult == result` when retryAfterString is in seconds or invalid
			// expect the difference between result and expectResult to be lower than 10%
			if !(expectResult == result || math.Abs(float64(expectResult-result))/float64(expectResult) < 0.10) {
				t.Fatalf(
					"incorrect retry after duration, want (ms): %d, got (ms): %d",
					expectResult.Milliseconds(), result.Milliseconds(),
				)
			}
		})
	}

	testFunc("retry after header in seconds", "10", 10*time.Second)
	testFunc("retry after header in date time", time.Now().Add(30*time.Second).UTC().Format(http.TimeFormat), 30*time.Second)
	testFunc("retry after header invalid", "invalid-retry-after", 0)
	testFunc("retry after header not in GMT", time.Now().Add(10*time.Second).Format("Mon, 02 Jan 2006 15:04:05 FAKETZ"), 0)
}

// helper calculate the max possible time duration calculated by timeutil.AddJitterToDuration.
func helper(d time.Duration) time.Duration {
	dv := d / 10
	if dv > 10*time.Second {
		dv = 10 * time.Second
	}

	return d + dv
}
