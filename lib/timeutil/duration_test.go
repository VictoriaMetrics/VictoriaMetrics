package timeutil

import (
	"fmt"
	"testing"
	"time"
)

func TestParseDuration(t *testing.T) {
	f := func(s string, resultExpected time.Duration) {
		t.Helper()
		result, err := ParseDuration(s)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		if result != resultExpected {
			t.Fatalf("unexpected result; got %v; want %v", result, resultExpected)
		}
	}

	f("0", 0)
	f("1s", time.Second)
	f("1m", time.Minute)
	f("1h", time.Hour)
	f("1d", time.Hour*24)
	f("1w", time.Hour*24*7)
	f("1m30s", time.Minute+time.Second*30)
	f("-1m30s", -(time.Minute + time.Second*30))
	f("1d-4h", time.Hour*20)
}

func TestParseDurationLimits(t *testing.T) {
	f := func(s string, want time.Duration) {
		t.Helper()
		got, err := ParseDuration(s)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != want {
			t.Fatalf("unexpected result: got %v, want %v", got, want)
		}
	}

	var s string
	var want time.Duration

	s = fmt.Sprintf("%dms", int64(minValidMilli))
	f(s, minDuration)
	s = fmt.Sprintf("%dms", int64(maxValidMilli))
	f(s, maxDuration)

	s = fmt.Sprintf("%ds", int64(minValidSecond))
	want = minValidSecond * time.Second
	f(s, want)
	s = fmt.Sprintf("%ds", int64(maxValidSecond))
	want = maxValidSecond * time.Second
	f(s, want)

	// When no unit is specified, seconds are assumed.
	s = fmt.Sprintf("%d", int64(minValidSecond))
	want = minValidSecond * time.Second
	f(s, want)
	s = fmt.Sprintf("%d", int64(maxValidSecond))
	want = maxValidSecond * time.Second
	f(s, want)

	minValidMinute := int64(minValidSecond) / 60
	maxValidMinute := int64(maxValidSecond) / 60
	s = fmt.Sprintf("%dm", minValidMinute)
	want = time.Duration(minValidMinute) * time.Minute
	f(s, want)
	s = fmt.Sprintf("%dm", maxValidMinute)
	want = time.Duration(maxValidMinute) * time.Minute
	f(s, want)

	minValidHour := minValidMinute / 60
	maxValidHour := maxValidMinute / 60
	s = fmt.Sprintf("%dh", minValidHour)
	want = time.Duration(minValidHour) * time.Hour
	f(s, want)
	s = fmt.Sprintf("%dh", maxValidHour)
	want = time.Duration(maxValidHour) * time.Hour
	f(s, want)

	minValidDay := minValidHour / 24
	maxValidDay := maxValidHour / 24
	s = fmt.Sprintf("%dd", minValidDay)
	want = time.Duration(minValidDay) * 24 * time.Hour
	f(s, want)
	s = fmt.Sprintf("%dd", maxValidDay)
	want = time.Duration(maxValidDay) * 24 * time.Hour
	f(s, want)

	minValidWeek := minValidDay / 7
	maxValidWeek := maxValidDay / 7
	s = fmt.Sprintf("%dw", minValidWeek)
	want = time.Duration(minValidWeek) * 7 * 24 * time.Hour
	f(s, want)
	s = fmt.Sprintf("%dw", maxValidWeek)
	want = time.Duration(maxValidWeek) * 7 * 24 * time.Hour
	f(s, want)

	minValidYear := minValidDay / 365
	maxValidYear := maxValidDay / 365
	s = fmt.Sprintf("%dy", minValidYear)
	want = time.Duration(minValidYear) * 365 * 24 * time.Hour
	f(s, want)
	s = fmt.Sprintf("%dy", maxValidYear)
	want = time.Duration(maxValidYear) * 365 * 24 * time.Hour
	f(s, want)
}

func TestParseDurationOutsideLimits(t *testing.T) {
	f := func(s string) {
		t.Helper()
		got, err := ParseDuration(s)
		gotDuration := time.Duration(got) * time.Millisecond
		if err == nil {
			t.Fatalf("ParseDuration(%s) unexpected result: got %d (%s), want error", s, got, gotDuration)
		}
	}

	f(fmt.Sprintf("%dms", int64(minValidMilli)-1))
	f(fmt.Sprintf("%dms", int64(maxValidMilli)+1))

	f(fmt.Sprintf("%ds", int64(minValidSecond)-1))
	f(fmt.Sprintf("%ds", int64(maxValidSecond)+1))

	minValidMinute := int64(minValidSecond)/60 - 1
	f(fmt.Sprintf("%dm", minValidMinute))
	maxValidMinute := int64(maxValidSecond)/60 + 1
	f(fmt.Sprintf("%dm", maxValidMinute))

	minValidHour := minValidMinute/60 - 1
	f(fmt.Sprintf("%dh", minValidHour))
	maxValidHour := maxValidMinute/60 + 2
	f(fmt.Sprintf("%dh", maxValidHour))

	minValidDay := minValidHour/24 - 1
	f(fmt.Sprintf("%dd", minValidDay))
	maxValidDay := maxValidHour/24 + 1
	f(fmt.Sprintf("%dd", maxValidDay))

	minValidWeek := minValidDay/7 - 1
	f(fmt.Sprintf("%dw", minValidWeek))
	maxValidWeek := maxValidDay/7 + 1
	f(fmt.Sprintf("%dw", maxValidWeek))

	minValidYear := minValidDay/365 - 1
	f(fmt.Sprintf("%dy", minValidYear))
	maxValidYear := maxValidDay/365 + 1
	f(fmt.Sprintf("%dy", maxValidYear))
}
