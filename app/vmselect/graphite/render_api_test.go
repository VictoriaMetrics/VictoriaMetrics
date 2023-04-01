package graphite

import (
	"testing"
	"time"
)

func TestParseIntervalSuccess(t *testing.T) {
	f := func(s string, intervalExpected int64) {
		t.Helper()
		interval, err := parseInterval(s)
		if err != nil {
			t.Fatalf("unexpected error in parseInterva(%q): %s", s, err)
		}
		if interval != intervalExpected {
			t.Fatalf("unexpected result for parseInterval(%q); got %d; want %d", s, interval, intervalExpected)
		}
	}
	f(`1ms`, 1)
	f(`-10.5ms`, -10)
	f(`+5.5s`, 5500)
	f(`7.85s`, 7850)
	f(`-7.85sec`, -7850)
	f(`-7.85secs`, -7850)
	f(`5seconds`, 5000)
	f(`10min`, 10*60*1000)
	f(`10 mins`, 10*60*1000)
	f(` 10  mins `, 10*60*1000)
	f(`10m`, 10*60*1000)
	f(`-10.5min`, -10.5*60*1000)
	f(`-10.5m`, -10.5*60*1000)
	f(`3minutes`, 3*60*1000)
	f(`3h`, 3*3600*1000)
	f(`-4.5hour`, -4.5*3600*1000)
	f(`7hours`, 7*3600*1000)
	f(`5d`, 5*24*3600*1000)
	f(`-3.5days`, -3.5*24*3600*1000)
	f(`0.5w`, 0.5*7*24*3600*1000)
	f(`10weeks`, 10*7*24*3600*1000)
	f(`2months`, 2*30*24*3600*1000)
	f(`2mo`, 2*30*24*3600*1000)
	f(`1.2y`, 1.2*365*24*3600*1000)
	f(`-3years`, -3*365*24*3600*1000)
}

func TestParseIntervalError(t *testing.T) {
	f := func(s string) {
		t.Helper()
		interval, err := parseInterval(s)
		if err == nil {
			t.Fatalf("expecting non-nil error for parseInterval(%q)", s)
		}
		if interval != 0 {
			t.Fatalf("unexpected non-zero interval for parseInterval(%q): %d", s, interval)
		}
	}
	f("")
	f("foo")
	f(`'1minute'`)
	f(`123`)
}

func TestParseTimeSuccess(t *testing.T) {
	startTime := time.Now()
	startTimestamp := startTime.UnixNano() / 1e6
	f := func(s string, timestampExpected int64) {
		t.Helper()
		timestamp, err := parseTime(startTime, s)
		if err != nil {
			t.Fatalf("unexpected error from parseTime(%q): %s", s, err)
		}
		if timestamp != timestampExpected {
			t.Fatalf("unexpected timestamp returned from parseTime(%q); got %d; want %d", s, timestamp, timestampExpected)
		}
	}
	f("now", startTimestamp)
	f("today", startTimestamp-startTimestamp%msecsPerDay)
	f("yesterday", startTimestamp-(startTimestamp%msecsPerDay)-msecsPerDay)
	f("1234567890", 1234567890000)
	f("18:36_20210223", 1614105360000)
	f("20210223", 1614038400000)
	f("02/23/21", 1614038400000)
	f("2021-02-23", 1614038400000)
	f("2021-02-23T18:36:12Z", 1614105372000)
	f("-3hours", startTimestamp-3*3600*1000)
	f("1.5minutes", startTimestamp+1.5*60*1000)
}

func TestParseTimeFailure(t *testing.T) {
	f := func(s string) {
		t.Helper()
		timestamp, err := parseTime(time.Now(), s)
		if err == nil {
			t.Fatalf("expecting non-nil error for parseTime(%q)", s)
		}
		if timestamp != 0 {
			t.Fatalf("expecting zero value for parseTime(%q); got %d", s, timestamp)
		}
	}
	f("")
	f("foobar")
	f("1235aafb")
}
