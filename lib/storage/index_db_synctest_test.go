//go:build goexperiment.synctest

package storage

import (
	"slices"
	"testing"
	"testing/synctest"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/uint64set"
	"github.com/google/go-cmp/cmp"
)

func TestIndexDB_MetricIDsNotMappedToTSIDsAreDeleted(t *testing.T) {
	defer testRemoveAll(t)

	keys := func(missingMetricIDs map[uint64]uint64) []uint64 {
		keys := []uint64{}
		for k := range missingMetricIDs {
			keys = append(keys, k)
		}
		slices.Sort(keys)
		return keys
	}

	synctest.Test(t, func(t *testing.T) {
		s := MustOpenStorage(t.Name(), OpenOptions{})
		defer s.MustClose()
		idbPrev, idbCurr := s.getPrevAndCurrIndexDBs()
		defer s.putPrevAndCurrIndexDBs(idbPrev, idbCurr)

		type want struct {
			missingMetricIDs        []uint64
			missingTSIDsForMetricID uint64
			deletedMetricIDs        []uint64
		}
		assertGetTSIDsFromMetricIDs := func(metricIDs []uint64, want want) {
			t.Helper()
			tsids, err := idbCurr.getTSIDsFromMetricIDs(nil, metricIDs, noDeadline)
			if err != nil {
				t.Fatalf("getTSIDsFromMetricIDs() failed unexpectedly: %v", err)
			}
			if diff := cmp.Diff([]TSID{}, tsids); diff != "" {
				t.Fatalf("unexpected tsids (-want, +got):\n%s", diff)
			}
			missingMetricIDs := keys(s.missingMetricIDs)
			if diff := cmp.Diff(want.missingMetricIDs, missingMetricIDs); diff != "" {
				t.Fatalf("unexpected tsids (-want, +got):\n%s", diff)
			}
			if got, want := idbCurr.missingTSIDsForMetricID.Load(), want.missingTSIDsForMetricID; got != want {
				t.Fatalf("unexpected missingTSIDsForMetricID metric value: got %d, want %d", got, want)
			}
			wantDeletedMetricIDs := &uint64set.Set{}
			wantDeletedMetricIDs.AddMulti(want.deletedMetricIDs)
			if !s.getDeletedMetricIDs().Equal(wantDeletedMetricIDs) {
				t.Fatalf("deleted metricIDs set is different from %v: %v", want.deletedMetricIDs, s.getDeletedMetricIDs().AppendTo(nil))
			}
		}

		metricIDs := []uint64{1, 2, 3, 4}

		// These metricIDs are not mapped to the corresponding TSIDs so they are
		// expected to be placed in missingMetricIDs cache but not be deleted yet.
		assertGetTSIDsFromMetricIDs(metricIDs, want{
			missingMetricIDs:        metricIDs,
			missingTSIDsForMetricID: 0,
			deletedMetricIDs:        []uint64{},
		})

		// If we repeat search after one minute, the get soft-deleted and a
		// corresponding metric is incremented. The metric will remain in
		// missingMetricIDs cache for another minute.
		time.Sleep(61 * time.Second)
		synctest.Wait()
		assertGetTSIDsFromMetricIDs(metricIDs, want{
			missingMetricIDs:        metricIDs,
			missingTSIDsForMetricID: uint64(len(metricIDs)),
			deletedMetricIDs:        metricIDs,
		})
	})
}
