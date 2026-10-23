package unittest

import (
	"testing"
	"time"
)

func TestUnitTest_Failure(t *testing.T) {
	f := func(files []string) {
		t.Helper()

		failed := UnitTest(files, false, nil, "", "", "")
		if !failed {
			t.Fatalf("expecting failed test")
		}
	}

	f([]string{"./testdata/failed-test-with-missing-rulefile.yaml"})

	f([]string{"./testdata/failed-test.yaml"})

	// start_timestamp on 1970-01-01
	f([]string{"./testdata/failed-start-timestamp.yaml"})

	// start_timestamp with unsupported format
	f([]string{"./testdata/failed-start-timestamp-format.yaml"})
}

func TestUnitTest_Success(t *testing.T) {
	f := func(disableGroupLabel bool, files []string, externalLabels []string, externalURL, httpPort string) {
		t.Helper()

		failed := UnitTest(files, disableGroupLabel, externalLabels, externalURL, httpPort, "")
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

	// custom start_timestamp
	f(false, []string{"./testdata/start-timestamp.yaml"}, []string{"cluster=prod"}, "http://grafana:3000", "")
}

func TestParseStartTimestamp(t *testing.T) {
	f := func(s string, expected time.Time) {
		t.Helper()

		got, err := parseStartTimestamp(s)
		if err != nil {
			t.Fatalf("unexpected error: %s", err)
		}
		if !got.Equal(expected) {
			t.Fatalf("unexpected result; got %s; want %s", got, expected)
		}
	}

	f("1700000000", time.Unix(1700000000, 0))
	f("1700000000.5", time.Unix(1700000000, 5e8))
	f("2024-06-01T12:00:00Z", time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC))
	f("2024-06-01T14:00:00+02:00", time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC))
}

func TestParseStartTimestamp_Failure(t *testing.T) {
	f := func(s string) {
		t.Helper()

		if _, err := parseStartTimestamp(s); err == nil {
			t.Fatalf("expecting non-nil error for %q", s)
		}
	}

	f("")
	f("yesterday")
	f("2024-06-01")
	f("2024-06-01 12:00:00")
}
