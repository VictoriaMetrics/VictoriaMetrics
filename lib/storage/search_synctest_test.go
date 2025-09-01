//go:build goexperiment.synctest

package storage

import (
	"math/rand"
	"testing"
	"testing/synctest"
	"time"
)

func TestSearch_metricNamesIndifferentIndexDBs(t *testing.T) {
	defer testRemoveAll(t)

	synctest.Test(t, func(t *testing.T) {
		const numMetrics = 10
		tr := TimeRange{
			MinTimestamp: time.Now().UnixMilli(),
			MaxTimestamp: time.Now().Add(23 * time.Hour).UnixMilli(),
		}
		rng := rand.New(rand.NewSource(1))
		mrs := testGenerateMetricRowsWithPrefix(rng, numMetrics, "metric", tr)
		s := MustOpenStorage(t.Name(), OpenOptions{})
		defer s.MustClose()
		s.AddRows(mrs[:numMetrics/2], defaultPrecisionBits)
		// Rotate the indexDB to ensure that the index for the entire dataset is
		// split across prev and curr indexDBs.
		s.mustRotateIndexDB(time.Now())
		s.AddRows(mrs[numMetrics/2:], defaultPrecisionBits)
		s.DebugFlush()

		tfs := NewTagFilters()
		if err := tfs.Add(nil, []byte(".*"), false, true); err != nil {
			t.Fatalf("Could not add tag filter: %v", err)
		}

		// Search for the first time. If the search logic tracks missing
		// metricID->metricName mappings (using Storage.wasMetricIDMissingBefore),
		// then half of the IDs will be recorded as missing the metricName even
		// though all the mappings are found. This is possible when metricIDs
		// from prev indexDB are searched in curr indexDB.
		if err := testAssertSearchResult(s, tr, tfs, mrs); err != nil {
			t.Fatalf("unexpected search result: %v", err)
		}

		var m Metrics
		s.UpdateMetrics(&m)
		if got, want := m.IndexDBMetrics.MissingTSIDsForMetricID, uint64(0); got != want {
			t.Fatalf("unexpected MissingTSIDsForMetricID count: got %d, want %d", got, want)
		}

		// Sleep > 60 seconds to go past the time interval after which the
		// metrics will be considered `missing before` should they again
		// participate in search.
		time.Sleep(61 * time.Second)
		synctest.Wait()

		// Search again. If the search logic tracks missing metricID-metricName
		// mappings, then the half of the metricIDs will be deleted. The search result
		// must still be full.
		if err := testAssertSearchResult(s, tr, tfs, mrs); err != nil {
			t.Fatalf("unexpected search result: %v", err)
		}

		s.UpdateMetrics(&m)
		if got, want := m.IndexDBMetrics.MissingTSIDsForMetricID, uint64(0); got != want {
			t.Fatalf("unexpected MissingTSIDsForMetricID count: got %d, want %d", got, want)
		}

		// Search again. Now that the metricIDs have been deleted, the search
		// result should only contain half of records.
		// This is not the case however, because the underlying search logic does
		// not track metricID-metricName mappings. Should this logic start
		// tracking them, it should be written so that this test does not fail.
		if err := testAssertSearchResult(s, tr, tfs, mrs); err != nil {
			t.Fatalf("unexpected search result: %v", err)
		}

		s.UpdateMetrics(&m)
		if got, want := m.IndexDBMetrics.MissingTSIDsForMetricID, uint64(0); got != want {
			t.Fatalf("unexpected MissingTSIDsForMetricID count: got %d, want %d", got, want)
		}
	})
}
