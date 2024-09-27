package promutils

import (
	"testing"
	"time"
)

func TestParseTimeAtSuccess(t *testing.T) {
	f := func(s string, currentTime, resultExpected int64) {
		t.Helper()
		result, err := ParseTimeAt(s, currentTime)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		if result != resultExpected {
			t.Fatalf("unexpected result; got %v; want %v", result, resultExpected)
		}
	}

	now := time.Now().UnixNano()

	// unix timestamp in seconds
	f("1562529662", now, 1562529662*1e9)
	f("1562529662.678", now, 1562529662678*1e6)

	// unix timestamp in milliseconds
	f("1562529662678", now, 1562529662678*1e6)

	// duration relative to the current time
	f("now", now, now)
	f("1h5s", now, now-3605*1e9)

	// negative duration relative to the current time
	f("-5m", now, now-5*60*1e9)
	f("-123", now, now-123*1e9)
	f("-123.456", now, now-123456*1e6)
	f("now-1h5m", now, now-(3600+5*60)*1e9)

	// Year
	f("2023Z", now, 1.6725312e+09*1e9)
	f("2023+02:00", now, 1.672524e+09*1e9)
	f("2023-02:00", now, 1.6725384e+09*1e9)

	// Year and month
	f("2023-05Z", now, 1.6828992e+09*1e9)
	f("2023-05+02:00", now, 1.682892e+09*1e9)
	f("2023-05-02:00", now, 1.6829064e+09*1e9)

	// Year, month and day
	f("2023-05-20Z", now, 1.6845408e+09*1e9)
	f("2023-05-20+02:30", now, 1.6845318e+09*1e9)
	f("2023-05-20-02:30", now, 1.6845498e+09*1e9)

	// Year, month, day and hour
	f("2023-05-20T04Z", now, 1.6845552e+09*1e9)
	f("2023-05-20T04+02:30", now, 1.6845462e+09*1e9)
	f("2023-05-20T04-02:30", now, 1.6845642e+09*1e9)

	// Year, month, day, hour and minute
	f("2023-05-20T04:57Z", now, 1.68455862e+09*1e9)
	f("2023-05-20T04:57+02:30", now, 1.68454962e+09*1e9)
	f("2023-05-20T04:57-02:30", now, 1.68456762e+09*1e9)

	// Year, month, day, hour, minute and second
	f("2023-05-20T04:57:43Z", now, 1.684558663e+09*1e9)
	f("2023-05-20T04:57:43+02:30", now, 1.684549663e+09*1e9)
	f("2023-05-20T04:57:43-02:30", now, 1.684567663e+09*1e9)

	// milliseconds
	f("2023-05-20T04:57:43.123Z", now, 1684558663123000000)
	f("2023-05-20T04:57:43.123456789+02:30", now, 1684549663123456789)
	f("2023-05-20T04:57:43.123456789-02:30", now, 1684567663123456789)
}

func TestParseTimeMsecFailure(t *testing.T) {
	f := func(s string) {
		t.Helper()
		msec, err := ParseTimeMsec(s)
		if msec != 0 {
			t.Fatalf("unexpected time parsed: %d; want 0", msec)
		}
		if err == nil {
			t.Fatalf("expecting non-nil error")
		}
	}

	f("")
	f("2263")
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
