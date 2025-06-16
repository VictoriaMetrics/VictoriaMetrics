//go:build goexperiment.synctest

package storage

import (
	"fmt"
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
		const (
			accountID  = 0
			projectID  = 0
			numMetrics = 10
		)
		date := uint64(tr.MinTimestamp) / msecPerDay
		idb := s.tb.MustGetIndexDB(tr.MinTimestamp)
		defer s.tb.PutIndexDB(idb)
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
			kb.B = marshalCommonPrefix(kb.B[:0], nsPrefixDateTagToMetricIDs, accountID, projectID)
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

		tfsAll := NewTagFilters(accountID, projectID)
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
