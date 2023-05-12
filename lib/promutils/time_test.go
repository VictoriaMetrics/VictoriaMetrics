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
	f("now", now)
	f("1h5s", now-3605)

	// negative duration relative to the current time
	f("-5m", now-5*60)
	f("-123", now-123)
	f("-123.456", now-123.456)
	f("now-1h5m", now-(3600+5*60))

	// Year
	f("2023", 1.6725312e+09)
	f("2023Z", 1.6725312e+09)
	f("2023+02:00", 1.672524e+09)
	f("2023-02:00", 1.6725384e+09)

	// Year and month
	f("2023-05", 1.6828992e+09)
	f("2023-05Z", 1.6828992e+09)
	f("2023-05+02:00", 1.682892e+09)
	f("2023-05-02:00", 1.6829064e+09)

	// Year, month and day
	f("2023-05-20", 1.6845408e+09)
	f("2023-05-20Z", 1.6845408e+09)
	f("2023-05-20+02:30", 1.6845318e+09)
	f("2023-05-20-02:30", 1.6845498e+09)

	// Year, month, day and hour
	f("2023-05-20T04", 1.6845552e+09)
	f("2023-05-20T04Z", 1.6845552e+09)
	f("2023-05-20T04+02:30", 1.6845462e+09)
	f("2023-05-20T04-02:30", 1.6845642e+09)

	// Year, month, day, hour and minute
	f("2023-05-20T04:57", 1.68455862e+09)
	f("2023-05-20T04:57Z", 1.68455862e+09)
	f("2023-05-20T04:57+02:30", 1.68454962e+09)
	f("2023-05-20T04:57-02:30", 1.68456762e+09)

	// Year, month, day, hour, minute and second
	f("2023-05-20T04:57:43", 1.684558663e+09)
	f("2023-05-20T04:57:43Z", 1.684558663e+09)
	f("2023-05-20T04:57:43+02:30", 1.684549663e+09)
	f("2023-05-20T04:57:43-02:30", 1.684567663e+09)

	// milliseconds
	f("2023-05-20T04:57:43.123Z", 1.6845586631230001e+09)
	f("2023-05-20T04:57:43.123456789+02:30", 1.6845496631234567e+09)
	f("2023-05-20T04:57:43.123456789-02:30", 1.6845676631234567e+09)
}

func TestParseTimeFailure(t *testing.T) {
	f := func(s string) {
		t.Helper()
		ts, err := ParseTime(s)
		if ts != 0 {
			t.Fatalf("unexpected time parsed: %f; want 0", ts)
		}
		if err == nil {
			t.Fatalf("expecting non-nil error")
		}
	}

	f("")
	f("23-45:50")
	f("1223-fo:ba")
	f("1223-12:ba")
	f("23-45")
	f("-123foobar")
	f("2oo5")
	f("2oob-a5")
	f("2oob-ar-a5")
	f("2oob-ar-azTx5")
	f("2oob-ar-azTxx:y5")
	f("2oob-ar-azTxx:yy:z5")
}
