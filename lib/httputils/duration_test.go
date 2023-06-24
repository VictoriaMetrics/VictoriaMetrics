package httputils

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"
)

func TestGetDurationSuccess(t *testing.T) {
	f := func(s string, dExpected int64) {
		t.Helper()
		urlStr := fmt.Sprintf("http://foo.bar/baz?s=%s", url.QueryEscape(s))
		r, err := http.NewRequest(http.MethodGet, urlStr, nil)
		if err != nil {
			t.Fatalf("unexpected error in NewRequest: %s", err)
		}

		// Verify defaultValue
		d, err := GetDuration(r, "foo", 123456)
		if err != nil {
			t.Fatalf("unexpected error when obtaining default time from GetDuration(%q): %s", s, err)
		}
		if d != 123456 {
			t.Fatalf("unexpected default value for GetDuration(%q); got %d; want %d", s, d, 123456)
		}

		// Verify dExpected
		d, err = GetDuration(r, "s", 123)
		if err != nil {
			t.Fatalf("unexpected error in GetDuration(%q): %s", s, err)
		}
		if d != dExpected {
			t.Fatalf("unexpected timestamp for GetDuration(%q); got %d; want %d", s, d, dExpected)
		}
	}

	f("1.234", 1234)
	f("1.23ms", 1)
	f("1.23s", 1230)
	f("2s56ms", 2056)
	f("2s-5ms", 1995)
	f("5m3.5s", 303500)
	f("2h", 7200000)
	f("1d", 24*3600*1000)
	f("7d5h4m3s534ms", 623043534)
}

func TestGetDurationError(t *testing.T) {
	f := func(s string) {
		t.Helper()
		urlStr := fmt.Sprintf("http://foo.bar/baz?s=%s", url.QueryEscape(s))
		r, err := http.NewRequest(http.MethodGet, urlStr, nil)
		if err != nil {
			t.Fatalf("unexpected error in NewRequest: %s", err)
		}

		if _, err := GetDuration(r, "s", 123); err == nil {
			t.Fatalf("expecting non-nil error in GetDuration(%q)", s)
		}
	}

	// Negative durations aren't supported
	f("-1.234")

	// Invalid duration
	f("foo")

	// Invalid suffix
	f("1md")
}
