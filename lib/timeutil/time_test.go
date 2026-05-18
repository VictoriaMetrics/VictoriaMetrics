package timeutil

import (
	"strings"
	"testing"
	"time"
)

func TestTryParseUnixTimestamp_Success(t *testing.T) {
	f := func(s string, timestampExpected int64) {
		t.Helper()

		timestamp, ok := TryParseUnixTimestamp(s)
		if !ok {
			t.Fatalf("cannot parse timestamp %q", s)
		}
		if timestamp != timestampExpected {
			t.Fatalf("unexpected timestamp returned from TryParseUnixTimestamp(%q); got %d; want %d", s, timestamp, timestampExpected)
		}
	}

	f("0", 0)

	// nanoseconds
	f("-1234567890123456789", -1234567890123456789)
	f("1234567890123456789", 1234567890123456789)
	f("1234567890123456.789", 1234567890123456789)

	// microseconds
	f("-1234567890123456", -1234567890123456000)
	f("1234567890123456", 1234567890123456000)
	f("1234567890123456.789", 1234567890123456789)

	// milliseconds
	f("-1234567890123", -1234567890123000000)
	f("1234567890123", 1234567890123000000)
	f("1234567890123.456", 1234567890123456000)

	// seconds
	f("-1234567890", -1234567890000000000)
	f("1234567890", 1234567890000000000)
	f("1234567890.123456789", 1234567890123456789)
	f("1234567890.12345678", 1234567890123456780)
	f("1234567890.1234567", 1234567890123456700)
	f("-1234567890.123456", -1234567890123456000)
	f("-1234567890.12345", -1234567890123450000)
	f("-1234567890.1234", -1234567890123400000)
	f("-1234567890.123", -1234567890123000000)
	f("-1234567890.12", -1234567890120000000)
	f("-1234567890.1", -1234567890100000000)

	// scientific notation
	f("1e9", 1000000000000000000)
	f("1.234e9", 1234000000000000000)
	f("-1.23456789e9", -1234567890000000000)
	f("1.234567890123456789e18", 1234567890123456789)
	f("-1.234567890123456789e18", -1234567890123456789)
	f("0.23456789e9", 234567890000000000)
	f("123.456789123e9", 123456789123000000)
	f("-1234.5678912e9", -1234567891200000000)
	f("123.678912e7", 1236789120000000000)
	f("1.23e7", 12300000000000000)
	f("1.23e6", 1230000000000000)
	f("1.23e5", 123000000000000)
	f("1.23e4", 12300000000000)
	f("1.23e3", 1230000000000)
	f("1.23e2", 123000000000)
	f("1.2e1", 12000000000)
	f("1123.456789123456789E15", 1123456789123456789)
}

func TestTryParseUnixTimestamp_Failure(t *testing.T) {
	f := func(s string) {
		t.Helper()

		_, ok := TryParseUnixTimestamp(s)
		if ok {
			t.Fatalf("expecting failure when parsing %q", s)
		}
	}

	// non-numeric timestamp
	f("")
	f("foobar")
	f("foo.bar")
	f("1.12345671x34")
	f("1.3e12345678x0123")
	f("1xs.12345671")
	f("1xs.12345671e5")
	f("-1xs.12345671e5")

	// missing fractional part
	f("1233344.")

	// too big timestamp
	f("12345678901234567.891")
	f("12345678901234567890")
	f("12345678901234.567891")
	f("12345678901234567890e3")
	f("12345678901234567890.234e3")
	f("-12345678901234567890")
	f("12345678901234567890.235424")
	f("12345678901234567890.235424e3")
	f("-12345678901234567890.235424")
	f("12345678901234567.89")
	f("12345678901234567.8")

	// too big fractional part
	f("0.1234567890123456789123")
	f("-0.1234567890123456789123")

	// too big decimal exponent
	f("1e19")
	f("1.3e123456789090123")

	// too small decimal exponent
	f("1.23e1")
	f("1.234e0")
	f("1E-1")
	f("1.3e-123456789090123")
}

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
	f("1562529662.6", now, 1562529662600*1e6)
	f("1562529662.67", now, 1562529662670*1e6)
	f("1562529662.678", now, 1562529662678*1e6)
	f("1562529662.678123", now, 1562529662678123*1e3)
	f("1562529662.678123456", now, 1562529662678123456)

	// unix timestamp in milliseconds
	f("1562529662678", now, 1562529662678*1e6)
	f("1562529662678.9", now, 1562529662678900*1e3)
	f("1562529662678.901", now, 1562529662678901*1e3)
	f("1562529662678.901324", now, 1562529662678901324)

	// unix timestamp in microseconds
	f("1562529662678901", now, 1562529662678901*1e3)
	f("1562529662678901.3", now, 1562529662678901300)
	f("1562529662678901.32", now, 1562529662678901320)
	f("1562529662678901.321", now, 1562529662678901321)

	// unix timestamp in nanoseconds
	f("1562529662678901234", now, 1562529662678901234)

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

func TestParseTimeAtLimits(t *testing.T) {
	f := func(s string, wantTime time.Time) {
		t.Helper()
		currentTimestamp := time.Now().UnixNano()
		got, err := ParseTimeAt(s, currentTimestamp)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if want := wantTime.UnixNano(); got != want {
			t.Fatalf("unexpected result; got %d; want %d", got, want)
		}
	}

	location := func(t *testing.T, location string) *time.Location {
		t.Helper()
		l, err := time.LoadLocation(location)
		if err != nil {
			t.Fatalf("could not load location %q: %v", location, err)
		}
		return l
	}
	east := location(t, "Etc/GMT-14") // UTC+14:00
	west := location(t, "Etc/GMT+12") // UTC-12:00

	// min year
	f("1970Z", time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC))
	f("1971+14:00", time.Date(1971, 1, 1, 0, 0, 0, 0, east))
	f("1970-12:00", time.Date(1970, 1, 1, 0, 0, 0, 0, west))

	// min month
	f("1970-01Z", time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC))
	f("1970-02+14:00", time.Date(1970, 2, 1, 0, 0, 0, 0, east))
	f("1970-01-12:00", time.Date(1970, 1, 1, 0, 0, 0, 0, west))

	// min day
	f("1970-01-01Z", time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC))
	f("1970-01-02+14:00", time.Date(1970, 1, 2, 0, 0, 0, 0, east))
	f("1970-01-01-12:00", time.Date(1970, 1, 1, 0, 0, 0, 0, west))

	// min hour
	f("1970-01-01T00Z", time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC))
	f("1970-01-01T14+14:00", time.Date(1970, 1, 1, 14, 0, 0, 0, east))
	f("1969-12-31T12-12:00", time.Date(1969, 12, 31, 12, 0, 0, 0, west))

	// min minute
	f("1970-01-01T00:00Z", time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC))
	f("1970-01-01T14:00+14:00", time.Date(1970, 1, 1, 14, 0, 0, 0, east))
	f("1969-12-31T12:00-12:00", time.Date(1969, 12, 31, 12, 0, 0, 0, west))

	// min second
	f("1970-01-01T00:00:00Z", time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC))
	f("1970-01-01T14:00:00+14:00", time.Date(1970, 1, 1, 14, 0, 0, 0, east))
	f("1969-12-31T12:00:00-12:00", time.Date(1969, 12, 31, 12, 0, 0, 0, west))

	// max year
	f("2262Z", time.Date(2262, 1, 1, 0, 0, 0, 0, time.UTC))
	f("2262+14:00", time.Date(2262, 1, 1, 0, 0, 0, 0, east))
	f("2262-12:00", time.Date(2262, 1, 1, 0, 0, 0, 0, west))

	// max month
	f("2262-04Z", time.Date(2262, 4, 1, 0, 0, 0, 0, time.UTC))
	f("2262-04+14:00", time.Date(2262, 4, 1, 0, 0, 0, 0, east))
	f("2262-04-12:00", time.Date(2262, 4, 1, 0, 0, 0, 0, west))

	// max day
	f("2262-04-11Z", time.Date(2262, 4, 11, 0, 0, 0, 0, time.UTC))
	f("2262-04-12+14:00", time.Date(2262, 4, 12, 0, 0, 0, 0, east))
	f("2262-04-11-12:00", time.Date(2262, 4, 11, 0, 0, 0, 0, west))

	// max hour
	f("2262-04-11T23Z", time.Date(2262, 4, 11, 23, 0, 0, 0, time.UTC))
	f("2262-04-12T13+14:00", time.Date(2262, 4, 12, 13, 0, 0, 0, east))
	f("2262-04-11T11-12:00", time.Date(2262, 4, 11, 11, 0, 0, 0, west))

	// max minute
	f("2262-04-11T23:47Z", time.Date(2262, 4, 11, 23, 47, 0, 0, time.UTC))
	f("2262-04-12T13:47+14:00", time.Date(2262, 4, 12, 13, 47, 0, 0, east))
	f("2262-04-11T11:47-12:00", time.Date(2262, 4, 11, 11, 47, 0, 0, west))

	// max second
	f("2262-04-11T23:47:16Z", time.Date(2262, 4, 11, 23, 47, 16, 0, time.UTC))
	f("2262-04-12T13:47:16+14:00", time.Date(2262, 4, 12, 13, 47, 16, 0, east))
	f("2262-04-11T11:47:16-12:00", time.Date(2262, 4, 11, 11, 47, 16, 0, west))
}

func TestParseTimeAtOutsideLimits(t *testing.T) {
	f := func(s string) {
		t.Helper()
		currentTimestamp := time.Now().UnixNano()
		got, err := ParseTimeAt(s, currentTimestamp)
		if err == nil {
			t.Fatalf("expected error but got %d", got)
		}
		if !strings.Contains(err.Error(), "must be in the range") {
			t.Fatalf("expected error: %v", err)
		}
	}

	// min year
	f("1969Z")
	f("1970+14:00")
	f("1969-12:00")

	// min month
	f("1969-12Z")
	f("1970-01+14:00")
	f("1969-12-12:00")

	// min day
	f("1969-12-31Z")
	f("1970-01-01+14:00")
	f("1969-12-31-12:00")

	// min hour
	f("1969-12-31T23Z")
	f("1970-01-01T13+14:00")
	f("1969-12-31T11-12:00")

	// min minute
	f("1969-12-31T23:59Z")
	f("1970-01-01T13:59+14:00")
	f("1969-12-31T11:59-12:00")

	// min second
	f("1969-12-31T23:59:59Z")
	f("1970-01-01T13:59:59+14:00")
	f("1969-12-31T11:59:59-12:00")

	// max year
	f("2263Z")
	f("2263+14:00")
	f("2263-12:00")

	// max month
	f("2262-05Z")
	f("2262-05+14:00")
	f("2262-05-12:00")

	// max day
	f("2262-04-12Z")
	f("2262-04-13+14:00")
	f("2262-04-12-12:00")

	// max hour
	f("2262-04-12T00Z")
	f("2262-04-12T14+14:00")
	f("2262-04-11T12-12:00")

	// max minute
	f("2262-04-11T23:48Z")
	f("2262-04-12T13:48+14:00")
	f("2262-04-11T11:48-12:00")

	// max second
	f("2262-04-11T23:47:17Z")
	f("2262-04-12T13:47:17+14:00")
	f("2262-04-11T11:47:17-12:00")
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
