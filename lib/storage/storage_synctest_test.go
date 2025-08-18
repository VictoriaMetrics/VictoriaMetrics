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
		ptw := s.tb.MustGetPartition(tr.MinTimestamp)
		idb := ptw.pt.idb
		defer s.tb.PutPartition(ptw)
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
		if diff := cmp.Diff([]uint64(nil), searchMetricIDs()); diff != "" {
			t.Fatalf("unexpected metricIDs (-want, +got):\n%s", diff)
		}
	})
}

func TestStorageRotateIndexDBPrefill(t *testing.T) {
	defer testRemoveAll(t)
	f := func(t *testing.T, opts OpenOptions, prefillStart time.Duration) {
		t.Helper()

		synctest.Run(func() {
			// Prefill of the next partition indexDB happens during the
			// (nextMonth-prefillStart, nextMonth] time interval.
			// Advance current time right before the the beginning of that interval.
			ct := time.Now().UTC()
			nextMonth := time.Date(ct.Year(), ct.Month()+1, 1, 0, 0, 0, 0, time.UTC)
			time.Sleep(nextMonth.Sub(ct.Add(prefillStart)))

			s := MustOpenStorage(t.Name(), opts)
			defer s.MustClose()

			const numSeries = 1000
			addRows := func() {
				t.Helper()
				rng := rand.New(rand.NewSource(1))
				ct := time.Now().UTC()
				tr := TimeRange{
					MinTimestamp: ct.Add(-prefillStart).UnixMilli(),
					MaxTimestamp: ct.UnixMilli(),
				}
				mrs := testGenerateMetricRowsWithPrefix(rng, numSeries, "metric.", tr)
				s.AddRows(mrs, 1)
				s.DebugFlush()
			}

			// Insert metrics into the empty storage right before the prefill
			// interval starts.
			addRows()
			if got, want := s.newTimeseriesCreated.Load(), uint64(numSeries); got != want {
				t.Fatalf("unexpected number of new timeseries: got %d, want %d", got, want)
			}
			if got, want := s.timeseriesPreCreated.Load(), uint64(0); got != want {
				t.Fatalf("unexpected number of pre-created timeseries: got %d, want %d", got, want)
			}

			// Sleep until half of the prefill interval has elapsed,
			// then verify that some time series have been pre-created.
			time.Sleep(prefillStart / 2)
			addRows()
			if got, want := s.timeseriesPreCreated.Load(), uint64(0); got <= want {
				t.Fatalf("unexpected number of pre-created timeseries: got %d, want > %d", got, want)
			}

			// Sleep until a minute before the next partition transition, verify
			// that almost all time series have been pre-created.
			ct = time.Now().UTC()
			time.Sleep(nextMonth.Sub(ct.Add(time.Minute)))
			addRows()
			if got, want := s.timeseriesPreCreated.Load(), uint64(numSeries/2); got <= want {
				t.Fatalf("unexpected number of pre-created timeseries: got %d, want > %d", got, want)
			}

			// Align the time with the start of the next month.
			time.Sleep(time.Minute)
			// Sleep until the transition to the next partition is over, verify
			// that the rest of time series have been re-created
			time.Sleep(prefillStart)
			newCreated := s.newTimeseriesCreated.Load()
			addRows()
			newCreated = s.newTimeseriesCreated.Load() - newCreated
			// If jump in time is bigger than 1h, the tsidCache will be cleared
			// and therefore the metrics will not be repopulated. Instead, new
			// metrics will be created.
			preCreated, repopulated := s.timeseriesPreCreated.Load(), s.timeseriesRepopulated.Load()
			if preCreated+repopulated+newCreated != numSeries {
				t.Fatalf("unexpected number of pre-populated, repopulated, and new timeseries: got %d + %d + %d, want %d", preCreated, repopulated, newCreated, numSeries)
			}
		})
	}

	// Verify an interval that is shorter than one hour.
	t.Run("30m", func(t *testing.T) {
		f(t, OpenOptions{IDBPrefillStart: 30 * time.Minute}, 30*time.Minute)
	})
	// Verify 1h inteval (which is also the default).
	// tsidCache will be cleared because it will have two cache rotations (one
	// every 30 mins). This means that once the new month starts the timeseries
	// that waren't pre-populated will be re-created instead of being
	// re-populated.
	t.Run("default", func(t *testing.T) {
		f(t, OpenOptions{IDBPrefillStart: 0}, time.Hour)
	})
	t.Run("1h", func(t *testing.T) {
		f(t, OpenOptions{IDBPrefillStart: time.Hour}, time.Hour)
	})
	// Vefiry 2h interval. Same here, the tsidCache will be cleared.
	t.Run("2h", func(t *testing.T) {
		f(t, OpenOptions{IDBPrefillStart: 2 * time.Hour}, 2*time.Hour)
	})
}
