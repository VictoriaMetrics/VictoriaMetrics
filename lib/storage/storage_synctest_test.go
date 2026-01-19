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
			t.Errorf("unexpected amount of slow inserts: got %d, want < %d", slowInserts, numSeries)
		}

	})
}

func TestStorageMustLoadNextDayMetricIDs(t *testing.T) {
	defer testRemoveAll(t)

	assertNextDayMetricIDs := func(t *testing.T, gotNextDayMetricIDs *nextDayMetricIDs, wantIDBID, wantDate uint64, wantLen int) {
		t.Helper()

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

	synctest.Test(t, func(t *testing.T) {
		// synctest starts at 2000-01-01T00:00:00Z.
		// Advance time to 23:30 to enable next day prefill.
		time.Sleep(23*time.Hour + 30*time.Minute) // 2000-01-01T23:30:00Z
		date := uint64(time.Now().UnixMilli()) / msecPerDay

		const numSeries = 1000
		s := MustOpenStorage(t.Name(), OpenOptions{})
		ptw := s.tb.MustGetPartition(time.Now().UnixMilli())
		idbID := ptw.pt.idb.id
		s.tb.PutPartition(ptw)

		rng := rand.New(rand.NewSource(1))
		mrs := testGenerateMetricRowsWithPrefix(rng, numSeries, "metric", TimeRange{
			MinTimestamp: time.Now().Add(-15 * time.Minute).UnixMilli(),
			MaxTimestamp: time.Now().UnixMilli(),
		})
		s.AddRows(mrs, defaultPrecisionBits)
		s.DebugFlush()

		// The next day metricIDs must appear in pendingNextDayMetricIDs cache.
		if s.pendingNextDayMetricIDs.Len() == 0 {
			t.Fatalf("unexpected pendingNextDayMetricIDs count: got 0, want > 0")
		}
		numNextDayMetricIDs := s.pendingNextDayMetricIDs.Len()
		// But not in the nextDayMetricIDs cache. The pending metrics will be
		// moved to it by a bg process a few seconds later.
		assertNextDayMetricIDs(t, s.nextDayMetricIDs.Load(), idbID, date, 0)

		// Wait for nextDayMetricIDs cache to populate.
		time.Sleep(15 * time.Second)
		synctest.Wait()

		// At this point, pending metricIDs must have been moved to
		// nextDayMetricIDs cache and the pendingNextDayMetricIDs must be empty.
		if got := s.pendingNextDayMetricIDs.Len(); got != 0 {
			t.Fatalf("unexpected pendingNextDayMetricIDs count: got %d, want 0", got)
		}
		// While the actual cache, must contain the exact number of metricIDs
		// that once were pending.
		assertNextDayMetricIDs(t, s.nextDayMetricIDs.Load(), idbID, date, numNextDayMetricIDs)

		// Close the storage to persist nextDayMetricIDs cache to a file.
		s.MustClose()
		// Open the storage again to ensure that the cache is populated
		// correctly.
		s = MustOpenStorage(t.Name(), OpenOptions{})
		if got := s.pendingNextDayMetricIDs.Len(); got != 0 {
			t.Fatalf("unexpected pendingNextDayMetricIDs count: got %d, want 0", got)
		}
		assertNextDayMetricIDs(t, s.nextDayMetricIDs.Load(), idbID, date, numNextDayMetricIDs)
		s.MustClose()

		// Advance the time by one day and open the storage.
		// Since the current date and the date in the cache file do not match,
		// nothing will be loaded into cache.
		time.Sleep(24 * time.Hour)
		date = uint64(time.Now().UnixMilli()) / msecPerDay
		s = MustOpenStorage(t.Name(), OpenOptions{})
		if got := s.pendingNextDayMetricIDs.Len(); got != 0 {
			t.Fatalf("unexpected pendingNextDayMetricIDs count: got %d, want 0", got)
		}
		assertNextDayMetricIDs(t, s.nextDayMetricIDs.Load(), idbID, date, 0)
		s.MustClose()
	})
}
