package vmstorage

import "testing"

func TestParseLogNewSeriesSeconds(t *testing.T) {
	got, err := parseLogNewSeriesSeconds("")
	if err != nil || got != defaultLogNewSeriesSeconds {
		t.Fatalf("empty: got %d, %v", got, err)
	}
	got, err = parseLogNewSeriesSeconds("120")
	if err != nil || got != 120 {
		t.Fatalf("120: got %d, %v", got, err)
	}
	if _, err = parseLogNewSeriesSeconds("-1"); err == nil {
		t.Fatal("expected negative seconds to be rejected")
	}
	if _, err = parseLogNewSeriesSeconds("0"); err == nil {
		t.Fatal("expected zero seconds to be rejected")
	}
	if _, err = parseLogNewSeriesSeconds("999999999"); err == nil {
		t.Fatal("expected huge seconds to be rejected")
	}
}
