//go:build synctest

package storage

import (
	"fmt"
	"math/rand"
	"path/filepath"
	"slices"
	"testing"
	"testing/synctest"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
	"github.com/google/go-cmp/cmp"
)

func TestStorageSearchTSIDs_CorruptedIndex(t *testing.T) {
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
		ptw := s.tb.MustGetPartition(tr.MinTimestamp)
		idb := ptw.pt.idb
		defer s.tb.PutPartition(ptw)
		var wantMetricIDs []uint64

		// Simulate corrupted index by not creating nsPrefixMetricIDToTSID
		// index entries.
		for i := range numMetrics {
			mn := MetricName{
				MetricGroup: []byte(fmt.Sprintf("metric_%d", i)),
			}
			var tsid TSID
			generateTSID(&tsid, &mn)
			wantMetricIDs = append(wantMetricIDs, tsid.MetricID)
			ii := testCreateIndexItems(date, &tsid, &mn, testIndexItemOpts{
				skipMetricIDToTSID: true,
			})

			idb.tb.AddItems(ii.Items)
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
			return metricIDs.AppendTo(nil)
		}
		searchTSIDs := func() []TSID {
			tsids, err := s.SearchTSIDs(nil, tfssAll, tr, 1e9, noDeadline)
			if err != nil {
				panic(fmt.Sprintf("SearchTSIDs() failed unexpectedly: %v", err))
			}
			return tsids
		}

		// Ensure that metricIDs can be searched.
		if diff := cmp.Diff(wantMetricIDs, searchMetricIDs()); diff != "" {
			t.Fatalf("unexpected metricIDs (-want, +got):\n%s", diff)
		}
		// Ensure that Storage.SearchTSIDs() returns empty result.
		// The corrupted index lets to find metricIDs by tag (`__name__` tag in
		// our case) but it lacks metricID->TSID mapping and hence the
		// empty search result.
		// The code detects this and puts such metricIDs into a special cache.
		if diff := cmp.Diff([]TSID(nil), searchTSIDs()); diff != "" {
			t.Fatalf("unexpected TSIDs (-want, +got):\n%s", diff)
		}
		// Ensure that the metricIDs still can be searched.
		if diff := cmp.Diff(wantMetricIDs, searchMetricIDs()); diff != "" {
			t.Fatalf("unexpected metricIDs (-want, +got):\n%s", diff)
		}
		// Ensure the metric that counts metricIDs for which no TSIDs were found
		// is not incremented yet.
		var m Metrics
		s.UpdateMetrics(&m)
		if got, want := m.TableMetrics.IndexDBMetrics.MissingTSIDsForMetricID, uint64(0); got != want {
			t.Fatalf("unexpected MissingTSIDsForMetricID: got %d, want %d", got, want)
		}

		time.Sleep(61 * time.Second)
		synctest.Wait()

		// If the same search is repeated after 1 minute, the metricIDs are
		// marked as deleted.
		if diff := cmp.Diff([]TSID(nil), searchTSIDs()); diff != "" {
			t.Fatalf("unexpected metric names (-want, +got):\n%s", diff)
		}
		// As a result they cannot be searched anymore.
		if diff := cmp.Diff([]uint64(nil), searchMetricIDs()); diff != "" {
			t.Fatalf("unexpected metricIDs (-want, +got):\n%s", diff)
		}
		// Ensure the metric that counts metricIDs for which no TSIDs were found
		// is incremented after the metricID deletion.
		s.UpdateMetrics(&m)
		if got, want := m.TableMetrics.IndexDBMetrics.MissingTSIDsForMetricID, uint64(numMetrics); got != want {
			t.Fatalf("unexpected MissingTSIDsForMetricID: got %d, want %d", got, want)
		}
	})
}

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
		ptw := s.tb.MustGetPartition(tr.MinTimestamp)
		idb := ptw.pt.idb
		defer s.tb.PutPartition(ptw)
		var wantMetricIDs []uint64

		// Simulate corrupted index by not creating nsPrefixMetricIDToMetricName
		// index entries.
		for i := range numMetrics {
			mn := MetricName{
				MetricGroup: []byte(fmt.Sprintf("metric_%d", i)),
			}
			var tsid TSID
			generateTSID(&tsid, &mn)
			wantMetricIDs = append(wantMetricIDs, tsid.MetricID)
			ii := testCreateIndexItems(date, &tsid, &mn, testIndexItemOpts{
				skipMetricIDToMetricName: true,
			})

			idb.tb.AddItems(ii.Items)
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
			return metricIDs.AppendTo(nil)
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
		// Ensure the metric that counts metricIDs for which no metric names
		// were found is not incremented yet.
		var m Metrics
		s.UpdateMetrics(&m)
		if got, want := m.TableMetrics.IndexDBMetrics.MissingMetricNamesForMetricID, uint64(0); got != want {
			t.Fatalf("unexpected MissingMetricNamesForMetricID: got %d, want %d", got, want)
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
		// Ensure the metric that counts metricIDs for which no metric names
		// were found is incremented after the metricID deletion.
		s.UpdateMetrics(&m)
		if got, want := m.TableMetrics.IndexDBMetrics.MissingMetricNamesForMetricID, uint64(numMetrics); got != want {
			t.Fatalf("unexpected MissingMetricNamesForMetricID: got %d, want %d", got, want)
		}
	})
}

type testIndexItemOpts struct {
	skipMetricIDToMetricName bool
	skipMetricIDToTSID       bool
	skipTagToMetricIDs       bool
	skipDateToMetricID       bool
	skipDateMetricNameToTSID bool
	skipDateTagToMetricIDs   bool
}

func testCreateIndexItems(date uint64, tsid *TSID, mn *MetricName, opts testIndexItemOpts) *indexItems {
	var ii indexItems

	if !opts.skipMetricIDToMetricName {
		// Create metricID -> metricName entry.
		ii.B = marshalCommonPrefix(ii.B, nsPrefixMetricIDToMetricName)
		ii.B = encoding.MarshalUint64(ii.B, tsid.MetricID)
		ii.B = mn.Marshal(ii.B)
		ii.Next()
	}

	if !opts.skipMetricIDToTSID {
		// Create metricID -> TSID entry.
		ii.B = marshalCommonPrefix(ii.B, nsPrefixMetricIDToTSID)
		ii.B = encoding.MarshalUint64(ii.B, tsid.MetricID)
		ii.B = tsid.Marshal(ii.B)
		ii.Next()
	}

	if !opts.skipTagToMetricIDs {
		// Create tag -> metricID entries for every tag in mn.
		kb := kbPool.Get()
		kb.B = marshalCommonPrefix(kb.B[:0], nsPrefixTagToMetricIDs)
		ii.registerTagIndexes(kb.B, mn, tsid.MetricID)
		ii.Next()
		kbPool.Put(kb)
	}

	if !opts.skipDateToMetricID {
		// Create date -> metricID entry.
		ii.B = marshalCommonPrefix(ii.B, nsPrefixDateToMetricID)
		ii.B = encoding.MarshalUint64(ii.B, date)
		ii.B = encoding.MarshalUint64(ii.B, tsid.MetricID)
		ii.Next()
	}

	if !opts.skipDateMetricNameToTSID {
		// Create metricName -> TSID entry.
		ii.B = marshalCommonPrefix(ii.B, nsPrefixDateMetricNameToTSID)
		ii.B = encoding.MarshalUint64(ii.B, date)
		ii.B = mn.Marshal(ii.B)
		ii.B = append(ii.B, kvSeparatorChar)
		ii.B = tsid.Marshal(ii.B)
		ii.Next()
	}

	if !opts.skipDateTagToMetricIDs {
		// Create per-day tag -> metricID entries for every tag in mn.
		kb := kbPool.Get()
		kb.B = marshalCommonPrefix(kb.B[:0], nsPrefixDateTagToMetricIDs)
		kb.B = encoding.MarshalUint64(kb.B, date)
		ii.registerTagIndexes(kb.B, mn, tsid.MetricID)
		kbPool.Put(kb)
	}

	return &ii
}

func TestStorageRotateIndexDBPrefill(t *testing.T) {
	defer testRemoveAll(t)
	f := func(t *testing.T, opts OpenOptions, prefillStart time.Duration) {
		t.Helper()

		synctest.Test(t, func(t *testing.T) {
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

// TestStorageAddRows_nextDayIndexPrefill tests gradual creation of per-day
// index entries of the next day during the last hour of the current day. This
// is an optimization that mitigates the ingestion slowdown at the beginning of
// a day.
//
// Problem: as the new day begins, indexDB suddenly lacks ALL the per-day
// entries for this new day and they need to be created. If the number of active
// timeseries is high enough, this may cause serious degradation of the data
// ingestion performance.
//
// Solution: start creating the next day entries during the last hour of the
// current day. In order to avoid the same slowdown, the entries are created
// gradually. I.e. at the beginning of the last hour, a very small fraction of
// next day index entries (compared to the total number of active timeseries)
// will be created. As current time gets closer and closer to midnight, the
// fraction becomes bigger. This does not mean that the number of next day index
// entries to create becomes proportionately bigger since some of them have
// already been added earlier that hour. Finally, only active timeseries are
// registered in the next day index. An active timeseries is one for which at
// least one sample have been received during the current hour and the timestamp
// of that sample must also fall within the current hour.
func TestStorageAddRows_nextDayIndexPrefill(t *testing.T) {
	defer testRemoveAll(t)

	countMetricIDs := func(t *testing.T, s *Storage, prefix string, tr TimeRange) int {
		t.Helper()

		tfs := NewTagFilters()
		if err := tfs.Add(nil, []byte(prefix+".*"), false, true); err != nil {
			t.Fatalf("unexpected error in TagFilters.Add: %v", err)
		}
		ids := testSearchMetricIDs(s, []*TagFilters{tfs}, tr, 1e9, noDeadline)
		return len(ids)
	}

	synctest.Test(t, func(t *testing.T) {
		// synctest starts at 2000-01-01T00:00:00Z.

		today := TimeRange{
			MinTimestamp: time.Now().UnixMilli(),
			MaxTimestamp: time.Now().UnixMilli() + msecPerDay - 1,
		}
		nextDay := TimeRange{
			MinTimestamp: today.MinTimestamp + msecPerDay,
			MaxTimestamp: today.MaxTimestamp + msecPerDay,
		}

		const numSeries = 1000
		rng := rand.New(rand.NewSource(1))

		// Verify that prefill hasn't started yet.
		// The prefill happens during the last hour of a day. At exactly
		// 23:00:00, however, it must not start yet.
		//
		// Advance the time 1m before the last hour.
		time.Sleep(23*time.Hour - 1*time.Minute) // 2000-01-01T22:59:00Z
		mrs0 := testGenerateMetricRowsWithPrefix(rng, numSeries, "metric0", TimeRange{
			MinTimestamp: time.Now().Add(-15 * time.Minute).UnixMilli(),
			MaxTimestamp: time.Now().Add(+15 * time.Minute).UnixMilli(),
		})
		s := MustOpenStorage(t.Name(), OpenOptions{})
		defer s.MustClose()
		s.AddRows(mrs0, defaultPrecisionBits)
		s.DebugFlush()
		if got, want := countMetricIDs(t, s, "metric0", today), numSeries; got != want {
			t.Fatalf("unexpected metric id count for today: got %d, want %d", got, want)
		}
		if got, want := countMetricIDs(t, s, "metric0", nextDay), 0; got != want {
			t.Fatalf("unexpected metric id count for next day: got %d, want %d", got, want)
		}
		// Again, at 23:00:00 the prefill must not start yet even for timestamps
		// beyond that time.
		time.Sleep(1 * time.Minute) // 2000-01-01T23:00:00Z
		synctest.Wait()
		s.AddRows(mrs0, defaultPrecisionBits)
		s.DebugFlush()
		if got, want := countMetricIDs(t, s, "metric0", today), numSeries; got != want {
			t.Fatalf("unexpected metric id count for today: got %d, want %d", got, want)
		}
		if got, want := countMetricIDs(t, s, "metric0", nextDay), 0; got != want {
			t.Fatalf("unexpected metric id count for next day: got %d, want %d", got, want)
		}

		// At 23:15 the prefill must work.
		//
		// However, the mrs1 timestamps are not within the current hour and
		// therefore the next day will not be prefilled with the corresponding
		// timeseries.
		//
		// The mrs2 timestamps are within the current hour so some next day index
		// entries will be created.
		time.Sleep(15 * time.Minute) // 2000-01-01T23:15:00Z
		mrs1 := testGenerateMetricRowsWithPrefix(rng, numSeries, "metric1", TimeRange{
			MinTimestamp: time.Now().Add(-30 * time.Minute).UnixMilli(),
			MaxTimestamp: time.Now().Add(-15 * time.Minute).UnixMilli(),
		})
		mrs2 := testGenerateMetricRowsWithPrefix(rng, numSeries, "metric2", TimeRange{
			MinTimestamp: time.Now().Add(-15 * time.Minute).UnixMilli(),
			MaxTimestamp: time.Now().UnixMilli(),
		})
		s.AddRows(mrs1, defaultPrecisionBits)
		s.AddRows(mrs2, defaultPrecisionBits)
		s.DebugFlush()
		if got, want := countMetricIDs(t, s, "metric1", today), numSeries; got != want {
			t.Fatalf("unexpected metric id count for today: got %d, want %d", got, want)
		}
		if got, want := countMetricIDs(t, s, "metric1", nextDay), 0; got != want {
			t.Fatalf("unexpected metric id count for next day: got %d, want %d", got, want)
		}
		if got, want := countMetricIDs(t, s, "metric2", today), numSeries; got != want {
			t.Fatalf("unexpected metric id count for today: got %d, want %d", got, want)
		}
		got15min := countMetricIDs(t, s, "metric2", nextDay)
		if got15min == 0 {
			t.Fatalf("unexpected metric id count for next day: got 0, want > 0")
		}

		time.Sleep(15 * time.Minute) // 2000-01-01T23:30:00Z
		mrs3 := testGenerateMetricRowsWithPrefix(rng, numSeries, "metric3", TimeRange{
			MinTimestamp: time.Now().Add(-15 * time.Minute).UnixMilli(),
			MaxTimestamp: time.Now().UnixMilli(),
		})
		s.AddRows(mrs3, defaultPrecisionBits)
		s.DebugFlush()
		if got, want := countMetricIDs(t, s, "metric3", today), numSeries; got != want {
			t.Fatalf("unexpected metric id count for today: got %d, want %d", got, want)
		}
		got30min := countMetricIDs(t, s, "metric3", nextDay)
		if got30min < got15min {
			t.Fatalf("unexpected metric id count for next day: got %d, want > %d", got30min, got15min)
		}

		time.Sleep(15 * time.Minute) // 2000-01-01T23:45:00Z
		mrs4 := testGenerateMetricRowsWithPrefix(rng, numSeries, "metric4", TimeRange{
			MinTimestamp: time.Now().Add(-15 * time.Minute).UnixMilli(),
			MaxTimestamp: time.Now().UnixMilli(),
		})
		s.AddRows(mrs4, defaultPrecisionBits)
		s.DebugFlush()
		got45min := countMetricIDs(t, s, "metric4", nextDay)
		if got45min < got30min {
			t.Fatalf("unexpected metric id count for next day: got %d, want > %d", got45min, got30min)
		}

		// Sleep until the next day
		// do not close storage, it resets dataMetricID cache and it will result into slow inserts
		// since dateMetricID cache is not persisted on-disk

		time.Sleep(35 * time.Minute) // 2000-01-02T00:20:00Z
		synctest.Wait()

		// Ingest data for the next day, it must hit dateMetricID cache and
		// do not result into significant amount of slow inserts.
		var m Metrics
		s.UpdateMetrics(&m)
		currDaySlowInserts := m.SlowPerDayIndexInserts
		mrs3NextDay := testGenerateMetricRowsWithPrefix(rng, numSeries, "metric3", TimeRange{
			MinTimestamp: time.Now().Add(-5 * time.Minute).UnixMilli(),
			MaxTimestamp: time.Now().UnixMilli(),
		})

		s.AddRows(mrs3NextDay, defaultPrecisionBits)
		s.DebugFlush()
		m.Reset()
		s.UpdateMetrics(&m)
		nextDaySlowInserts := m.SlowPerDayIndexInserts
		slowInserts := nextDaySlowInserts - currDaySlowInserts
		if slowInserts >= numSeries {
			t.Fatalf("unexpected amount of slow inserts: got %d, want < %d", slowInserts, numSeries)
		}

	})
}

func assertPendingNextDayMetricIDsEmpty(t *testing.T, s *Storage) {
	t.Helper()
	got := s.pendingNextDayMetricIDs.Len()
	if got != 0 {
		t.Fatalf("unexpected s.pendingNextDayMetricIDs count: got %d, want 0", got)
	}
}

func assertPendingNextDayMetricIDsNotEmpty(t *testing.T, s *Storage) {
	t.Helper()
	got := s.pendingNextDayMetricIDs.Len()
	if got == 0 {
		t.Fatalf("unexpected s.pendingNextDayMetricIDs count: got 0, want > 0")
	}
}

func assertNextDayMetricIDs(t *testing.T, s *Storage, wantIDBID, wantDate uint64, wantLen int) {
	t.Helper()

	gotNextDayMetricIDs := s.nextDayMetricIDs.Load()

	if got, want := gotNextDayMetricIDs.idbID, wantIDBID; got != want {
		t.Fatalf("unexpected nextDayMetricIDs idb id: got %d, want %d", got, want)
	}
	if got, want := gotNextDayMetricIDs.date, wantDate; got != want {
		t.Fatalf("unexpected nextDayMetricIDs date: got %d, want %d", got, want)
	}
	if got, want := gotNextDayMetricIDs.metricIDs.Len(), wantLen; got != want {
		t.Fatalf("unexpected nextDayMetricIDs count: got %d, want %d", got, want)
	}
}

func sleepUntil(t *testing.T, year int, month time.Month, day, hour, min, sec, nsec int) {
	t.Helper()
	future := time.Date(year, month, day, hour, min, sec, nsec, time.UTC)
	now := time.Now().UTC()
	d := future.Sub(now)
	if d <= 0 {
		t.Fatalf("future time %v is before now time %v", future, now)
	}
	time.Sleep(d)
}

// TestStorageNextDayMetricIDs_updatedAsynchronously verifies that the metricIDs
// registered in per-day index during the next day prefill do not appear in
// Storage.nextDayMetricIDs right away but only after some time (seconds).
func TestStorageNextDayMetricIDs_updatedAsynchronously(t *testing.T) {
	defer testRemoveAll(t)

	synctest.Test(t, func(t *testing.T) {
		// synctest starts at 2000-01-01T00:00:00Z.

		// Advance time to the last hour of the day to enable next day index
		// prefill.
		sleepUntil(t, 2000, 1, 1, 23, 30, 30, 0)

		const numSeries = 1000
		s := MustOpenStorage(t.Name(), OpenOptions{})
		defer s.MustClose()
		ptw := s.tb.MustGetPartition(time.Now().UnixMilli())
		idbID := ptw.pt.idb.id
		s.tb.PutPartition(ptw)
		date := uint64(time.Now().UnixMilli()) / msecPerDay
		rng := rand.New(rand.NewSource(1))

		// Insert some data.
		mrs := testGenerateMetricRowsWithPrefix(rng, numSeries, "metric", TimeRange{
			MinTimestamp: time.Now().Add(-15 * time.Minute).UnixMilli(),
			MaxTimestamp: time.Now().UnixMilli(),
		})
		s.AddRows(mrs, defaultPrecisionBits)
		s.DebugFlush()

		// Immediately after data ingestion, the next day metricIDs must appear
		// in s.pendingNextDayMetricIDs but not in the s.nextDayMetricIDs. The
		// pending metrics will be moved to it by a background task a few seconds
		// later.
		assertPendingNextDayMetricIDsNotEmpty(t, s)
		assertNextDayMetricIDs(t, s, idbID, date, 0)
		numNextDayMetricIDs := s.pendingNextDayMetricIDs.Len()

		// Wait for nextDayMetricIDs to populate. Pending metricIDs must be
		// moved to nextDayMetricIDs after which pendingNextDayMetricIDs must be
		// empty. nextDayMetricIDs must contain the exact number of metricIDs
		// that once were pending.
		time.Sleep(15 * time.Second)
		assertPendingNextDayMetricIDsEmpty(t, s)
		assertNextDayMetricIDs(t, s, idbID, date, numNextDayMetricIDs)
	})
}

// TestStorageNextDayMetricIDs_loadFromStoreToFile verifies the logic of loading
// Storage.nextDayMetricIDs from file during the storage start-up and storing it
// back to a file during the storage shutdown.
func TestStorageNextDayMetricIDs_loadFromStoreToFile(t *testing.T) {
	defer testRemoveAll(t)

	synctest.Test(t, func(t *testing.T) {
		// synctest starts at 2000-01-01T00:00:00Z.

		// Advance time to the last hour of the day to enable next day index
		// prefill.
		sleepUntil(t, 2000, 1, 1, 23, 30, 30, 0)

		const numSeries = 1000
		s := MustOpenStorage(t.Name(), OpenOptions{})
		ptw := s.tb.MustGetPartition(time.Now().UnixMilli())
		idbID := ptw.pt.idb.id
		s.tb.PutPartition(ptw)
		date := uint64(time.Now().UnixMilli()) / msecPerDay

		// Insert some data. Next day metricIDs must appear in
		// Storage.nextDayMetricIDs.
		rng := rand.New(rand.NewSource(1))
		mrs := testGenerateMetricRowsWithPrefix(rng, numSeries, "metric", TimeRange{
			MinTimestamp: time.Now().Add(-15 * time.Minute).UnixMilli(),
			MaxTimestamp: time.Now().UnixMilli(),
		})
		s.AddRows(mrs, defaultPrecisionBits)
		s.DebugFlush()
		numNextDayMetricIDs := s.pendingNextDayMetricIDs.Len()
		time.Sleep(15 * time.Second)
		assertNextDayMetricIDs(t, s, idbID, date, numNextDayMetricIDs)

		// Close the storage to persist nextDayMetricIDs to a file and then open
		// it again to ensure that nextDayMetricIDs is populated correctly.
		s.MustClose()
		s = MustOpenStorage(t.Name(), OpenOptions{})
		assertNextDayMetricIDs(t, s, idbID, date, numNextDayMetricIDs)

		// Close storage and open it again at the last moment of the day.
		// nextDayMetricIDs must still be populated.
		s.MustClose()
		sleepUntil(t, 2000, 1, 1, 23, 59, 59, 999_999_999)
		s = MustOpenStorage(t.Name(), OpenOptions{})
		assertNextDayMetricIDs(t, s, idbID, date, numNextDayMetricIDs)

		// Close storage and open it again at the first second of the next day.
		// nextDayMetricIDs must still be populated because nextDayMetricIDs
		// needs to be preserved during the first hour of the day in order to
		// speed up data ingestion.
		s.MustClose()
		sleepUntil(t, 2000, 1, 2, 0, 0, 0, 0)
		s = MustOpenStorage(t.Name(), OpenOptions{})
		assertNextDayMetricIDs(t, s, idbID, date, numNextDayMetricIDs)

		// Close storage and open it again at the last moment of the first hour
		// of the next day. nextDayMetricIDs must still be populated.
		s.MustClose()
		sleepUntil(t, 2000, 1, 2, 0, 59, 59, 999_999_999)
		s = MustOpenStorage(t.Name(), OpenOptions{})
		assertNextDayMetricIDs(t, s, idbID, date, numNextDayMetricIDs)

		// Close storage and open it again at the first second of the second
		// hour of the next day. nextDayMetricIDs must be reset.
		s.MustClose()
		sleepUntil(t, 2000, 1, 2, 1, 0, 0, 0)
		s = MustOpenStorage(t.Name(), OpenOptions{})
		assertNextDayMetricIDs(t, s, idbID, date+1, 0)

		// Close the storage and open it again at the last hour.
		// Insert data again to populate nextDayMetricIDs.
		s.MustClose()
		sleepUntil(t, 2000, 1, 2, 23, 30, 0, 0)
		s = MustOpenStorage(t.Name(), OpenOptions{})
		mrs = testGenerateMetricRowsWithPrefix(rng, numSeries, "metric", TimeRange{
			MinTimestamp: time.Now().Add(-15 * time.Minute).UnixMilli(),
			MaxTimestamp: time.Now().UnixMilli(),
		})
		s.AddRows(mrs, defaultPrecisionBits)
		s.DebugFlush()
		numNextDayMetricIDs = s.pendingNextDayMetricIDs.Len()
		time.Sleep(15 * time.Second)
		assertNextDayMetricIDs(t, s, idbID, date+1, numNextDayMetricIDs)

		// Close storage and open it again 24h later.
		// While it is the last hour of the day, the current date and the date
		// in nextDayMetricIDs do not match and therefore nextDayMetricIDs must
		// not be populated.
		s.MustClose()
		sleepUntil(t, 2000, 1, 3, 23, 30, 0, 0)
		s = MustOpenStorage(t.Name(), OpenOptions{})
		assertNextDayMetricIDs(t, s, idbID, date+2, 0)

		// Ingest some data and confirm nextDayMetricIDs is not empty.
		mrs = testGenerateMetricRowsWithPrefix(rng, numSeries, "metric", TimeRange{
			MinTimestamp: time.Now().Add(-15 * time.Minute).UnixMilli(),
			MaxTimestamp: time.Now().UnixMilli(),
		})
		s.AddRows(mrs, defaultPrecisionBits)
		s.DebugFlush()
		numNextDayMetricIDs = s.pendingNextDayMetricIDs.Len()
		time.Sleep(15 * time.Second)
		assertNextDayMetricIDs(t, s, idbID, date+2, numNextDayMetricIDs)

		// Close storage and open it again at the first hour of the day after
		// tomorrow. While it is the last hour of the day, the metricIDs in
		// nextDayMetricIDs is not from yesterday but the day before yesterday
		// and therefore nextDayMetricIDs must not be populated but it's date
		// must still be day before the current date.
		s.MustClose()
		sleepUntil(t, 2000, 1, 5, 0, 30, 0, 0)
		s = MustOpenStorage(t.Name(), OpenOptions{})
		assertNextDayMetricIDs(t, s, idbID, date+3, 0)

		// Close the storage to conclude the test.
		s.MustClose()
	})
}

// TestStorageNextDayMetricIDs_loadFromStoreToFile verifies the logic of updating
// Storage.nextDayMetricIDs at runtime.
func TestStorageNextDayMetricIDs_update(t *testing.T) {
	defer testRemoveAll(t)

	synctest.Test(t, func(t *testing.T) {
		// synctest starts at 2000-01-01T00:00:00Z.

		// Advance time to just before the last hour of the day.
		sleepUntil(t, 2000, 1, 1, 22, 59, 59, 999_999_999)

		const numSeries = 1000
		s := MustOpenStorage(t.Name(), OpenOptions{})
		defer s.MustClose()
		ptw := s.tb.MustGetPartition(time.Now().UnixMilli())
		idbID := ptw.pt.idb.id
		s.tb.PutPartition(ptw)
		date := uint64(time.Now().UnixMilli()) / msecPerDay
		rng := rand.New(rand.NewSource(1))

		// The next day index prefill must not start before the last hour of the
		// day. Therefore, nextDayMetricIDs must be empty.
		mrs := testGenerateMetricRowsWithPrefix(rng, numSeries, "metric", TimeRange{
			MinTimestamp: time.Now().Add(-15 * time.Minute).UnixMilli(),
			MaxTimestamp: time.Now().UnixMilli(),
		})
		s.AddRows(mrs, defaultPrecisionBits)
		s.DebugFlush()
		assertNextDayMetricIDs(t, s, idbID, date, 0)

		// Advance time to the middle of the last hour of the day to enable next
		// day prefill and insert some data. Next day metricIDs must appear in
		// Storage.nextDayMetricIDs.
		sleepUntil(t, 2000, 1, 1, 23, 30, 0, 0)
		mrs = testGenerateMetricRowsWithPrefix(rng, numSeries, "metric", TimeRange{
			MinTimestamp: time.Now().Add(-15 * time.Minute).UnixMilli(),
			MaxTimestamp: time.Now().UnixMilli(),
		})
		s.AddRows(mrs, defaultPrecisionBits)
		s.DebugFlush()
		numNextDayMetricIDs := s.pendingNextDayMetricIDs.Len()
		time.Sleep(15 * time.Second)
		assertNextDayMetricIDs(t, s, idbID, date, numNextDayMetricIDs)

		// Advance the time to the end of the last hour of the current day and
		// confirm that nextDayMetricIDs are not reset during this hour.
		sleepUntil(t, 2000, 1, 1, 23, 59, 59, 999_999_999)
		assertNextDayMetricIDs(t, s, idbID, date, numNextDayMetricIDs)

		// Advance the time to the end of the first hour of the next day and
		// confirm that nextDayMetricIDs are not reset during this hour.
		sleepUntil(t, 2000, 1, 2, 0, 59, 59, 999_999_999)
		assertNextDayMetricIDs(t, s, idbID, date, numNextDayMetricIDs)

		// Advance the time to the beginning of the second hour of the next day and
		// confirm that nextDayMetricIDs is reset.
		sleepUntil(t, 2000, 1, 2, 1, 0, 30, 0)
		assertNextDayMetricIDs(t, s, idbID, date+1, 0)
	})
}

// TestStorageLastPartitionMetrics checks that "last partition" metrics
// correspond to the current partition and not some future partition.
func TestStorageLastPartitionMetrics(t *testing.T) {
	defer testRemoveAll(t)

	addRows := func(t *testing.T, s *Storage, prefix string, tr TimeRange) {
		t.Helper()
		const numSeries = 1000
		rng := rand.New(rand.NewSource(1))
		mrs := testGenerateMetricRowsWithPrefix(rng, numSeries, prefix, tr)
		want := s.newTimeseriesCreated.Load() + numSeries
		s.AddRows(mrs, defaultPrecisionBits)
		s.DebugFlush()
		if got := s.newTimeseriesCreated.Load(); got != want {
			t.Fatalf("unexpected number of new timeseries: got %d, want %d", got, want)
		}
		// wait for merged parts to be attached to the table
		time.Sleep(time.Minute)
	}
	assertLastPartitionEmpty := func(t *testing.T, s *Storage) {
		t.Helper()
		var m Metrics
		s.UpdateMetrics(&m)
		lpm := m.TableMetrics.LastPartition
		if lpm.SmallPartsCount != 0 {
			t.Fatalf("unexpected last partition SmallPartsCount: got %d, want 0", lpm.SmallPartsCount)
		}
		if lpm.IndexDBMetrics.FileBlocksCount != 0 {
			t.Fatalf("unexpected last partition IndexDBMetrics.FileBlocksCount: got %d, want 0", lpm.IndexDBMetrics.FileBlocksCount)
		}
	}
	assertLastPartitionNonEmpty := func(t *testing.T, s *Storage) {
		t.Helper()
		var m Metrics
		s.UpdateMetrics(&m)
		lpm := m.TableMetrics.LastPartition
		if lpm.SmallPartsCount == 0 {
			t.Fatalf("unexpected last partition SmallPartsCount: got 0, want > 0")
		}
		if lpm.IndexDBMetrics.FileBlocksCount == 0 {
			t.Fatalf("unexpected last partition IndexDBMetrics.FileBlocksCount: got 0, want > 0")
		}
	}

	synctest.Test(t, func(t *testing.T) {
		// Advance current time to 2h before the next month, 2000-01-31T22:00:00Z.
		time.Sleep(31*24*time.Hour - 2*time.Hour)
		ct := time.Now().UTC()

		// Open the storage, make sure current partition is empty.
		s := MustOpenStorage(t.Name(), OpenOptions{
			FutureRetention: 2 * 365 * 24 * time.Hour,
		})
		defer s.MustClose()
		assertLastPartitionEmpty(t, s)

		// Insert rows with future timestamps. Current partition must be empty.
		addRows(t, s, "future", TimeRange{
			MinTimestamp: ct.Add(365 * 24 * time.Hour).UnixMilli(),
			MaxTimestamp: ct.Add(366 * 24 * time.Hour).UnixMilli(),
		})
		assertLastPartitionEmpty(t, s)

		// Insert rows with timestamps within current partition.
		// Current partition must be not empty.
		addRows(t, s, "current", TimeRange{
			MinTimestamp: ct.UnixMilli(),
			MaxTimestamp: ct.Add(time.Hour).UnixMilli(),
		})
		assertLastPartitionNonEmpty(t, s)

		// Advance current time to the the next month, 2000-02-01T00:30:00Z.
		// last partition is now 2000-02 and it must be empty.
		time.Sleep(2*time.Hour + time.Minute*30)
		assertLastPartitionEmpty(t, s)
	})
}

func TestStorage_futureAndHistoricalRetention(t *testing.T) {
	defer testRemoveAll(t)

	assertData := func(t *testing.T, s *Storage, tr TimeRange, want []MetricRow) {
		t.Helper()
		tfs := NewTagFilters()
		if err := tfs.Add(nil, []byte(".*"), false, true); err != nil {
			t.Fatalf("TagFilters.Add() failed unexpectedly: %v", err)
		}
		if err := testAssertSearchResult(s, tr, tfs, want); err != nil {
			t.Fatalf("[now: %v tr: %v] search failed unexpectedly: %v", time.Now().UTC(), &tr, err)
		}
	}

	synctest.Test(t, func(t *testing.T) {
		// synctests start at 2000-01-01T00:00:00Z

		var s *Storage
		retention := 180 * 24 * time.Hour
		futureRetention := 180 * 24 * time.Hour

		s = MustOpenStorage(t.Name(), OpenOptions{
			Retention:       retention,
			FutureRetention: futureRetention,
		})

		// Ingest samples for previous and future year. 10 samples per day.
		const numSeries = 10
		start := time.Date(1999, 1, 1, 0, 0, 0, 0, time.UTC)
		end := time.Date(2001, 1, 1, 0, 0, 0, 0, time.UTC)
		rng := rand.New(rand.NewSource(1))
		wantData := make(map[TimeRange][]MetricRow)
		for day := start; day.Before(end); {
			prefix := fmt.Sprintf("metric_%d_%d_%d", day.Year(), day.Month(), day.Day())
			tr := TimeRange{
				MinTimestamp: day.UnixMilli(),
				MaxTimestamp: day.UnixMilli() + msecPerDay - 1,
			}
			mrs := testGenerateMetricRowsWithPrefix(rng, numSeries, prefix, tr)
			wantData[tr] = mrs
			s.AddRows(mrs, defaultPrecisionBits)

			day = time.Date(day.Year(), day.Month(), day.Day()+1, 0, 0, 0, 0, time.UTC)
		}
		s.DebugFlush()

		// Advance time one partition at a time. Before each time advancement,
		// check the query results for each day between the original start and end
		// time.
		//
		// This is to test how historical and future retentions affect the
		// stored data over time.
		now := time.Now().UTC()
		dataEnd := now.Add(futureRetention - 24*time.Hour)
		for now.Before(end) {
			for day := start; day.Before(end); {
				tr := TimeRange{
					MinTimestamp: day.UnixMilli(),
					MaxTimestamp: day.UnixMilli() + msecPerDay - 1,
				}
				dataStart := now.Add(-retention)
				if day.Before(dataStart) || day.After(dataEnd) {
					assertData(t, s, tr, nil)
				} else {
					assertData(t, s, tr, wantData[tr])
				}
				day = time.Date(day.Year(), day.Month(), day.Day()+1, 0, 0, 0, 0, time.UTC)
			}

			s.MustClose()
			nextMonth := time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, 0, time.UTC)
			time.Sleep(nextMonth.Sub(now))
			now = nextMonth
			s = MustOpenStorage(t.Name(), OpenOptions{
				Retention:       retention,
				FutureRetention: futureRetention,
			})
		}

		s.MustClose()
	})
}

func TestStorage_defaultFutureRetention(t *testing.T) {
	defer testRemoveAll(t)

	assertData := func(t *testing.T, s *Storage, tr TimeRange, want []MetricRow) {
		t.Helper()
		tfs := NewTagFilters()
		if err := tfs.Add(nil, []byte(".*"), false, true); err != nil {
			t.Fatalf("TagFilters.Add() failed unexpectedly: %v", err)
		}
		if err := testAssertSearchResult(s, tr, tfs, want); err != nil {
			t.Fatalf("[now: %v tr: %v] search failed unexpectedly: %v", time.Now().UTC(), &tr, err)
		}
	}

	synctest.Test(t, func(t *testing.T) {
		// synctests start at 2000-01-01T00:00:00Z

		s := MustOpenStorage(t.Name(), OpenOptions{})
		defer s.MustClose()

		// Ingest samples for this and several days in the future. 10 samples
		// per hour.
		const numSeries = 10
		start := time.Now().UTC()
		end := start.Add(10 * 24 * time.Hour)
		rng := rand.New(rand.NewSource(1))
		wantData := make(map[TimeRange][]MetricRow)
		for ts := start; ts.Before(end); {
			prefix := fmt.Sprintf("metric_%04d_%02d_%02d_%02d", ts.Year(), ts.Month(), ts.Day(), ts.Hour())
			tr := TimeRange{
				MinTimestamp: ts.UnixMilli(),
				MaxTimestamp: ts.UnixMilli() + msecPerHour - 1,
			}
			mrs := testGenerateMetricRowsWithPrefix(rng, numSeries, prefix, tr)
			wantData[tr] = mrs
			s.AddRows(mrs, defaultPrecisionBits)

			ts = ts.Add(time.Hour)
		}
		s.DebugFlush()

		dataStart := start
		dataEnd := dataStart.Add(2*24*time.Hour - time.Hour)
		for ts := start; ts.Before(end); ts = ts.Add(time.Hour) {
			tr := TimeRange{
				MinTimestamp: ts.UnixMilli(),
				MaxTimestamp: ts.UnixMilli() + msecPerHour - 1,
			}
			if ts.Before(dataStart) || ts.After(dataEnd) {
				assertData(t, s, tr, nil)
			} else {
				assertData(t, s, tr, wantData[tr])
			}
		}

	})
}

func TestStorage_partitionsOutsideRetentionAreRemoved(t *testing.T) {
	defer testRemoveAll(t)

	assertPathExists := func(t *testing.T, path string, want bool) {
		t.Helper()
		if got := fs.IsPathExist(path); got != want {
			t.Fatalf("unexpected path existence test result for %s: got %t, want %t", path, got, want)
		}
	}

	assertPtExists := func(t *testing.T, pt string, want bool) {
		t.Helper()
		assertPathExists(t, filepath.Join(t.Name(), "data", "small", pt), want)
		assertPathExists(t, filepath.Join(t.Name(), "data", "big", pt), want)
		assertPathExists(t, filepath.Join(t.Name(), "data", "indexdb", pt), want)
	}

	synctest.Test(t, func(t *testing.T) {
		// synctests start at 2000-01-01T00:00:00Z

		retention := 80 * 24 * time.Hour
		futureRetention := 180 * 24 * time.Hour
		s := MustOpenStorage(t.Name(), OpenOptions{
			Retention:       retention,
			FutureRetention: futureRetention,
		})

		// Ingest samples with future timestamps that span the entire retention.
		// This should create the corresponding partitions.
		rng := rand.New(rand.NewSource(1))
		mrs := testGenerateMetricRowsWithPrefix(rng, 1000, "metric", TimeRange{
			MinTimestamp: time.Now().Add(-retention).UnixMilli(),
			MaxTimestamp: time.Now().Add(futureRetention - time.Second).UnixMilli(),
		})
		s.AddRows(mrs, defaultPrecisionBits)
		s.DebugFlush()

		assertPtExists(t, "1999_09", false)
		assertPtExists(t, "1999_10", true)
		assertPtExists(t, "1999_11", true)
		assertPtExists(t, "1999_12", true)
		assertPtExists(t, "2000_01", true)
		assertPtExists(t, "2000_02", true)
		assertPtExists(t, "2000_03", true)
		assertPtExists(t, "2000_04", true)
		assertPtExists(t, "2000_05", true)
		assertPtExists(t, "2000_06", true)
		assertPtExists(t, "2000_07", false)

		// Reopen storage with smaller future retention. Future partitions
		// outside the new future retention must be removed.
		s.MustClose()
		s = MustOpenStorage(t.Name(), OpenOptions{
			Retention:       retention,
			FutureRetention: 45 * 24 * time.Hour,
		})

		// Wait for background task to remove future partitions.
		time.Sleep(2 * time.Minute)

		assertPtExists(t, "1999_09", false)
		assertPtExists(t, "1999_10", true)
		assertPtExists(t, "1999_11", true)
		assertPtExists(t, "1999_12", true)
		assertPtExists(t, "2000_01", true)
		assertPtExists(t, "2000_02", true)
		assertPtExists(t, "2000_03", false)
		assertPtExists(t, "2000_04", false)
		assertPtExists(t, "2000_05", false)
		assertPtExists(t, "2000_06", false)
		assertPtExists(t, "2000_07", false)

		// Reopen storage with smaller retention. Historical partitions
		// outside the new future retention must be removed.
		s.MustClose()
		s = MustOpenStorage(t.Name(), OpenOptions{
			Retention:       45 * 24 * time.Hour,
			FutureRetention: 45 * 24 * time.Hour,
		})

		// Wait for background task to remove future partitions.
		time.Sleep(2 * time.Minute)

		assertPtExists(t, "1999_09", false)
		assertPtExists(t, "1999_10", false)
		assertPtExists(t, "1999_11", true)
		assertPtExists(t, "1999_12", true)
		assertPtExists(t, "2000_01", true)
		assertPtExists(t, "2000_02", true)
		assertPtExists(t, "2000_03", false)
		assertPtExists(t, "2000_04", false)
		assertPtExists(t, "2000_05", false)
		assertPtExists(t, "2000_06", false)
		assertPtExists(t, "2000_07", false)

		s.MustClose()
	})
}

func TestStorage_denyQueriesOutsideRetention(t *testing.T) {
	defer testRemoveAll(t)

	tfsAll := NewTagFilters()
	if err := tfsAll.Add(nil, []byte(".*"), false, true); err != nil {
		t.Fatalf("TagFilters.Add() failed unexpectedly: %v", err)
	}
	tfssAll := []*TagFilters{tfsAll}

	assertData := func(t *testing.T, s *Storage, tr TimeRange, wantData []MetricRow, wantErr bool) {
		t.Helper()
		err := testAssertSearchResult(s, tr, tfsAll, wantData)
		gotErr := err != nil
		if gotErr != wantErr {
			t.Fatalf("Search: unmet error expectation for timeRange=%v: got %t, want %t (err: %v)", &tr, gotErr, wantErr, err)
		}
	}
	assertMetricNames := func(t *testing.T, s *Storage, tr TimeRange, wantData []string, wantErr bool) {
		t.Helper()
		metricNames, err := s.SearchMetricNames(nil, tfssAll, tr, 1e9, noDeadline)
		gotErr := err != nil
		if gotErr != wantErr {
			t.Fatalf("SearchMetricNames(): unmet error expectation for timeRange=%v: got %t, want %t (err: %v)", &tr, gotErr, wantErr, err)
		}
		var gotData []string
		for _, name := range metricNames {
			var mn MetricName
			if err := mn.UnmarshalString(name); err != nil {
				t.Fatalf("Could not unmarshal metric name %q: %v", name, err)
			}
			gotData = append(gotData, string(mn.MetricGroup))
		}
		if diff := cmp.Diff(wantData, gotData); diff != "" {
			t.Fatalf("unexpected metric names (-want, +got):\n%s", diff)
		}
	}
	assertLabelNames := func(t *testing.T, s *Storage, tr TimeRange, wantData []string, wantErr bool) {
		t.Helper()
		gotData, err := s.SearchLabelNames(nil, nil, tr, 1e9, 1e9, noDeadline)
		gotErr := err != nil
		if gotErr != wantErr {
			t.Fatalf("SearchLabelNames(): unmet error expectation for timeRange=%v: got %t, want %t (err: %v)", &tr, gotErr, wantErr, err)
		}
		slices.Sort(gotData)
		slices.Sort(wantData)
		if diff := cmp.Diff(wantData, gotData); diff != "" {
			t.Fatalf("unexpected label names (-want, +got):\n%s", diff)
		}
	}
	assertLabelValues := func(t *testing.T, s *Storage, tr TimeRange, wantData []string, wantErr bool) {
		t.Helper()
		gotData, err := s.SearchLabelValues(nil, "__name__", nil, tr, 1e9, 1e9, noDeadline)
		gotErr := err != nil
		if gotErr != wantErr {
			t.Fatalf("SearchLabelValues(): unmet error expectation for timeRange=%v: got %t, want %t (err: %v)", &tr, gotErr, wantErr, err)
		}
		slices.Sort(gotData)
		slices.Sort(wantData)
		if diff := cmp.Diff(wantData, gotData); diff != "" {
			t.Fatalf("unexpected label values (-want, +got):\n%s", diff)
		}
	}
	assertTagValueSuffixes := func(t *testing.T, s *Storage, tr TimeRange, wantData []string, wantErr bool) {
		t.Helper()
		gotData, err := s.SearchTagValueSuffixes(nil, tr, "", "", '.', 1e9, noDeadline)
		gotErr := err != nil
		if gotErr != wantErr {
			t.Fatalf("SearchTagValueSuffixes(): unmet error expectation for timeRange=%v: got %t, want %t (err: %v)", &tr, gotErr, wantErr, err)
		}
		slices.Sort(gotData)

		if diff := cmp.Diff(wantData, gotData); diff != "" {
			t.Fatalf("unexpected tag value suffixes (-want, +got):\n%s", diff)
		}
	}

	assertGraphitePaths := func(t *testing.T, s *Storage, tr TimeRange, wantData []string, wantErr bool) {
		t.Helper()
		gotData, err := s.SearchGraphitePaths(nil, tr, []byte("*"), 1e9, noDeadline)
		gotErr := err != nil
		if gotErr != wantErr {
			t.Fatalf("SearchTagValueSuffixes(): unmet error expectation for timeRange=%v: got %t, want %t (err: %v)", &tr, gotErr, wantErr, err)
		}
		slices.Sort(gotData)

		if diff := cmp.Diff(wantData, gotData); diff != "" {
			t.Errorf("unexpected graphite paths (-want, +got):\n%s", diff)
		}
	}

	synctest.Test(t, func(t *testing.T) {
		// synctests start at 2000-01-01T00:00:00Z
		now := time.Now().UTC()
		retention := 30 * 24 * time.Hour
		futureRetention := 30 * 24 * time.Hour
		day := 24 * time.Hour
		trMatches := TimeRange{
			MinTimestamp: now.Add(-retention).UnixMilli(),
			MaxTimestamp: now.Add(futureRetention).UnixMilli(),
		}
		trContains := TimeRange{
			MinTimestamp: now.Add(-(retention + day)).UnixMilli(),
			MaxTimestamp: now.Add(futureRetention + day).UnixMilli(),
		}
		trInside := TimeRange{
			MinTimestamp: now.Add(-(retention - day)).UnixMilli(),
			MaxTimestamp: now.Add(futureRetention - day).UnixMilli(),
		}
		trOverlapsLeft := TimeRange{
			MinTimestamp: now.Add(-(retention + day)).UnixMilli(),
			MaxTimestamp: now.Add(futureRetention - day).UnixMilli(),
		}
		trOverlapsRight := TimeRange{
			MinTimestamp: now.Add(-(retention - day)).UnixMilli(),
			MaxTimestamp: now.Add(futureRetention + day).UnixMilli(),
		}
		trOutsideLeft := TimeRange{
			MinTimestamp: now.Add(-(retention + 2*day)).UnixMilli(),
			MaxTimestamp: now.Add(-(retention + day)).UnixMilli(),
		}
		trOutsideRight := TimeRange{
			MinTimestamp: now.Add(futureRetention + day).UnixMilli(),
			MaxTimestamp: now.Add(futureRetention + 2*day).UnixMilli(),
		}

		mn := MetricName{
			MetricGroup: []byte("metric"),
		}
		metricNameRaw := mn.marshalRaw(nil)
		mr := MetricRow{
			MetricNameRaw: metricNameRaw,
			Timestamp:     now.UnixMilli(),
			Value:         123,
		}
		wantMRs := []MetricRow{mr}
		wantMetricNames := []string{"metric"}
		wantLabelNames := []string{"__name__"}
		emptyLabelNames := []string{}
		wantLabelValues := []string{"metric"}
		emptyLabelValues := []string{}
		wantTagValueSuffixes := []string{"metric"}
		emptyTagValueSuffixes := []string{}
		wantGraphitePaths := []string{"metric"}
		emptyGraphitePaths := []string{}

		s := MustOpenStorage(t.Name(), OpenOptions{
			Retention:       retention,
			FutureRetention: futureRetention,
		})

		s.AddRows([]MetricRow{mr}, defaultPrecisionBits)
		s.DebugFlush()

		assertData(t, s, trInside, wantMRs, false)
		assertData(t, s, trMatches, wantMRs, false)
		assertData(t, s, trContains, wantMRs, false)
		assertData(t, s, trOverlapsLeft, wantMRs, false)
		assertData(t, s, trOverlapsRight, wantMRs, false)
		assertData(t, s, trOutsideLeft, nil, false)
		assertData(t, s, trOutsideRight, nil, false)

		assertMetricNames(t, s, trInside, wantMetricNames, false)
		assertMetricNames(t, s, trMatches, wantMetricNames, false)
		assertMetricNames(t, s, trContains, wantMetricNames, false)
		assertMetricNames(t, s, trOverlapsLeft, wantMetricNames, false)
		assertMetricNames(t, s, trOverlapsRight, wantMetricNames, false)
		assertMetricNames(t, s, trOutsideLeft, nil, false)
		assertMetricNames(t, s, trOutsideRight, nil, false)

		assertLabelNames(t, s, trInside, wantLabelNames, false)
		assertLabelNames(t, s, trMatches, wantLabelNames, false)
		assertLabelNames(t, s, trContains, wantLabelNames, false)
		assertLabelNames(t, s, trOverlapsLeft, wantLabelNames, false)
		assertLabelNames(t, s, trOverlapsRight, wantLabelNames, false)
		assertLabelNames(t, s, trOutsideLeft, emptyLabelNames, false)
		assertLabelNames(t, s, trOutsideRight, emptyLabelNames, false)

		assertLabelValues(t, s, trInside, wantLabelValues, false)
		assertLabelValues(t, s, trMatches, wantLabelValues, false)
		assertLabelValues(t, s, trContains, wantLabelValues, false)
		assertLabelValues(t, s, trOverlapsLeft, wantLabelValues, false)
		assertLabelValues(t, s, trOverlapsRight, wantLabelValues, false)
		assertLabelValues(t, s, trOutsideLeft, emptyLabelValues, false)
		assertLabelValues(t, s, trOutsideRight, emptyLabelValues, false)

		assertTagValueSuffixes(t, s, trInside, wantTagValueSuffixes, false)
		assertTagValueSuffixes(t, s, trMatches, wantTagValueSuffixes, false)
		assertTagValueSuffixes(t, s, trContains, wantTagValueSuffixes, false)
		assertTagValueSuffixes(t, s, trOverlapsLeft, wantTagValueSuffixes, false)
		assertTagValueSuffixes(t, s, trOverlapsRight, wantTagValueSuffixes, false)
		assertTagValueSuffixes(t, s, trOutsideLeft, emptyTagValueSuffixes, false)
		assertTagValueSuffixes(t, s, trOutsideRight, emptyTagValueSuffixes, false)

		assertGraphitePaths(t, s, trInside, wantGraphitePaths, false)
		assertGraphitePaths(t, s, trMatches, wantGraphitePaths, false)
		assertGraphitePaths(t, s, trContains, wantGraphitePaths, false)
		assertGraphitePaths(t, s, trOverlapsLeft, wantGraphitePaths, false)
		assertGraphitePaths(t, s, trOverlapsRight, wantGraphitePaths, false)
		assertGraphitePaths(t, s, trOutsideLeft, emptyGraphitePaths, false)
		assertGraphitePaths(t, s, trOutsideRight, emptyGraphitePaths, false)

		// Restart storage with DenyQueriesOutsideRetention on.
		s.MustClose()
		s = MustOpenStorage(t.Name(), OpenOptions{
			Retention:                   retention,
			FutureRetention:             futureRetention,
			DenyQueriesOutsideRetention: true,
		})
		defer s.MustClose()

		assertData(t, s, trInside, wantMRs, false)
		assertData(t, s, trMatches, wantMRs, false)
		assertData(t, s, trContains, nil, true)
		assertData(t, s, trOverlapsLeft, nil, true)
		assertData(t, s, trOverlapsRight, nil, true)
		assertData(t, s, trOutsideLeft, nil, true)
		assertData(t, s, trOutsideRight, nil, true)

		assertMetricNames(t, s, trInside, wantMetricNames, false)
		assertMetricNames(t, s, trMatches, wantMetricNames, false)
		assertMetricNames(t, s, trContains, nil, true)
		assertMetricNames(t, s, trOverlapsLeft, nil, true)
		assertMetricNames(t, s, trOverlapsRight, nil, true)
		assertMetricNames(t, s, trOutsideLeft, nil, true)
		assertMetricNames(t, s, trOutsideRight, nil, true)

		// DenyQueriesOutsideRetention does not apply to searching label names.
		assertLabelNames(t, s, trInside, wantLabelNames, false)
		assertLabelNames(t, s, trMatches, wantLabelNames, false)
		assertLabelNames(t, s, trContains, wantLabelNames, false)
		assertLabelNames(t, s, trOverlapsLeft, wantLabelNames, false)
		assertLabelNames(t, s, trOverlapsRight, wantLabelNames, false)
		assertLabelNames(t, s, trOutsideLeft, emptyLabelNames, false)
		assertLabelNames(t, s, trOutsideRight, emptyLabelNames, false)

		// DenyQueriesOutsideRetention does not apply to searching label values.
		assertLabelValues(t, s, trInside, wantLabelValues, false)
		assertLabelValues(t, s, trMatches, wantLabelValues, false)
		assertLabelValues(t, s, trContains, wantLabelValues, false)
		assertLabelValues(t, s, trOverlapsLeft, wantLabelValues, false)
		assertLabelValues(t, s, trOverlapsRight, wantLabelValues, false)
		assertLabelValues(t, s, trOutsideLeft, emptyLabelValues, false)
		assertLabelValues(t, s, trOutsideRight, emptyLabelValues, false)

		// DenyQueriesOutsideRetention does not apply to searching tag value suffixes.
		assertTagValueSuffixes(t, s, trInside, wantTagValueSuffixes, false)
		assertTagValueSuffixes(t, s, trMatches, wantTagValueSuffixes, false)
		assertTagValueSuffixes(t, s, trContains, wantTagValueSuffixes, false)
		assertTagValueSuffixes(t, s, trOverlapsLeft, wantTagValueSuffixes, false)
		assertTagValueSuffixes(t, s, trOverlapsRight, wantTagValueSuffixes, false)
		assertTagValueSuffixes(t, s, trOutsideLeft, emptyTagValueSuffixes, false)
		assertTagValueSuffixes(t, s, trOutsideRight, emptyTagValueSuffixes, false)

		// DenyQueriesOutsideRetention does not apply to searching graphite paths.
		assertGraphitePaths(t, s, trInside, wantGraphitePaths, false)
		assertGraphitePaths(t, s, trMatches, wantGraphitePaths, false)
		assertGraphitePaths(t, s, trContains, wantGraphitePaths, false)
		assertGraphitePaths(t, s, trOverlapsLeft, wantGraphitePaths, false)
		assertGraphitePaths(t, s, trOverlapsRight, wantGraphitePaths, false)
		assertGraphitePaths(t, s, trOutsideLeft, emptyGraphitePaths, false)
		assertGraphitePaths(t, s, trOutsideRight, emptyGraphitePaths, false)

	})
}

func TestStorageAddRows_MaxBackfillAge(t *testing.T) {
	defer testRemoveAll(t)

	mn := MetricName{
		MetricGroup: []byte("metric"),
	}
	mr := MetricRow{
		MetricNameRaw: mn.marshalRaw(nil),
		Value:         123,
	}

	f := func(s *Storage, age time.Duration, want uint64) {
		t.Helper()

		mr.Timestamp = time.Now().UTC().Add(-age).UnixMilli()
		s.AddRows([]MetricRow{mr}, defaultPrecisionBits)
		s.DebugFlush()
		if got := s.tooSmallTimestampRows.Load(); got != want {
			t.Fatalf("unexpected number of tooSmallTimestampRows: got %d, want %d", got, want)
		}
	}

	synctest.Test(t, func(t *testing.T) {
		// synctest time begins at 2000-01-01T00:00:00Z.

		retention1y := 365 * 24 * time.Hour
		var s *Storage

		s = MustOpenStorage(t.Name(), OpenOptions{
			Retention: retention1y,
			// By default MaxBackfillAge must be the same as Retention
		})
		// Verify that the sample with timestamp 1ms older than retention is
		// rejected.
		f(s, retention1y+time.Millisecond, 1)
		// Verify that the sample with timestamp which is exactly at retention
		// boundary is accepted.
		f(s, retention1y, 1)

		// Restart storage with negative MaxBackfillAge. In this case,
		// MaxBackfillAge must be the same as Retention.
		// Also advance time a bit so that the storage will not use the same
		// nanosecond for creating a new part for storing the samples.
		s.MustClose()
		time.Sleep(time.Nanosecond)
		s = MustOpenStorage(t.Name(), OpenOptions{
			Retention:      retention1y,
			MaxBackfillAge: -1,
		})
		f(s, retention1y+time.Millisecond, 1)
		f(s, retention1y, 1)

		// Restart storage with MaxBackfillAge bigger than Retention. In this
		// case, MaxBackfillAge must be the same as Retention.
		s.MustClose()
		time.Sleep(time.Nanosecond)
		s = MustOpenStorage(t.Name(), OpenOptions{
			Retention:      retention1y,
			MaxBackfillAge: retention1y + time.Millisecond,
		})
		f(s, retention1y+time.Millisecond, 1)
		f(s, retention1y, 1)

		// Restart storage with MaxBackfillAge smaller than Retention.
		s.MustClose()
		time.Sleep(time.Nanosecond)
		s = MustOpenStorage(t.Name(), OpenOptions{
			Retention:      retention1y,
			MaxBackfillAge: retention1y - time.Millisecond,
		})
		f(s, retention1y, 1)
		f(s, retention1y-time.Millisecond, 1)

		s.MustClose()
	})
}

func TestStorageAddRows_currHourMetricIDs(t *testing.T) {
	defer testRemoveAll(t)

	f := func(t *testing.T, disablePerDayIndex bool) {
		synctest.Test(t, func(t *testing.T) {
			s := MustOpenStorage(t.Name(), OpenOptions{
				DisablePerDayIndex: disablePerDayIndex,
			})
			defer s.MustClose()

			now := time.Now().UTC()
			currHourTR := TimeRange{
				MinTimestamp: time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), 0, 0, 0, time.UTC).UnixMilli(),
				MaxTimestamp: time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), 59, 59, 999_999_999, time.UTC).UnixMilli(),
			}
			currHour := uint64(currHourTR.MinTimestamp / 1000 / 3600)
			prevHourTR := TimeRange{
				MinTimestamp: currHourTR.MinTimestamp - msecPerHour,
				MaxTimestamp: currHourTR.MaxTimestamp - msecPerHour,
			}
			rng := rand.New(rand.NewSource(1))

			// Test current hour metricIDs population when data ingestion takes the
			// slow path. The database is empty, therefore the index and the
			// tsidCache contain no metricIDs, therefore the data ingestion will
			// take slow path.

			mrs := testGenerateMetricRowsWithPrefix(rng, 1000, "slow_path", currHourTR)
			s.AddRows(mrs, defaultPrecisionBits)
			s.DebugFlush()
			s.updateCurrHourMetricIDs(currHour)
			if got, want := s.currHourMetricIDs.Load().m.Len(), 1000; got != want {
				t.Fatalf("[slow path] unexpected current hour metric ID count: got %d, want %d", got, want)
			}

			// Test current hour metricIDs population when data ingestion takes the
			// fast path (when the metricIDs are found in the tsidCache)

			// First insert samples to populate the tsidCache. The samples belong to
			// the previous hour, therefore the metricIDs won't be added to
			// currHourMetricIDs.
			mrs = testGenerateMetricRowsWithPrefix(rng, 1000, "fast_path", prevHourTR)
			s.AddRows(mrs, defaultPrecisionBits)
			s.DebugFlush()
			s.updateCurrHourMetricIDs(currHour)
			if got, want := s.currHourMetricIDs.Load().m.Len(), 1000; got != want {
				t.Fatalf("[fast path] unexpected current hour metric ID count after ingesting samples for previous hour: got %d, want %d", got, want)
			}

			// Now ingest the same metrics. This time the metricIDs will be found in
			// tsidCache so the ingestion will take the fast path.
			mrs = testGenerateMetricRowsWithPrefix(rng, 1000, "fast_path", currHourTR)
			s.AddRows(mrs, defaultPrecisionBits)
			s.DebugFlush()
			s.updateCurrHourMetricIDs(currHour)
			if got, want := s.currHourMetricIDs.Load().m.Len(), 2000; got != want {
				t.Fatalf("[fast path] unexpected current hour metric ID count: got %d, want %d", got, want)
			}

			// Test current hour metricIDs population when data ingestion takes the
			// slower path (when the metricIDs are not found in the tsidCache but
			// found in the index)

			// First insert samples to populate the index. The samples belong to
			// the previous hour, therefore the metricIDs won't be added to
			// currHourMetricIDs.
			mrs = testGenerateMetricRowsWithPrefix(rng, 1000, "slower_path", prevHourTR)
			s.AddRows(mrs, defaultPrecisionBits)
			s.DebugFlush()
			s.updateCurrHourMetricIDs(currHour)
			if got, want := s.currHourMetricIDs.Load().m.Len(), 2000; got != want {
				t.Fatalf("[slower path] unexpected current hour metric ID count after ingesting samples for previous hour: got %d, want %d", got, want)
			}
			// Inserted samples were also added to the tsidCache. Drop it to
			// enforce the fallback to index search.
			s.resetAndSaveTSIDCache()

			// Now ingest the same metrics. This time the metricIDs will be searched
			// and found in index so the ingestion will take the slower path.
			mrs = testGenerateMetricRowsWithPrefix(rng, 1000, "slower_path", currHourTR)
			s.AddRows(mrs, defaultPrecisionBits)
			s.DebugFlush()
			s.updateCurrHourMetricIDs(currHour)
			if got, want := s.currHourMetricIDs.Load().m.Len(), 3000; got != want {
				t.Fatalf("[slower path] unexpected current hour metric ID count: got %d, want %d", got, want)
			}
		})
	}

	t.Run("disablePerDayIndex=false", func(t *testing.T) {
		f(t, false)
	})
	t.Run("disablePerDayIndex=true", func(t *testing.T) {
		f(t, true)
	})
}
