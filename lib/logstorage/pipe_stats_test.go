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
	f("1B", 1)
	f("1K", 1_000)
	f("1KB", 1_000)
	f("5.5KiB", 5.5*(1<<10))
	f("10MB500KB10B", 10*1_000_000+500*1_000+10)
	f("10M", 10*1_000_000)
	f("-10MB", -10*1_000_000)

	// ipv4 mask
	f("/0", 1<<32)
	f("/32", 1)
	f("/16", 1<<16)
	f("/8", 1<<24)
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
}

func TestTryParseBucketOffset_Success(t *testing.T) {
	f := func(s string, resultExpected float64) {
		t.Helper()

		result, ok := tryParseBucketOffset(s)
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
	f("1B", 1)
	f("1K", 1_000)
	f("1KB", 1_000)
	f("5.5KiB", 5.5*(1<<10))
	f("10MB500KB10B", 10*1_000_000+500*1_000+10)
	f("10M", 10*1_000_000)
	f("-10MB", -10*1_000_000)
}

func TestTryParseBucketOffset_Failure(t *testing.T) {
	f := func(s string) {
		t.Helper()

		_, ok := tryParseBucketOffset(s)
		if ok {
			t.Fatalf("expecting error when parsing %q", s)
		}
	}

	f("")
	f("foo")
}
