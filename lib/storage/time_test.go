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
