package fasttime

import (
	"testing"
	"time"
)

func TestUnixTimestamp(t *testing.T) {
	tsExpected := uint64(time.Now().Unix())
	ts := UnixTimestamp()
	if ts-tsExpected > 1 {
		t.Fatalf("unexpected UnixTimestamp; got %d; want %d", ts, tsExpected)
	}
}

func TestUnixDate(t *testing.T) {
	dateExpected := uint64(time.Now().Unix() / (24 * 3600))
	date := UnixDate()
	if date-dateExpected > 1 {
		t.Fatalf("unexpected UnixDate; got %d; want %d", date, dateExpected)
	}
}

func TestUnixHour(t *testing.T) {
	hourExpected := uint64(time.Now().Unix() / 3600)
	hour := UnixHour()
	if hour-hourExpected > 1 {
		t.Fatalf("unexpected UnixHour; got %d; want %d", hour, hourExpected)
	}
}

func TestUnixTime(t *testing.T) {
	tsExpected := time.Now()
	ts := UnixTime()
	if ts.Sub(tsExpected) > time.Second {
		t.Fatalf("unexpected UnixTime; got %s; want %s", ts, tsExpected)
	}
}
