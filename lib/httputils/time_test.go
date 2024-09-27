package httputils

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
		r, err := http.NewRequest(http.MethodGet, urlStr, nil)
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

	f("2019Z", 1546300800000)
	f("2019-01Z", 1546300800000)
	f("2019-02Z", 1548979200000)
	f("2019-02-01Z", 1548979200000)
	f("2019-02-02Z", 1549065600000)
	f("2019-02-02T00Z", 1549065600000)
	f("2019-02-02T01Z", 1549069200000)
	f("2019-02-02T01:00Z", 1549069200000)
	f("2019-02-02T01:01Z", 1549069260000)
	f("2019-02-02T01:01:00Z", 1549069260000)
	f("2019-02-02T01:01:01Z", 1549069261000)
	f("2020-02-21T16:07:49.433Z", 1582301269433)
	f("2019-07-07T20:47:40+03:00", 1562521660000)
	f("-292273086-05-16T16:47:06Z", minTimeMsecs)
	f("292277025-08-18T07:12:54.999999999Z", maxTimeMsecs)
	f("1562529662.324", 1562529662324)
	f("-9223372036.854", minTimeMsecs)
	f("-9223372036.855", minTimeMsecs)
	f("1223372036.855", 1223372036855)
}

func TestGetTimeError(t *testing.T) {
	f := func(s string) {
		t.Helper()
		urlStr := fmt.Sprintf("http://foo.bar/baz?s=%s", url.QueryEscape(s))
		r, err := http.NewRequest(http.MethodGet, urlStr, nil)
		if err != nil {
			t.Fatalf("unexpected error in NewRequest: %s", err)
		}

		if _, err := GetTime(r, "s", 123); err == nil {
			t.Fatalf("expecting non-nil error in GetTime(%q)", s)
		}
	}

	f("foo")
	f("foo1")
	f("1245-5")
	f("2022-x7")
	f("2022-02-x7")
	f("2022-02-02Tx7")
	f("2022-02-02T00:x7")
	f("2022-02-02T00:00:x7")
	f("2022-02-02T00:00:00a")
	f("2019-07-07T20:01:02Zisdf")
	f("2019-07-07T20:47:40+03:00123")
	f("-292273086-05-16T16:47:07Z")
	f("292277025-08-18T07:12:54.999999998Z")
	f("123md")
	f("-12.3md")
}
