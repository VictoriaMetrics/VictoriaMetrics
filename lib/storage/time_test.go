package storage

import (
	"testing"
	"time"
)

func TestTimeRangeFromPartition(t *testing.T) {
	for i := 0; i < 24*30*365; i++ {
		testTimeRangeFromPartition(t, time.Now().Add(time.Hour*time.Duration(i)))
	}
}

func testTimeRangeFromPartition(t *testing.T, initialTime time.Time) {
	t.Helper()

	y, m, _ := initialTime.UTC().Date()
	var tr TimeRange
	tr.fromPartitionTime(initialTime)

	minTime := timestampToTime(tr.MinTimestamp)
	minY, minM, _ := minTime.Date()
	if minY != y {
		t.Fatalf("unexpected year for MinTimestamp; got %d; want %d", minY, y)
	}
	if minM != m {
		t.Fatalf("unexpected month for MinTimestamp; got %d; want %d", minM, m)
	}

	// Verify that the previous millisecond form tr.MinTimestamp belongs to the previous month.
	tr.MinTimestamp--
	prevTime := timestampToTime(tr.MinTimestamp)
	prevY, prevM, _ := prevTime.Date()
	if prevY*12+int(prevM-1)+1 != minY*12+int(minM-1) {
		t.Fatalf("unexpected prevY, prevM; got %d, %d; want %d, %d+1;\nprevTime=%s\nminTime=%s", prevY, prevM, minY, minM, prevTime, minTime)
	}

	maxTime := timestampToTime(tr.MaxTimestamp)
	maxY, maxM, _ := maxTime.Date()
	if maxY != y {
		t.Fatalf("unexpected year for MaxTimestamp; got %d; want %d", maxY, y)
	}
	if maxM != m {
		t.Fatalf("unexpected month for MaxTimestamp; got %d; want %d", maxM, m)
	}

	// Verify that the next millisecond from tr.MaxTimestamp belongs to the next month.
	tr.MaxTimestamp++
	nextTime := timestampToTime(tr.MaxTimestamp)
	nextY, nextM, _ := nextTime.Date()
	if nextY*12+int(nextM-1)-1 != maxY*12+int(maxM-1) {
		t.Fatalf("unexpected nextY, nextM; got %d, %d; want %d, %d+1;\nnextTime=%s\nmaxTime=%s", nextY, nextM, maxY, maxM, nextTime, maxTime)
	}
}

func TestTimeRangeDateRange(t *testing.T) {
	f := func(tr TimeRange, wantMinDate, wantMaxDate uint64) {
		t.Helper()

		gotMinDate, gotMaxDate := tr.DateRange()
		if gotMinDate != wantMinDate {
			t.Errorf("unexpected min date: got %d, want %d", gotMinDate, wantMinDate)
		}
		if gotMaxDate != wantMaxDate {
			t.Errorf("unexpected max date: got %d, want %d", gotMaxDate, wantMaxDate)
		}
	}

	var tr TimeRange

	// MinTimestamp is less than MaxTimestamp, the timestamps belong to the
	// different days. Min date must be less than the max date.
	tr = TimeRange{1*msecPerDay + 123, 2*msecPerDay + 456}
	f(tr, 1, 2)

	// MinTimestamp is less than MaxTimestamp and both timestamps belong to the
	// same day. Max date must be the same as min date.
	tr = TimeRange{1*msecPerDay + 123, 1*msecPerDay + 456}
	f(tr, 1, 1)

	// MinTimestamp equals to MaxTimestamp. Max date must be the same as min
	// date.
	tr = TimeRange{1*msecPerDay + 123, 1*msecPerDay + 123}
	f(tr, 1, 1)

	// MinTimestamp is the first millisecond of the day and equals to
	// MaxTimestamp. Min and max dates must be the same.
	tr = TimeRange{1 * msecPerDay, 1 * msecPerDay}
	f(tr, 1, 1)

	// MinTimestamp is greater than MaxTimestamp MaxTimestamp. Max date must be
	// the same as min date.
	tr = TimeRange{2*msecPerDay + 654, 1*msecPerDay + 321}
	f(tr, 2, 2)
}

func TestDateToString(t *testing.T) {
	f := func(date uint64, want string) {
		t.Helper()

		if got := dateToString(date); got != want {
			t.Errorf("dateToString(%d) unexpected return value: got %q, want %q", date, got, want)
		}
	}

	f(globalIndexDate, "[entire retention period]")
	f(1, "1970-01-02")
	f(10, "1970-01-11")
}

func TestTimeRangeString(t *testing.T) {
	f := func(tr TimeRange, want string) {
		t.Helper()

		if got := tr.String(); got != want {
			t.Errorf("TimeRange.String() unexpected return value: got %q, want %q", got, want)
		}
	}

	f(globalIndexTimeRange, "[entire retention period]")
	f(TimeRange{
		MinTimestamp: 0,
		MaxTimestamp: 1,
	}, "[1970-01-01T00:00:00Z..1970-01-01T00:00:00.001Z]")
	f(TimeRange{
		MinTimestamp: 1,
		MaxTimestamp: 2,
	}, "[1970-01-01T00:00:00.001Z..1970-01-01T00:00:00.002Z]")
	f(TimeRange{
		MinTimestamp: time.Date(2024, 9, 6, 0, 0, 0, 000, time.UTC).UnixMilli(),
		MaxTimestamp: time.Date(2024, 9, 7, 0, 0, 0, 000, time.UTC).UnixMilli() - 1,
	}, "[2024-09-06T00:00:00Z..2024-09-06T23:59:59.999Z]")
}
