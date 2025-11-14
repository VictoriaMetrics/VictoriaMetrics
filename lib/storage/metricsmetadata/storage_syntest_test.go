//go:build goexperiment.synctest

package metricsmetadata

import (
	"testing"
	"testing/synctest"
	"time"

	"github.com/google/go-cmp/cmp"
)

func TestWriteEviction(t *testing.T) {

	synctest.Test(t, func(t *testing.T) {
		s := NewStorage(256 * bucketsCount)
		defer s.MustClose()

		rows := []Row{
			{MetricFamilyName: []byte("metric_name_1"), Help: []byte("some useless help message"), Unit: []byte("seconds")},
			{MetricFamilyName: []byte("metric_name_2"), Help: []byte("some useless help message"), Unit: []byte("seconds")},
			{MetricFamilyName: []byte("metric_name_3"), Help: []byte("some useless help message"), Unit: []byte("seconds")},
			{MetricFamilyName: []byte("metric_name_4"), Help: []byte("some useless help message"), Unit: []byte("seconds")},
		}
		s.Add(rows)

		got := s.Get(-1, "")
		expected := []*Row{
			{MetricFamilyName: []byte("metric_name_2"), Help: []byte("some useless help message"), Unit: []byte("seconds")},
			{MetricFamilyName: []byte("metric_name_1"), Help: []byte("some useless help message"), Unit: []byte("seconds")},
			{MetricFamilyName: []byte("metric_name_4"), Help: []byte("some useless help message"), Unit: []byte("seconds")},
		}

		sortRows(expected)
		if diff := cmp.Diff(got, expected, rowCmpOpts); len(diff) > 0 {
			t.Errorf("unexpected rows (-want, +got):\n%s", diff)
		}

		// evict all previous records by max storage size
		rows = []Row{
			{MetricFamilyName: []byte("metric_name_6"), Help: []byte("some useless help message"), Unit: []byte("seconds")},
			{MetricFamilyName: []byte("metric_name_7"), Help: []byte("some useless help message"), Unit: []byte("seconds")},
			{MetricFamilyName: []byte("metric_name_9"), Help: []byte("some useless help message"), Unit: []byte("seconds")},
			{MetricFamilyName: []byte("metric_name_10"), Help: []byte("some useless help message"), Unit: []byte("seconds")},
			{MetricFamilyName: []byte("metric_name_11"), Help: []byte("some useless help message"), Unit: []byte("seconds")},
		}
		s.Add(rows)
		got = s.Get(-1, "")

		expected = []*Row{
			{MetricFamilyName: []byte("metric_name_6"), Help: []byte("some useless help message"), Unit: []byte("seconds")},
			{MetricFamilyName: []byte("metric_name_7"), Help: []byte("some useless help message"), Unit: []byte("seconds")},
			{MetricFamilyName: []byte("metric_name_9"), Help: []byte("some useless help message"), Unit: []byte("seconds")},
			{MetricFamilyName: []byte("metric_name_10"), Help: []byte("some useless help message"), Unit: []byte("seconds")},
			{MetricFamilyName: []byte("metric_name_11"), Help: []byte("some useless help message"), Unit: []byte("seconds")},
		}
		sortRows(expected)
		if diff := cmp.Diff(got, expected, rowCmpOpts); len(diff) > 0 {
			t.Errorf("unexpected rows (-want, +got):\n%s", diff)
		}

		// evict all records based on expire duration
		time.Sleep(metadataExpireDuration + time.Hour)
		synctest.Wait()
		got = s.Get(-1, "")
		expected = expected[:0]
		if diff := cmp.Diff(got, expected, rowCmpOpts); len(diff) > 0 {
			t.Errorf("unexpected rows (-want, +got):\n%s", diff)
		}
		var sm MetadataStorageMetrics
		s.UpdateMetrics(&sm)
		if sm.CurrentSizeBytes != 0 {
			t.Fatalf("unexpected size: %d want 0", sm.CurrentSizeBytes)
		}
		if sm.ItemsCurrent != 0 {
			t.Fatalf("unexpected items count: %d want 0", sm.ItemsCurrent)
		}
	})

}
