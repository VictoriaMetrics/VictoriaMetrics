package prometheus

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
		ts, err := getTime(r, "foo", 123)
		if err != nil {
			t.Fatalf("unexpected error when obtaining default time from getTime(%q): %s", s, err)
		}
		if ts != 123 {
			t.Fatalf("unexpected default value for getTime(%q); got %d; want %d", s, ts, 123)
		}

		// Verify timestampExpected
		ts, err = getTime(r, "s", 123)
		if err != nil {
			t.Fatalf("unexpected error in getTime(%q): %s", s, err)
		}
		if ts != timestampExpected {
			t.Fatalf("unexpected timestamp for getTime(%q); got %d; want %d", s, ts, timestampExpected)
		}
	}

	f("2019-07-07T20:01:02Z", 1562529662000)
	f("2019-07-07T20:47:40+03:00", 1562521660000)
	f("-292273086-05-16T16:47:06Z", -9223372036854)
	f("292277025-08-18T07:12:54.999999999Z", 9223372036854)
	f("1562529662.324", 1562529662324)
	f("-9223372036.854", -9223372036854)
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
		ts, err := getTime(r, "foo", 123)
		if err != nil {
			t.Fatalf("unexpected error when obtaining default time from getTime(%q): %s", s, err)
		}
		if ts != 123 {
			t.Fatalf("unexpected default value for getTime(%q); got %d; want %d", s, ts, 123)
		}

		// Verify timestampExpected
		_, err = getTime(r, "s", 123)
		if err == nil {
			t.Fatalf("expecting non-nil error in getTime(%q)", s)
		}
	}

	f("foo")
	f("2019-07-07T20:01:02Zisdf")
	f("2019-07-07T20:47:40+03:00123")
	f("-292273086-05-16T16:47:07Z")
	f("292277025-08-18T07:12:54.999999998Z")
	f("-9223372036.855")
	f("9223372036.855")
}
