package vmstorage

import (
	"math"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/cgroup"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
)

func TestCalculateMaxMetricsLimitByResource(t *testing.T) {
	f := func(maxConcurrentRequest, remainingMemory, expect int) {
		t.Helper()
		maxMetricsLimit := calculateMaxUniqueTimeseries(maxConcurrentRequest, remainingMemory)
		if maxMetricsLimit != expect {
			t.Fatalf("unexpected max metrics limit: got %d, want %d", maxMetricsLimit, expect)
		}
	}

	// 64-bit architectures support memory sizes > 4GB.
	if strconv.IntSize == 64 {
		// 8 CPU & 32 GiB
		f(16, int(math.Round(32*1024*1024*1024*0.4)), 4294967)
		// 4 CPU & 32 GiB
		f(8, int(math.Round(32*1024*1024*1024*0.4)), 8589934)
	}

	// 2 CPU & 4 GiB
	f(4, int(math.Round(4*1024*1024*1024*0.4)), 2147483)

	// other edge cases
	f(0, int(math.Round(4*1024*1024*1024*0.4)), 2e9)
	f(4, 0, 0)

}

func TestGetMaxMetrics(t *testing.T) {
	originalMaxUniqueTimeSeries := *maxUniqueTimeseries
	defer func() {
		*maxUniqueTimeseries = originalMaxUniqueTimeSeries
		fs.MustRemoveDir(t.Name())
	}()

	maxConcurrentRequests := 2 * cgroup.AvailableCPUs()
	f := func(searchQueryLimit, storageMaxUniqueTimeseries, expect int) {
		t.Helper()
		*maxUniqueTimeseries = storageMaxUniqueTimeseries
		s := storage.MustOpenStorage(t.Name(), storage.OpenOptions{})
		vms := newVMStorage(s, maxConcurrentRequests, func(mrs []storage.MetricRow) {})
		defer vms.Stop()
		maxMetrics := vms.getMaxMetrics(searchQueryLimit)
		if maxMetrics != expect {
			t.Fatalf("unexpected max metrics: got %d, want %d", maxMetrics, expect)
		}
	}

	f(0, 1e6, 1e6)
	f(2e6, 0, 2e6)
	f(2e6, 1e6, 1e6)
}

func TestGetSearchMaxMetrics(t *testing.T) {
	s := storage.MustOpenStorage(t.TempDir(), storage.OpenOptions{})
	vms := &VMStorage{s: s, maxUniqueTimeseries: 2, maxUniqueTimeSeriesCalculated: 2}
	defer s.MustClose()
	timestamp := time.Now().UnixMilli()
	var rows []storage.MetricRow
	for _, name := range []string{"metric1", "metric2", "metric3"} {
		rows = append(rows, storage.MetricRow{
			MetricNameRaw: storage.MarshalMetricNameRaw(nil, []prompb.Label{{Name: "__name__", Value: name}}),
			Timestamp:     timestamp,
			Value:         1,
		})
	}
	s.AddRows(rows, 64)
	s.DebugFlush()
	f := func(sq *storage.SearchQuery, wantCount int, wantError bool) {
		t.Helper()
		sr, count, err := vms.GetSearch(t.Context(), nil, sq)
		if err != nil {
			t.Fatalf("cannot initialize search: %s", err)
		}
		defer vms.PutSearch(sr)
		err = sr.Error()
		if wantError {
			if err == nil || !strings.Contains(err.Error(), "the number of matching timeseries exceeds") {
				t.Fatalf("expected series limit error, got %v", err)
			}
		} else if err != nil {
			t.Fatalf("unexpected search error: %s", err)
		}
		if count != wantCount {
			t.Fatalf("unexpected series count: got %d, want %d", count, wantCount)
		}
	}

	// 3 matching series exceed the storage limit.
	sq := storage.NewSearchQuery(timestamp-1000, timestamp+1000, [][]storage.TagFilter{{{
		Value: []byte("metric.*"), IsRegexp: true,
	}}}, 1000)
	f(sq, 0, true)

	// An unset request limit falls back to the storage limit.
	sq.MaxMetrics = 0
	f(sq, 0, true)

	// 2 matching series are allowed at the storage limit.
	sq.TagFilterss[0][0].Value = []byte("metric[12]")
	sq.MaxMetrics = 1000
	f(sq, 2, false)

	// 2 matching series exceed the request limit，which is the lower one.
	sq.MaxMetrics = 1
	f(sq, 0, true)

	// 1 matching series is allowed at the request limit.
	sq.TagFilterss[0][0].Value = []byte("metric1")
	f(sq, 1, false)
}
