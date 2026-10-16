package storage

import (
	"fmt"
	"math/rand"
	"testing"
	"time"
)

func TestLegacyIndexDBContainsTimeRange(t *testing.T) {
	defer testRemoveAll(t)
	for _, disablePerDayIndex := range []bool{false, true} {
		name := fmt.Sprintf("disablePerDayIndex=%t", disablePerDayIndex)
		t.Run(name, func(t *testing.T) {
			testLegacyIndexDBContainsTimeRange(t, disablePerDayIndex)
		})
	}
}

func testLegacyIndexDBContainsTimeRange(t *testing.T, disablePerDayIndex bool) {
	defer testRemoveAll(t)

	rng := rand.New(rand.NewSource(1))
	const numMetrics = 10000
	dataTR := TimeRange{
		MinTimestamp: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli(),
		MaxTimestamp: time.Date(2026, 1, 15, 23, 59, 59, 999_999_999, time.UTC).UnixMilli(),
	}
	mrs := testGenerateMetricRowsWithPrefix(rng, numMetrics, "metric", dataTR)

	// fill legacy index with data
	s := MustOpenStorage(t.Name(), OpenOptions{
		DisablePerDayIndex: disablePerDayIndex,
	})
	s.AddRows(mrs, defaultPrecisionBits)
	s.DebugFlush()
	s = mustConvertToLegacy(s)
	defer s.MustClose()

	legacyIDBs := s.getLegacyIndexDBs()
	defer s.putLegacyIndexDBs(legacyIDBs)
	// Legacy indexDB time range is [0, MaxInt64], while its dataTR is
	// [2026-01-01T00:00:00.000Z, 2026-01-15T23:59:59.999Z].
	db := legacyIDBs.getIDBPrev()

	f := func(tr TimeRange, wantContains bool, wantTR TimeRange) {
		t.Helper()
		gotContains, gotTR := db.containsTimeRange(tr)
		if gotContains != wantContains {
			t.Fatalf("unexpected containsTimeRange() result: got %t, want %t", gotContains, wantContains)
		}
		if wantContains && disablePerDayIndex {
			wantTR = globalIndexTimeRange
		}
		if gotTR != wantTR {
			t.Fatalf("unexpected containsTimeRange() adjusted time range: got %s, want %s", &gotTR, &wantTR)
		}
	}

	var tr TimeRange

	// Time range is fully included into indexDB tr but before indexDB dataTR.
	//
	// Still expect indexDB to contain the time range because
	// lecacyContainsTimeRange() only checks whether tr.MinTimestamp is greater
	// than dataTR.MaxTimestamp.
	tr.MinTimestamp = time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC).UnixMilli()
	tr.MaxTimestamp = time.Date(2025, 12, 31, 23, 59, 59, 999_999_999, time.UTC).UnixMilli()
	f(tr, true, tr)

	// Time range is fully included into indexDB tr and overlaps with indexDB
	// dataTR on first millisecond.
	tr.MinTimestamp = time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC).UnixMilli()
	tr.MaxTimestamp = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli()
	f(tr, true, tr)

	// Time range is fully included into indexDB tr and overlaps with indexDB
	// dataTR on the left.
	tr.MinTimestamp = time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC).UnixMilli()
	tr.MaxTimestamp = time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC).UnixMilli()
	f(tr, true, tr)

	// Time range is fully included into indexDB tr and fully included into
	// dataTR on the left.
	tr.MinTimestamp = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli()
	tr.MaxTimestamp = time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC).UnixMilli()
	f(tr, true, tr)

	// Time range is fully included into indexDB tr and fully included into
	// dataTR.
	tr.MinTimestamp = time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC).UnixMilli()
	tr.MaxTimestamp = time.Date(2026, 1, 10, 0, 0, 0, 0, time.UTC).UnixMilli()
	f(tr, true, tr)

	// Time range is fully included into indexDB tr and fully included into
	// dataTR on the right.
	tr.MinTimestamp = time.Date(2026, 1, 10, 0, 0, 0, 0, time.UTC).UnixMilli()
	tr.MaxTimestamp = time.Date(2026, 1, 15, 23, 59, 59, 999_000_000, time.UTC).UnixMilli()
	f(tr, true, tr)

	// Time range is fully included into indexDB tr and overlaps with dataTR on
	// the right
	tr.MinTimestamp = time.Date(2026, 1, 10, 0, 0, 0, 0, time.UTC).UnixMilli()
	tr.MaxTimestamp = time.Date(2026, 1, 20, 0, 0, 0, 0, time.UTC).UnixMilli()
	f(tr, true, tr)

	// Time range is fully included into indexDB tr and overlaps with dataTR on
	// the right on the last millisecond
	tr.MinTimestamp = time.Date(2026, 1, 15, 23, 59, 59, 999_000_000, time.UTC).UnixMilli()
	tr.MaxTimestamp = time.Date(2026, 1, 20, 0, 0, 0, 0, time.UTC).UnixMilli()
	f(tr, true, tr)

	// Time range is fully included into indexDB tr but it is after dataTR.
	tr.MinTimestamp = time.Date(2026, 1, 16, 0, 0, 0, 0, time.UTC).UnixMilli()
	tr.MaxTimestamp = time.Date(2026, 1, 20, 0, 0, 0, 0, time.UTC).UnixMilli()
	if disablePerDayIndex {
		f(tr, true, tr)
	} else {
		f(tr, false, tr)
	}
}
