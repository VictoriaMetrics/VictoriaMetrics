package searchutils

import (
	"fmt"
	"net/http"
	"net/url"
	"testing"
)

func TestGetTimeSuccess(t *testing.T) {
	f := func(s string, timestampExpected int64) {
		t.Helper()
		urlStr := fmt.Sprintf("http://foo.bar/baz?s=%s", url.QueryEscape(s))
		r, err := http.NewRequest("GET", urlStr, nil)
		if err != nil {
			t.Fatalf("unexpected error in NewRequest: %s", err)
		}

		// Verify defaultValue
		ts, err := GetTime(r, "foo", 123456)
		if err != nil {
			t.Fatalf("unexpected error when obtaining default time from GetTime(%q): %s", s, err)
		}
		if ts != 123000 {
			t.Fatalf("unexpected default value for GetTime(%q); got %d; want %d", s, ts, 123000)
		}

		// Verify timestampExpected
		ts, err = GetTime(r, "s", 123)
		if err != nil {
			t.Fatalf("unexpected error in GetTime(%q): %s", s, err)
		}
		if ts != timestampExpected {
			t.Fatalf("unexpected timestamp for GetTime(%q); got %d; want %d", s, ts, timestampExpected)
		}
	}

	f("2019-07-07T20:01:02Z", 1562529662000)
	f("2019-07-07T20:47:40+03:00", 1562521660000)
	f("-292273086-05-16T16:47:06Z", minTimeMsecs)
	f("292277025-08-18T07:12:54.999999999Z", maxTimeMsecs)
	f("1562529662.324", 1562529662324)
	f("-9223372036.854", minTimeMsecs)
	f("-9223372036.855", minTimeMsecs)
	f("9223372036.855", maxTimeMsecs)
}

func TestGetTimeError(t *testing.T) {
	f := func(s string) {
		t.Helper()
		urlStr := fmt.Sprintf("http://foo.bar/baz?s=%s", url.QueryEscape(s))
		r, err := http.NewRequest("GET", urlStr, nil)
		if err != nil {
			t.Fatalf("unexpected error in NewRequest: %s", err)
		}

		// Verify defaultValue
		ts, err := GetTime(r, "foo", 123456)
		if err != nil {
			t.Fatalf("unexpected error when obtaining default time from GetTime(%q): %s", s, err)
		}
		if ts != 123000 {
			t.Fatalf("unexpected default value for GetTime(%q); got %d; want %d", s, ts, 123000)
		}

		// Verify timestampExpected
		_, err = GetTime(r, "s", 123)
		if err == nil {
			t.Fatalf("expecting non-nil error in GetTime(%q)", s)
		}
	}

	f("foo")
	f("2019-07-07T20:01:02Zisdf")
	f("2019-07-07T20:47:40+03:00123")
	f("-292273086-05-16T16:47:07Z")
	f("292277025-08-18T07:12:54.999999998Z")
}
