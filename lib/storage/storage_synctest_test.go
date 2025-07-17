//go:build goexperiment.synctest

package storage

import (
	"fmt"
	"math/rand"
	"testing"
	"testing/synctest"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/google/go-cmp/cmp"
)

func TestStorageSearchMetricNames_CorruptedIndex(t *testing.T) {
	defer testRemoveAll(t)

	synctest.Run(func() {
		s := MustOpenStorage(t.Name(), OpenOptions{})
		defer s.MustClose()

		now := time.Now().UTC()
		tr := TimeRange{
			MinTimestamp: time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC).UnixMilli(),
			MaxTimestamp: time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 999_999_999, time.UTC).UnixMilli(),
		}
		const numMetrics = 10
		date := uint64(tr.MinTimestamp) / msecPerDay
		idb, putCurrIndexDB := s.getCurrIndexDB()
		defer putCurrIndexDB()
		var wantMetricIDs []uint64

		// Symulate corrupted index by inserting `(date, tag) -> metricID`
		// entries only.
		for i := range numMetrics {
			metricName := []byte(fmt.Sprintf("metric_%d", i))
			metricID := generateUniqueMetricID()
			wantMetricIDs = append(wantMetricIDs, metricID)

			ii := getIndexItems()

			// Create per-day tag -> metricID entries for every tag in mn.
			kb := kbPool.Get()
			kb.B = marshalCommonPrefix(kb.B[:0], nsPrefixDateTagToMetricIDs)
			kb.B = encoding.MarshalUint64(kb.B, date)
			ii.B = append(ii.B, kb.B...)
			ii.B = marshalTagValue(ii.B, nil)
			ii.B = marshalTagValue(ii.B, metricName)
			ii.B = encoding.MarshalUint64(ii.B, metricID)
			ii.Next()
			kbPool.Put(kb)

			idb.tb.AddItems(ii.Items)

			putIndexItems(ii)
		}
		idb.tb.DebugFlush()

		tfsAll := NewTagFilters()
		if err := tfsAll.Add([]byte("__name__"), []byte(".*"), false, true); err != nil {
			panic(fmt.Sprintf("unexpected error in TagFilters.Add: %v", err))
		}
		tfssAll := []*TagFilters{tfsAll}

		searchMetricIDs := func() []uint64 {
			metricIDs, err := idb.searchMetricIDs(nil, tfssAll, tr, 1e9, noDeadline)
			if err != nil {
				panic(fmt.Sprintf("searchMetricIDs() failed unexpectedly: %v", err))
			}
			return metricIDs
		}
		searchMetricNames := func() []string {
			metricNames, err := s.SearchMetricNames(nil, tfssAll, tr, 1e9, noDeadline)
			if err != nil {
				panic(fmt.Sprintf("SearchMetricNames() failed unexpectedly: %v", err))
			}
			return metricNames
		}

		// Ensure that metricIDs can be searched.
		if diff := cmp.Diff(wantMetricIDs, searchMetricIDs()); diff != "" {
			t.Fatalf("unexpected metricIDs (-want, +got):\n%s", diff)
		}
		// Ensure that Storage.SearchMetricNames() returns empty result.
		// The corrupted index lets to find metricIDs by tag (`__name__` tag in
		// our case) but it lacks metricID->metricName mapping and hence the
		// empty search result.
		// The code detects this and puts such metricIDs into a special cache.
		if diff := cmp.Diff([]string{}, searchMetricNames()); diff != "" {
			t.Fatalf("unexpected metric names (-want, +got):\n%s", diff)
		}
		// Ensure that the metricIDs still can be searched.
		if diff := cmp.Diff(wantMetricIDs, searchMetricIDs()); diff != "" {
			t.Fatalf("unexpected metricIDs (-want, +got):\n%s", diff)
		}

		time.Sleep(61 * time.Second)
		synctest.Wait()

		// If the same search is repeated after 1 minute, the metricIDs are
		// marked as deleted.
		if diff := cmp.Diff([]string{}, searchMetricNames()); diff != "" {
			t.Fatalf("unexpected metric names (-want, +got):\n%s", diff)
		}
		// As a result they cannot be searched anymore.
		if diff := cmp.Diff([]uint64{}, searchMetricIDs()); diff != "" {
			t.Fatalf("unexpected metricIDs (-want, +got):\n%s", diff)
		}
	})
}

func TestRotateIndexDBPrefill(t *testing.T) {
	defer testRemoveAll(t)

	synctest.Run(func() {
		// allign time to 05:00 in order to properly start rotation cycle
		time.Sleep(time.Hour * 5)
		s := MustOpenStorage(t.Name(), OpenOptions{Retention: time.Hour * 24, IDBPrefillStart: time.Hour * 2})
		defer s.MustClose()
		// first rotation cycle in 4 hours due to synctest start time of 00:00:00
		rng := rand.New(rand.NewSource(1))
		ct := time.Now()
		tr := TimeRange{
			MinTimestamp: ct.Add(time.Hour).UnixMilli(),
			MaxTimestamp: ct.Add(time.Hour * 24).UnixMilli(),
		}
		const numSeries = 1000

		mrs := testGenerateMetricRowsWithPrefix(rng, numSeries, "metric.", tr)
		s.AddRows(mrs, 1)
		s.DebugFlush()
		createdSeries := s.newTimeseriesCreated.Load()
		if createdSeries != numSeries {
			t.Fatalf("unexpected number of created series (-%d;+%d)", numSeries, createdSeries)
		}
		// wait for half time before rotation
		// rotation must happen in 28 hours
		time.Sleep(time.Hour * 22)
		s.AddRows(mrs, 1)
		s.DebugFlush()

		preCreated := s.timeseriesPreCreated.Load()
		if preCreated == 0 {
			t.Fatalf("expected some timeseries to be re-created, got: %d", preCreated)
		}

		// wait for the last minute before rotation
		// almost all series must be re-created
		time.Sleep(time.Minute * 59)
		s.AddRows(mrs, 1)
		s.DebugFlush()
		preCreated = s.timeseriesPreCreated.Load()
		if preCreated == 0 || preCreated < numSeries/2 {
			t.Fatalf("expected more than 50 percent of timeseries to be re-created, got: %d", preCreated)
		}
		// wait for rotation to happen
		// rest series must be re-populated
		time.Sleep(time.Hour)
		s.AddRows(mrs, 1)
		s.DebugFlush()
		createdSeries, reCreated, rePopulated := s.newTimeseriesCreated.Load(), s.timeseriesPreCreated.Load(), s.timeseriesRepopulated.Load()
		if createdSeries != numSeries {
			t.Fatalf("unexpected number of created series (-%d;+%d)", numSeries, createdSeries)
		}
		if reCreated+rePopulated != numSeries {
			t.Fatalf("unexpected number of re-created=%d and re-populated=%d series, want sum to be equal to %d", numSeries, createdSeries, numSeries)
		}
	})
}
