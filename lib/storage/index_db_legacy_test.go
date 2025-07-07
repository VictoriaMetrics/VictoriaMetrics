package storage

import (
	"math/rand"
	"testing"
	"time"
)

func TestLegacyContainsTimeRange(t *testing.T) {
	defer testRemoveAll(t)

	const (
		accountID  = 12
		projectID  = 34
		numMetrics = 10000
	)
	tr := TimeRange{
		MinTimestamp: time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli(),
		MaxTimestamp: time.Date(2025, 1, 31, 23, 59, 59, 999_999_999, time.UTC).UnixMilli(),
	}
	rng := rand.New(rand.NewSource(1))
	data := testGenerateMetricRowsWithPrefixForTenantID(rng, accountID, projectID, numMetrics, "metric", tr)

	f := func(idb *indexDB, tr TimeRange, want bool) {
		t.Helper()
		is := idb.getIndexSearch(accountID, projectID, noDeadline)
		defer idb.putIndexSearch(is)

		got := is.legacyContainsTimeRange(tr)

		if got != want {
			t.Fatalf("legacyContainsTimeRange(%s) for index db %s returns unexpected result: got %t, want %t", tr.String(), idb.name, got, want)
		}
	}

	// fill legacy index with data
	s := MustOpenStorage(t.Name(), OpenOptions{})
	s.AddRows(data, defaultPrecisionBits)
	s.DebugFlush()
	s.MustClose()
	testStorageConvertToLegacy(t, accountID, projectID)

	// fill partitioned index with data
	s = MustOpenStorage(t.Name(), OpenOptions{})
	s.AddRows(data, defaultPrecisionBits)
	s.DebugFlush()
	defer s.MustClose()

	legacyIDBPrev, legacyIDBCurr := s.getLegacyIndexDBs()
	defer s.putLegacyIndexDBs(legacyIDBPrev, legacyIDBCurr)

	idbs := s.tb.GetIndexDBs(tr)
	defer s.tb.PutIndexDBs(idbs)
	if len(idbs) != 1 {
		t.Fatalf("unexpected number of indexDBs for one month time range %s: got %d, want 1", tr.String(), len(idbs))
	}
	idb := idbs[0]

	// fully before tr
	tr1 := TimeRange{
		MinTimestamp: time.Date(2024, 4, 1, 0, 0, 0, 0, time.UTC).UnixMilli(),
		MaxTimestamp: time.Date(2024, 10, 31, 23, 59, 59, 999_999_999, time.UTC).UnixMilli(),
	}
	// overlapping with tr from the left
	tr2 := TimeRange{
		MinTimestamp: time.Date(2024, 12, 1, 0, 0, 0, 0, time.UTC).UnixMilli(),
		MaxTimestamp: time.Date(2025, 1, 15, 23, 59, 59, 999_999_999, time.UTC).UnixMilli(),
	}
	// fully inside tr
	tr3 := TimeRange{
		MinTimestamp: time.Date(2025, 1, 7, 0, 0, 0, 0, time.UTC).UnixMilli(),
		MaxTimestamp: time.Date(2023, 1, 21, 23, 59, 59, 999_999_999, time.UTC).UnixMilli(),
	}
	// overlapping with tr from the right
	tr4 := TimeRange{
		MinTimestamp: time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC).UnixMilli(),
		MaxTimestamp: time.Date(2025, 2, 15, 23, 59, 59, 999_999_999, time.UTC).UnixMilli(),
	}
	// fully after tr
	tr5 := TimeRange{
		MinTimestamp: time.Date(2025, 2, 1, 0, 0, 0, 0, time.UTC).UnixMilli(),
		MaxTimestamp: time.Date(2025, 3, 31, 23, 59, 59, 999_999_999, time.UTC).UnixMilli(),
	}

	// legacy previous indexDB has no data
	f(legacyIDBPrev, globalIndexTimeRange, true)
	f(legacyIDBPrev, tr1, false)
	f(legacyIDBPrev, tr2, false)
	f(legacyIDBPrev, tr3, false)
	f(legacyIDBPrev, tr4, false)
	f(legacyIDBPrev, tr5, false)

	// legacy current indexDB has some data
	f(legacyIDBCurr, globalIndexTimeRange, true)
	f(legacyIDBCurr, tr1, true)
	f(legacyIDBCurr, tr2, true)
	f(legacyIDBCurr, tr3, true)
	f(legacyIDBCurr, tr4, true)
	f(legacyIDBCurr, tr5, false)

	// partitioned indexDB return true for any time range
	f(idb, globalIndexTimeRange, true)
	f(idb, tr1, true)
	f(idb, tr2, true)
	f(idb, tr3, true)
	f(idb, tr4, true)
	f(idb, tr5, true)
}
