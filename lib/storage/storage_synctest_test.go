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
		idbPrev, idbCurr := s.getPrevAndCurrIndexDBs()
		defer s.putPrevAndCurrIndexDBs(idbPrev, idbCurr)
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

			idbCurr.tb.AddItems(ii.Items)
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
		if got, want := m.IndexDBMetrics.MissingTSIDsForMetricID, uint64(0); got != want {
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
		if got, want := m.IndexDBMetrics.MissingTSIDsForMetricID, uint64(numMetrics); got != want {
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
		idbPrev, idbCurr := s.getPrevAndCurrIndexDBs()
		defer s.putPrevAndCurrIndexDBs(idbPrev, idbCurr)
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

			idbCurr.tb.AddItems(ii.Items)
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
		if got, want := m.IndexDBMetrics.MissingMetricNamesForMetricID, uint64(0); got != want {
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
		if got, want := m.IndexDBMetrics.MissingMetricNamesForMetricID, uint64(numMetrics); got != want {
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
	f(OpenOptions{Retention: time.Hour * 24, IDBPrefillStart: time.Hour}, time.Hour)

	// Zero IDBPrefillStart option should fallback to 1 hour prefill start:
	f(OpenOptions{Retention: time.Hour * 24, IDBPrefillStart: 0}, time.Hour)

	// Test a custom prefill duration: 2h:
	// VictoriaMetrics starts prefill indexDB at 2 A.M UTC, while indexDB rotates at 4 A.M UTC.
	f(OpenOptions{Retention: time.Hour * 24, IDBPrefillStart: 2 * time.Hour}, 2*time.Hour)
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
		// But first, advance the time 1m before the last hour to fill the
		// currHourMetricIDs cache.
		//
		// currHourMetricIDs cache plays important role in prefilling the index
		// with next day entries. In order for timeseries to be added to the
		// next day index, its metricID must be in that cache. The metricID is
		// added to that cache when the timeseries sample is added to the
		// storage. The only problem is that it happens asynchronously, i.e.
		// they aren't visible right away. First, Storage.add() adds the
		// metricID to the s.pendingHourEntries, and only after 11 seconds, a
		// background task copies those pending entries to currHourMetricIDs
		// cache.
		//
		// Thus, the testing code needs to insert a timeseries twice:
		// first time - to register it in the currHourMetricIDs, and second time
		// (after some time) to actually test the prefill.
		time.Sleep(23*time.Hour - 1*time.Minute) // 2000-01-01T22:59:00Z
		mrs0 := testGenerateMetricRowsWithPrefix(rng, numSeries, "metric0", TimeRange{
			MinTimestamp: time.Now().Add(-15 * time.Minute).UnixMilli(),
			MaxTimestamp: time.Now().UnixMilli(),
		})
		s := MustOpenStorage(t.Name(), OpenOptions{})
		s.AddRows(mrs0, defaultPrecisionBits)
		s.DebugFlush()
		if got, want := countMetricIDs(t, s, "metric0", today), numSeries; got != want {
			t.Fatalf("unexpected metric id count for today: got %d, want %d", got, want)
		}
		if got, want := countMetricIDs(t, s, "metric0", nextDay), 0; got != want {
			t.Fatalf("unexpected metric id count for next day: got %d, want %d", got, want)
		}
		// Give some time for the background process to update currHourMetricIDs
		// cache with metricIDs of samples that have just been inserted.
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

		// Close the storage and reopen it 15m later instead of keeping it open
		// and waiting. This is to make the test faster. Storage has a lot of
		// background tasks that are activated every 1-10 seconds and synctest's
		// time.Sleep() will wake them up many times. Closing storage before
		// sleeping seems to eliminate this.
		//
		// At 23:15 the prefill must work. Again, in order to make it to the
		// next day the timeseries must be active, i.e. 1) we have seen at least
		// one same for that timeseries within the current hour and 2) the
		// timestamp of that same also was within the current hour.
		//
		// Both mrs1 and mrs2 samples have been seen within the current hour.
		// However, the mrs1 timestamp are not within the current hour and
		// therefore the next day will not be prefilled with the corresponding
		// timeseries.
		s.MustClose()
		time.Sleep(15 * time.Minute) // 2000-01-01T23:15:00Z
		s = MustOpenStorage(t.Name(), OpenOptions{})
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
		// mrs1 and mrs2 have been inserted but their metricIDs are not in
		// currHourMetricIDs cache yet. Therefore no next day index entries will
		// be created.
		if got, want := countMetricIDs(t, s, "metric1", today), numSeries; got != want {
			t.Fatalf("unexpected metric id count for today: got %d, want %d", got, want)
		}
		if got, want := countMetricIDs(t, s, "metric1", nextDay), 0; got != want {
			t.Fatalf("unexpected metric id count for next day: got %d, want %d", got, want)
		}
		if got, want := countMetricIDs(t, s, "metric2", today), numSeries; got != want {
			t.Fatalf("unexpected metric id count for today: got %d, want %d", got, want)
		}
		if got, want := countMetricIDs(t, s, "metric2", nextDay), 0; got != want {
			t.Fatalf("unexpected metric id count for next day: got %d, want %d", got, want)
		}
		// Give some time for the background process to update currHourMetricIDs
		// cache with metricIDs of samples that have just been inserted.
		time.Sleep(15 * time.Second)
		synctest.Wait()
		s.AddRows(mrs1, defaultPrecisionBits)
		s.AddRows(mrs2, defaultPrecisionBits)
		s.DebugFlush()
		// mrs1 and mrs2 have been inserted again and their metricIDs are now in
		// currHourMetricIDs cache. Therefore some mrs2 next day index entries
		// will be created.
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

		// Close the storage and reopen it at 23:30.
		s.MustClose()
		time.Sleep(15 * time.Minute) // 2000-01-01T23:30:15Z
		s = MustOpenStorage(t.Name(), OpenOptions{})
		mrs3 := testGenerateMetricRowsWithPrefix(rng, numSeries, "metric3", TimeRange{
			MinTimestamp: time.Now().Add(-15 * time.Minute).UnixMilli(),
			MaxTimestamp: time.Now().UnixMilli(),
		})
		s.AddRows(mrs3, defaultPrecisionBits)
		s.DebugFlush()
		if got, want := countMetricIDs(t, s, "metric3", today), numSeries; got != want {
			t.Fatalf("unexpected metric id count for today: got %d, want %d", got, want)
		}
		if got, want := countMetricIDs(t, s, "metric3", nextDay), 0; got != want {
			t.Fatalf("unexpected metric id count for next day: got %d, want %d", got, want)
		}
		// Give some time for the background process to update currHourMetricIDs
		// cache with metricIDs of samples that have just been inserted.
		time.Sleep(15 * time.Second)
		synctest.Wait()
		s.AddRows(mrs3, defaultPrecisionBits)
		s.DebugFlush()
		if got, want := countMetricIDs(t, s, "metric3", today), numSeries; got != want {
			t.Fatalf("unexpected metric id count for today: got %d, want %d", got, want)
		}
		// Since we are now closer to midnight than we were at 23:15, more next
		// day entries must be created.
		got30min := countMetricIDs(t, s, "metric3", nextDay)
		if got30min < got15min {
			t.Fatalf("unexpected metric id count for next day: got %d, want > %d", got30min, got15min)
		}

		// Close the storage and reopen it at 23:45.
		s.MustClose()
		time.Sleep(15 * time.Minute) // 2000-01-01T23:45:30Z
		s = MustOpenStorage(t.Name(), OpenOptions{})
		mrs4 := testGenerateMetricRowsWithPrefix(rng, numSeries, "metric4", TimeRange{
			MinTimestamp: time.Now().Add(-15 * time.Minute).UnixMilli(),
			MaxTimestamp: time.Now().UnixMilli(),
		})
		s.AddRows(mrs4, defaultPrecisionBits)
		s.DebugFlush()
		if got, want := countMetricIDs(t, s, "metric4", nextDay), 0; got != want {
			t.Fatalf("unexpected metric id count for next day: got %d, want %d", got, want)
		}
		// Give some time for the background process to update currHourMetricIDs
		// cache with metricIDs of samples that have just been inserted.
		time.Sleep(15 * time.Second)
		synctest.Wait()
		s.AddRows(mrs4, defaultPrecisionBits)
		s.DebugFlush()
		// Since we are now closer to midnight than we were at 23:30, more next
		// day entries must be created.
		got45min := countMetricIDs(t, s, "metric4", nextDay)
		if got45min < got30min {
			t.Fatalf("unexpected metric id count for next day: got %d, want > %d", got45min, got30min)
		}

		s.MustClose()
	})
}

func TestStorageMustLoadNextDayMetricIDs(t *testing.T) {
	defer testRemoveAll(t)

	assertNextDayMetricIDs := func(t *testing.T, gotNextDayMetricIDs *nextDayMetricIDs, wantGen, wantDate uint64, wantLen int) {
		t.Helper()

		if got, want := gotNextDayMetricIDs.generation, wantGen; got != want {
			t.Fatalf("unexpected nextDayMetricIDs idb generation: got %d, want %d", got, want)
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
		date := uint64(time.Now().UnixMilli() / msecPerDay)

		const numSeries = 1000
		s := MustOpenStorage(t.Name(), OpenOptions{})
		idbPrev, idbCurr := s.getPrevAndCurrIndexDBs()
		genCurr := idbCurr.generation
		s.putPrevAndCurrIndexDBs(idbPrev, idbCurr)

		rng := rand.New(rand.NewSource(1))
		mrs := testGenerateMetricRowsWithPrefix(rng, numSeries, "metric", TimeRange{
			MinTimestamp: time.Now().Add(-15 * time.Minute).UnixMilli(),
			MaxTimestamp: time.Now().UnixMilli(),
		})
		s.AddRows(mrs, defaultPrecisionBits)
		s.DebugFlush()

		// After the initial ingestion, the metricIDs are not in
		// currHourMetricIDs cache yet, so no timeseries will be registered in
		// next day index.
		if got := s.pendingNextDayMetricIDs.Len(); got != 0 {
			t.Fatalf("unexpected pendingNextDayMetricIDs count: got %d, want 0", got)
		}
		assertNextDayMetricIDs(t, s.nextDayMetricIDs.Load(), genCurr, date, 0)

		// Wait for currHourMetricIDs cache to populate and ingest the same data
		// again.
		time.Sleep(15 * time.Second)
		synctest.Wait()
		s.AddRows(mrs, defaultPrecisionBits)
		s.DebugFlush()

		// The next day metricIDs must now appear in pendingNextDayMetricIDs cache.
		if s.pendingNextDayMetricIDs.Len() == 0 {
			t.Fatalf("unexpected pendingNextDayMetricIDs count: got 0, want > 0")
		}
		numNextDayMetricIDs := s.pendingNextDayMetricIDs.Len()
		// But not in the nextDayMetricIDs cache. The pending metrics will be
		// moved to it by a bg process a few seconds later.
		assertNextDayMetricIDs(t, s.nextDayMetricIDs.Load(), genCurr, date, 0)

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
		assertNextDayMetricIDs(t, s.nextDayMetricIDs.Load(), genCurr, date, numNextDayMetricIDs)

		// Close the storage to persist nextDayMetricIDs cache to a file.
		s.MustClose()
		// Open the storage again to enrure that the cache is populated
		// correctly.
		s = MustOpenStorage(t.Name(), OpenOptions{})
		if got := s.pendingNextDayMetricIDs.Len(); got != 0 {
			t.Fatalf("unexpected pendingNextDayMetricIDs count: got %d, want 0", got)
		}
		assertNextDayMetricIDs(t, s.nextDayMetricIDs.Load(), genCurr, date, numNextDayMetricIDs)

		// Try loading the cache file contents for a different indexDB.
		genOther := genCurr + 1
		gotNextDayMetricIDs := s.mustLoadNextDayMetricIDs(genOther, date)
		assertNextDayMetricIDs(t, gotNextDayMetricIDs, genOther, date, 0)

		// Try loading the cache file contents for a different date.
		dateOther := date + 1
		gotNextDayMetricIDs = s.mustLoadNextDayMetricIDs(genCurr, dateOther)
		assertNextDayMetricIDs(t, gotNextDayMetricIDs, genCurr, dateOther, 0)

		s.MustClose()
	})
}
