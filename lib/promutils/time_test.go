package promutils

import (
	"math"
	"testing"
	"time"
)

func TestParseTimeSuccess(t *testing.T) {
	f := func(s string, resultExpected float64) {
		t.Helper()
		result, err := ParseTime(s)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		if math.Abs(result-resultExpected) > 10 {
			t.Fatalf("unexpected result; got %v; want %v", result, resultExpected)
		}
	}

	now := float64(time.Now().UnixNano()) / 1e9
	// duration relative to the current time
	f("1h5s", now-3605)

	// negative duration relative to the current time
	f("-5m", now-5*60)

	// Year
	f("2023", 1.6725312e+09)

	// Year and month
	f("2023-05", 1.6828992e+09)

	// Year, month and day
	f("2023-05-20", 1.6845408e+09)

	// Year, month, day and hour
	f("2023-05-20T04", 1.6845552e+09)

	// Year, month, day, hour and minute
	f("2023-05-20T04:57", 1.68455862e+09)

	// Year, month, day, hour, minute and second
	f("2023-05-20T04:57:43", 1.684558663e+09)

	// RFC3339
	f("2023-05-20T04:57:43Z", 1.684558663e+09)
	f("2023-05-20T04:57:43+02:30", 1.684549663e+09)
	f("2023-05-20T04:57:43-02:30", 1.684567663e+09)
	f("2023-05-20T04:57:43.123Z", 1.6845586631230001e+09)
	f("2023-05-20T04:57:43.123456789Z", 1.6845586631230001e+09)
}
