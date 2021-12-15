package storage

import (
	"strings"
	"testing"
)

func TestParseDownsamplingPeriodsFailure(t *testing.T) {
	f := func(name string, src []string) {
		t.Helper()
		t.Run(name, func(t *testing.T) {
			if _, err := parseDownsamplingPeriods(src); err == nil {
				t.Fatalf("want fail for input: %s", strings.Join(src, ","))
			}
		})
	}
	f("empty duration", []string{"15d"})
	f("empty interval", []string{":1m"})
	f("incorrect duration decrease", []string{"30d:15h", "60d:1h"})
	f("duplicate offset", []string{"30d:15h", "30d:1h"})
	f("duplicate interval", []string{"60d:1h", "30d:1h"})
	f("not multiple intervals", []string{"90d:12h", "60:9h", "30d:7h"})
}

func TestParseDownsamplingPeriodsSuccess(t *testing.T) {
	f := func(name string, src []string, expected []DownsamplingPeriod) {
		t.Helper()
		t.Run(name, func(t *testing.T) {
			dsps, err := parseDownsamplingPeriods(src)
			if err != nil {
				t.Fatalf("cannot parse downsampling configuration for: %s, err: %s", strings.Join(src, ","), err)
			}
			assertDownsamplingPeriods(t, expected, dsps)
		})
	}
	f("one period", []string{"30d:1m"}, []DownsamplingPeriod{
		{Offset: 30 * 24 * 3600 * 1000, Interval: 60 * 1000},
	})
	f("three periods", []string{"15d:30s", "30d:1m", "60d:15m"}, []DownsamplingPeriod{
		{Offset: 60 * 24 * 3600 * 1000, Interval: 15 * 60 * 1000},
		{Offset: 30 * 24 * 3600 * 1000, Interval: 60 * 1000},
		{Offset: 15 * 24 * 3600 * 1000, Interval: 30 * 1000},
	})
	f("with the same divider periods", []string{"15d:1m", "30d:7m", "60d:14m", "90d:28m"}, []DownsamplingPeriod{
		{Offset: 90 * 24 * 3600 * 1000, Interval: 28 * 60 * 1000},
		{Offset: 60 * 24 * 3600 * 1000, Interval: 14 * 60 * 1000},
		{Offset: 30 * 24 * 3600 * 1000, Interval: 7 * 60 * 1000},
		{Offset: 15 * 24 * 3600 * 1000, Interval: 60 * 1000},
	})
}

func assertDownsamplingPeriods(t *testing.T, want, got []DownsamplingPeriod) {
	t.Helper()
	if len(want) != len(got) {
		t.Fatalf("len mismatch, want: %d, got: %d", len(want), len(got))
	}
	for i := 0; i < len(want); i++ {
		if want[i] != got[i] {
			t.Fatalf("want period: %s, got period: %s, idx: %d", want[i], got[i], i)
		}
	}
}
