package utils

import (
	"testing"
	"time"
)

func TestGetTime_Failure(t *testing.T) {
	f := func(s string) {
		t.Helper()

		_, err := ParseTime(s)
		if err == nil {
			t.Fatalf("expecting non-nil error")
		}
	}

	// empty string
	f("")

	// negative time
	f("-292273086-05-16T16:47:06Z")
}

func TestGetTime_Success(t *testing.T) {
	f := func(s string, resultExpected time.Time) {
		t.Helper()

		result, err := ParseTime(s)
		if err != nil {
			t.Fatalf("ParseTime() error: %s", err)
		}
		if result.Unix() != resultExpected.Unix() {
			t.Fatalf("unexpected result; got %s; want %s", result, resultExpected)
		}
	}

	// only year
	f("2019Z", time.Date(2019, 1, 1, 0, 0, 0, 0, time.UTC))

	// year and month
	f("2019-01Z", time.Date(2019, 1, 1, 0, 0, 0, 0, time.UTC))

	// year and not first month
	f("2019-02Z", time.Date(2019, 2, 1, 0, 0, 0, 0, time.UTC))

	// year, month and day
	f("2019-02-01Z", time.Date(2019, 2, 1, 0, 0, 0, 0, time.UTC))

	// year, month and not first day
	f("2019-02-10Z", time.Date(2019, 2, 10, 0, 0, 0, 0, time.UTC))

	// year, month, day and time
	f("2019-02-02T00Z", time.Date(2019, 2, 2, 0, 0, 0, 0, time.UTC))

	// year, month, day and one hour time
	f("2019-02-02T01Z", time.Date(2019, 2, 2, 1, 0, 0, 0, time.UTC))

	// time with zero minutes
	f("2019-02-02T01:00Z", time.Date(2019, 2, 2, 1, 0, 0, 0, time.UTC))

	// time with one minute
	f("2019-02-02T01:01Z", time.Date(2019, 2, 2, 1, 1, 0, 0, time.UTC))

	// time with zero seconds
	f("2019-02-02T01:01:00Z", time.Date(2019, 2, 2, 1, 1, 0, 0, time.UTC))

	// timezone with one second
	f("2019-02-02T01:01:01Z", time.Date(2019, 2, 2, 1, 1, 1, 0, time.UTC))

	// time with seconds and timezone
	f("2019-07-07T20:47:40+03:00", func() time.Time {
		l, _ := time.LoadLocation("Europe/Kiev")
		return time.Date(2019, 7, 7, 20, 47, 40, 0, l)
	}())

	// float timestamp representation",
	f("1562529662.324", time.Date(2019, 7, 7, 20, 01, 02, 324e6, time.UTC))

	// negative timestamp
	f("-9223372036.855", time.Date(1970, 01, 01, 00, 00, 00, 00, time.UTC))

	// big timestamp
	f("1223372036855", time.Date(2008, 10, 7, 9, 33, 56, 855e6, time.UTC))

	// duration time
	f("1h5m", time.Now().Add(-1*time.Hour).Add(-5*time.Minute))
}
