package logstorage

import (
	"testing"
)

func TestTryParseBucketSize_Success(t *testing.T) {
	f := func(s string, resultExpected float64) {
		t.Helper()

		result, ok := tryParseBucketSize(s)
		if !ok {
			t.Fatalf("cannot parse %q", s)
		}
		if result != resultExpected {
			t.Fatalf("unexpected result; got %f; want %f", result, resultExpected)
		}
	}

	// integers
	f("0", 0)
	f("123", 123)
	f("1_234_678", 1234678)
	f("-1_234_678", -1234678)

	// floating-point numbers
	f("0.0", 0)
	f("123.435", 123.435)
	f("1_000.433_344", 1000.433344)
	f("-1_000.433_344", -1000.433344)

	// durations
	f("5m", 5*nsecsPerMinute)
	f("1h5m3.5s", nsecsPerHour+5*nsecsPerMinute+3.5*nsecsPerSecond)
	f("-1h5m3.5s", -(nsecsPerHour + 5*nsecsPerMinute + 3.5*nsecsPerSecond))

	// bytes
	f("1b", 1)
	f("1k", 1_000)
	f("1Kb", 1_000)
	f("5.5KiB", 5.5*(1<<10))
	f("10MB500KB10B", 10*1_000_000+500*1_000+10)
	f("10m0k", 10*1_000_000)
}

func TestTryParseBucketSize_Failure(t *testing.T) {
	f := func(s string) {
		t.Helper()

		_, ok := tryParseBucketSize(s)
		if ok {
			t.Fatalf("expecting error when parsing %q", s)
		}
	}

	f("")
	f("foo")

	// negative bytes are forbidden
	f("-10MB")
}
