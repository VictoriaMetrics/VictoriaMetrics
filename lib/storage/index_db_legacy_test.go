package storage

import (
	"math/rand"
	"testing"
	"time"
)

func TestLegacyContainsTimeRange(t *testing.T) {
	defer testRemoveAll(t)

	rng := rand.New(rand.NewSource(1))
	const numMetrics = 10000
	trPrev := TimeRange{
		MinTimestamp: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli(),
		MaxTimestamp: time.Date(2025, 1, 15, 23, 59, 59, 999_999_999, time.UTC).UnixMilli(),
	}
	trCurr := TimeRange{
		MinTimestamp: time.Date(2025, 1, 16, 0, 0, 0, 0, time.UTC).UnixMilli(),
		MaxTimestamp: time.Date(2025, 1, 31, 23, 59, 59, 999_999_999, time.UTC).UnixMilli(),
	}
	trPt := TimeRange{
		MinTimestamp: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli(),
		MaxTimestamp: time.Date(2025, 1, 31, 23, 59, 59, 999_999_999, time.UTC).UnixMilli(),
	}
	mrsPrev := testGenerateMetricRowsWithPrefix(rng, numMetrics, "legacy_prev", trPrev)
	mrsCurr := testGenerateMetricRowsWithPrefix(rng, numMetrics, "legacy_curr", trCurr)
	mrsPt := testGenerateMetricRowsWithPrefix(rng, numMetrics, "pt", trPt)

	f := func(idb *indexDB, tr TimeRange, want bool) {
		t.Helper()
		is := idb.getIndexSearch(noDeadline)
		defer idb.putIndexSearch(is)

		got := is.legacyContainsTimeRange(tr)

		if got != want {
			t.Fatalf("legacyContainsTimeRange(%s) for index db %s returns unexpected result: got %t, want %t", tr.String(), idb.name, got, want)
		}
	}

	// fill legacy index with data
	s := MustOpenStorage(t.Name(), OpenOptions{})
	s.AddRows(mrsPrev, defaultPrecisionBits)
	s.DebugFlush()
	s = mustConvertToLegacy(s)
	s.AddRows(mrsCurr, defaultPrecisionBits)
	s.DebugFlush()
	s = mustConvertToLegacy(s)
	// fill partitioned index with data
	s.AddRows(mrsPt, defaultPrecisionBits)
	s.DebugFlush()
	defer s.MustClose()

	legacyIDBs := s.getLegacyIndexDBs()
	defer s.putLegacyIndexDBs(legacyIDBs)

	ptws := s.tb.GetPartitions(trPt)
	defer s.tb.PutPartitions(ptws)
	if len(ptws) != 1 {
		t.Fatalf("unexpected number of partitions for one month time range %v: got %d, want 1", &trPt, len(ptws))
	}
	idb := ptws[0].pt.idb

	var tr TimeRange

	// Global index time range.
	tr = globalIndexTimeRange
	f(legacyIDBs.getIDBPrev(), tr, true)
	f(legacyIDBs.getIDBCurr(), tr, true)
	f(idb, tr, true)

	// Fully before trPrev, trCurr, and trPt.
	tr = TimeRange{
		MinTimestamp: time.Date(2024, 12, 1, 0, 0, 0, 0, time.UTC).UnixMilli(),
		MaxTimestamp: time.Date(2024, 12, 31, 23, 59, 59, 999_999_999, time.UTC).UnixMilli(),
	}
	f(legacyIDBs.getIDBPrev(), tr, true)
	f(legacyIDBs.getIDBCurr(), tr, true)
	f(idb, tr, true)

	// Overlaps with trPrev and trPt on the left side, fully before trCurr.
	tr = TimeRange{
		MinTimestamp: time.Date(2024, 12, 1, 0, 0, 0, 0, time.UTC).UnixMilli(),
		MaxTimestamp: time.Date(2025, 1, 7, 23, 59, 59, 999_999_999, time.UTC).UnixMilli(),
	}
	f(legacyIDBs.getIDBPrev(), tr, true)
	f(legacyIDBs.getIDBCurr(), tr, true)
	f(idb, tr, true)

	// Fully inside trPrev and trPt, fully before trCurr.
	tr = TimeRange{
		MinTimestamp: time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC).UnixMilli(),
		MaxTimestamp: time.Date(2025, 1, 7, 23, 59, 59, 999_999_999, time.UTC).UnixMilli(),
	}
	f(legacyIDBs.getIDBPrev(), tr, true)
	f(legacyIDBs.getIDBCurr(), tr, true)
	f(idb, tr, true)

	// Fully inside trPt, overlaps with trPrev on the right side and trCurr on
	// the left side.
	tr = TimeRange{
		MinTimestamp: time.Date(2025, 1, 7, 0, 0, 0, 0, time.UTC).UnixMilli(),
		MaxTimestamp: time.Date(2025, 1, 21, 23, 59, 59, 999_999_999, time.UTC).UnixMilli(),
	}
	f(legacyIDBs.getIDBPrev(), tr, true)
	f(legacyIDBs.getIDBCurr(), tr, true)
	f(idb, tr, true)

	// Fully inside trPt and trCurr, fully after trPrev.
	tr = TimeRange{
		MinTimestamp: time.Date(2025, 1, 18, 0, 0, 0, 0, time.UTC).UnixMilli(),
		MaxTimestamp: time.Date(2025, 1, 21, 23, 59, 59, 999_999_999, time.UTC).UnixMilli(),
	}
	f(legacyIDBs.getIDBPrev(), tr, false)
	f(legacyIDBs.getIDBCurr(), tr, true)
	f(idb, tr, true)

	// Overlaps with trPt and trCurr on the right side, fully after trPrev.
	tr = TimeRange{
		MinTimestamp: time.Date(2025, 1, 21, 0, 0, 0, 0, time.UTC).UnixMilli(),
		MaxTimestamp: time.Date(2025, 2, 21, 23, 59, 59, 999_999_999, time.UTC).UnixMilli(),
	}
	f(legacyIDBs.getIDBPrev(), tr, false)
	f(legacyIDBs.getIDBCurr(), tr, true)
	f(idb, tr, true)

	// fully after trPrev, trCurr, and trPt.
	tr = TimeRange{
		MinTimestamp: time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC).UnixMilli(),
		MaxTimestamp: time.Date(2025, 3, 31, 23, 59, 59, 999_999_999, time.UTC).UnixMilli(),
	}
	f(legacyIDBs.getIDBPrev(), tr, false)
	f(legacyIDBs.getIDBCurr(), tr, false)
	f(idb, tr, true)
}
