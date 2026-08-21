package unittest

import (
	"testing"
	"time"
)

func TestUnitTest_Failure(t *testing.T) {
	f := func(files []string) {
		t.Helper()

		failed := UnitTest(files, false, nil, "", "", "", "")
		if !failed {
			t.Fatalf("expecting failed test")
		}
	}

	f([]string{"./testdata/failed-test-with-missing-rulefile.yaml"})

	f([]string{"./testdata/failed-test.yaml"})
}

func TestUnitTest_Success(t *testing.T) {
	f := func(disableGroupLabel bool, files []string, externalLabels []string, externalURL, httpPort string) {
		t.Helper()

		failed := UnitTest(files, disableGroupLabel, externalLabels, externalURL, httpPort, "", "")
		if failed {
			t.Fatalf("unexpected failed test")
		}
	}

	// run multi files with random http port
	f(false, []string{"./testdata/test1.yaml", "./testdata/test2.yaml"}, []string{"cluster=prod"}, "http://grafana:3000", "")

	// disable group label
	// template with null external values
	// specify httpListenAddr
	f(true, []string{"./testdata/disable-group-label.yaml"}, nil, "", "8880")
}

func TestParseTestStartTime_Success(t *testing.T) {
	f := func(s string, resultExpected time.Time) {
		t.Helper()

		result, err := parseTestStartTime(s)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		if !result.Equal(resultExpected) {
			t.Fatalf("unexpected start time; got %s; want %s", result, resultExpected)
		}
	}

	// empty value must keep the default start time
	f("", defaultTestStartTime)

	f("2000-01-01T00:00:00Z", defaultTestStartTime)

	// the earliest start time vmstorage can hold samples at
	f("1970-01-02T00:00:00Z", minTestTime)

	f("2024-12-10T12:10:26Z", time.Date(2024, 12, 10, 12, 10, 26, 0, time.UTC))

	// non-UTC offsets must be accepted and normalized to UTC
	f("2024-12-10T15:10:26+03:00", time.Date(2024, 12, 10, 12, 10, 26, 0, time.UTC))

	// a start time in the future is legal as long as the samples still land inside the
	// window vmstorage accepts. -futureRetention would cut this off after two days if
	// processFlags did not widen it.
	future := time.Now().UTC().AddDate(0, 0, 30).Truncate(time.Second)
	f(future.Format(time.RFC3339), future)

	// the latest start time this storage accepts
	f(maxTestTime.Format(time.RFC3339Nano), maxTestTime)
}

func TestParseTestStartTime_Failure(t *testing.T) {
	f := func(s string) {
		t.Helper()

		if _, err := parseTestStartTime(s); err == nil {
			t.Fatalf("expecting non-nil error for %q", s)
		}
	}

	// missing timezone
	f("2000-01-01T00:00:00")

	// date only
	f("2000-01-01")

	// unix timestamp is not RFC3339
	f("946684800")

	f("foobar")

	// the first day of the Unix epoch is reserved for the global index search:
	// input_series seeded there are dropped, so rules never see them
	f("1970-01-01T00:00:00Z")
	f("1970-01-01T23:59:59Z")
	f("1969-12-31T00:00:00Z")

	// beyond the window vmstorage accepts: samples seeded there are dropped just as
	// silently as the ones below minTestTime
	f(maxTestTime.Add(time.Millisecond).Format(time.RFC3339Nano))
	f("2300-01-01T00:00:00Z")
	f("9999-12-31T23:59:59Z")
}

func TestCalcMaxTestTime(t *testing.T) {
	f := func(now, resultExpected time.Time) {
		t.Helper()

		result := calcMaxTestTime(now)
		if !result.Equal(resultExpected) {
			t.Fatalf("unexpected max test time for now=%s; got %s; want %s", now, result, resultExpected)
		}
	}

	// While now+100y is still representable, -futureRetention is the binding limit.
	//
	// The expected value is written out rather than computed, because the whole point of the
	// case is that 100y is 36500 days and not 100 calendar years: AddDate(100, 0, 0) here
	// would say 2126-01-01, which is 24 leap days later than vmstorage would accept.
	f(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), time.Date(2125, 12, 8, 0, 0, 0, 0, time.UTC))

	// once it is not, the storage limit is
	f(time.Date(2200, 1, 1, 0, 0, 0, 0, time.UTC), maxStorageTime)

	// exactly at the crossover
	f(maxStorageTime.Add(-testRetentionDuration), maxStorageTime)

	// One day below the crossover the retention is still what binds, so the result must be
	// one day below maxStorageTime -- not maxStorageTime. This is the case calendar
	// arithmetic got wrong: AddDate(100, 0, 0) overshoots into the clamp and reports the
	// storage limit for every `now` in the last ~24 days before the crossover, accepting
	// start times whose samples are then dropped.
	f(maxStorageTime.Add(-testRetentionDuration).Add(-24*time.Hour), maxStorageTime.Add(-24*time.Hour))
}

func TestTestRetentionDuration(t *testing.T) {
	// vmstorage compares sample timestamps against now+(-futureRetention), and the flag
	// parses `y` as 365 days. If this ever stops being true, maxTestTime silently starts
	// accepting start times whose samples never land.
	if got, want := testRetentionDuration, 100*365*24*time.Hour; got != want {
		t.Fatalf("unexpected testRetentionDuration for testRetention=%q; got %s; want %s", testRetention, got, want)
	}
}

func TestCheckTestTime(t *testing.T) {
	f := func(ts time.Time, wantErr bool) {
		t.Helper()

		err := checkTestTime(ts, "the thing under test")
		if wantErr && err == nil {
			t.Fatalf("expecting non-nil error for %s", ts)
		}
		if !wantErr && err != nil {
			t.Fatalf("unexpected error for %s: %s", ts, err)
		}
	}

	f(defaultTestStartTime, false)
	f(minTestTime, false)
	f(maxTestTime, false)

	f(minTestTime.Add(-time.Millisecond), true)
	f(maxTestTime.Add(time.Millisecond), true)

	// a legal start time whose evaluation offset walks past the end of the window
	f(maxTestTime.Add(4*time.Minute), true)
}

func TestUnitTest_StartTime(t *testing.T) {
	defer func() {
		testStartTime = defaultTestStartTime
	}()

	f := func(startTime string, resultExpected time.Time) {
		t.Helper()

		testStartTime = time.Time{}
		failed := UnitTest([]string{"./testdata/test1.yaml"}, false, []string{"cluster=prod"}, "http://grafana:3000", "", "", startTime)
		if failed {
			t.Fatalf("unexpected failed test")
		}
		if !testStartTime.Equal(resultExpected) {
			t.Fatalf("unexpected start time; got %s; want %s", testStartTime, resultExpected)
		}
	}

	f("", defaultTestStartTime)

	f("2020-06-01T00:00:00Z", time.Date(2020, 6, 1, 0, 0, 0, 0, time.UTC))
}
