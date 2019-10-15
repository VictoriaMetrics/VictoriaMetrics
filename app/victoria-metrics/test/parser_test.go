package test

import (
	"testing"
	"time"
)

func TestPopulateTimeTplString(t *testing.T) {
	now, err := time.Parse(time.RFC3339, "2006-01-02T15:04:05Z")
	if err != nil {
		t.Fatalf("unexpected error when parsing time: %s", err)
	}
	f := func(s, resultExpected string) {
		t.Helper()
		result := PopulateTimeTplString(s, now)
		if result != resultExpected {
			t.Fatalf("unexpected result; got %q; want %q", result, resultExpected)
		}
	}
	f("", "")
	f("{TIME_S}", "1136214245")
	f("now: {TIME_S}, past 30s: {TIME_MS-30s}, now: {TIME_S}", "now: 1136214245, past 30s: 1136214215000, now: 1136214245")
	f("now: {TIME_MS}, past 30m: {TIME_MSZ-30m}, past 2h: {TIME_NS-2h}", "now: 1136214245000, past 30m: 1136212445000, past 2h: 1136207045000000000")
}
