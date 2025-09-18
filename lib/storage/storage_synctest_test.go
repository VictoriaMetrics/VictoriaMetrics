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

	synctest.Test(t, func(t *testing.T) {
		s := MustOpenStorage(t.Name(), OpenOptions{})
		defer s.MustClose()

		now := time.Now().UTC()
		tr := TimeRange{
			MinTimestamp: time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC).UnixMilli(),
			MaxTimestamp: time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 999_999_999, time.UTC).UnixMilli(),
		}
		const numMetrics = 10
		date := uint64(tr.MinTimestamp) / msecPerDay
		idbPrev, idbCurr := s.getPrevAndCurrIndexDBs()
		defer s.putPrevAndCurrIndexDBs(idbPrev, idbCurr)
		var wantMetricIDs []uint64

		// Simulate corrupted index by inserting `(date, tag) -> metricID`
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

			idbCurr.tb.AddItems(ii.Items)

			putIndexItems(ii)
		}
		idbCurr.tb.DebugFlush()

		tfsAll := NewTagFilters()
		if err := tfsAll.Add([]byte("__name__"), []byte(".*"), false, true); err != nil {
			panic(fmt.Sprintf("unexpected error in TagFilters.Add: %v", err))
		}
		tfssAll := []*TagFilters{tfsAll}

		searchMetricIDs := func() []uint64 {
			metricIDs, err := idbCurr.searchMetricIDs(nil, tfssAll, tr, 1e9, noDeadline)
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
		if diff := cmp.Diff([]uint64(nil), searchMetricIDs()); diff != "" {
			t.Fatalf("unexpected metricIDs (-want, +got):\n%s", diff)
		}
	})
}

func TestStorageRotateIndexDBPrefill(t *testing.T) {
	f := func(opts OpenOptions, prefillStart time.Duration) {
		defer testRemoveAll(t)
		t.Helper()

		synctest.Test(t, func(t *testing.T) {
			// Align start time to 05:00 in order to have 23h before the next rotation cycle at 04:00 next morning.
			time.Sleep(time.Hour * 5)

			nextRotationTime := time.Now().Add(time.Hour * 23).Truncate(time.Hour)

			s := MustOpenStorage(t.Name(), opts)
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

			// Sleep until a minute before the prefill start time,
			// then verify that no timeseries have been pre-created yet.
			time.Sleep(time.Hour*23 - prefillStart - 1*time.Minute)
			s.AddRows(mrs, 1)
			s.DebugFlush()
			preCreated := s.timeseriesPreCreated.Load()
			if preCreated != 0 {
				t.Fatalf("expected no timeseries to be re-created, got: %d", preCreated)
			}

			// Sleep until half of the prefill rotation interval has elapsed,
			// then verify that some time series have been pre-created.
			time.Sleep(prefillStart / 2)
			s.AddRows(mrs, 1)
			s.DebugFlush()
			preCreated = s.timeseriesPreCreated.Load()
			if preCreated == 0 {
				t.Fatalf("expected some timeseries to be re-created, got: %d", preCreated)
			}

			// Sleep until a minute before the index rotation,
			// verify that almost all time series have been pre-created.
			time.Sleep(nextRotationTime.Sub(time.Now().Add(time.Minute)))
			s.AddRows(mrs, 1)
			s.DebugFlush()
			preCreated = s.timeseriesPreCreated.Load()
			if preCreated == 0 || preCreated < numSeries/2 {
				t.Fatalf("expected more than 50 percent of timeseries to be re-created, got: %d", preCreated)
			}

			// Sleep until the rotation is over, verify that the rest of time series have been re-created
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

	// Test the default prefill start duration, see -storage.idbPrefillStart flag:
	// VictoriaMetrics starts prefill indexDB at 3 A.M UTC, while indexDB rotates at 4 A.M UTC.
	f(
		OpenOptions{Retention: time.Hour * 24, IDBPrefillStart: time.Hour},
		time.Hour,
	)

	// Zero IDBPrefillStart option should fallback to 1 hour prefill start:
	f(
		OpenOptions{Retention: time.Hour * 24, IDBPrefillStart: 0},
		time.Hour,
	)

	// Test a custom prefill duration: 2h:
	// VictoriaMetrics starts prefill indexDB at 2 A.M UTC, while indexDB rotates at 4 A.M UTC.
	f(
		OpenOptions{Retention: time.Hour * 24, IDBPrefillStart: 2 * time.Hour},
		2*time.Hour,
	)
}
