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
	f("1970-01-02T00:00:00Z", minTestStartTime)

	f("2024-12-10T12:10:26Z", time.Date(2024, 12, 10, 12, 10, 26, 0, time.UTC))

	// non-UTC offsets must be accepted and normalized to UTC
	f("2024-12-10T15:10:26+03:00", time.Date(2024, 12, 10, 12, 10, 26, 0, time.UTC))
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
