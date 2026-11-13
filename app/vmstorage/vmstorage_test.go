package vmstorage

import (
	"context"
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
	for _, tc := range []struct {
		name      string
		filter    string
		limit     int
		wantCount int
		wantError bool
	}{
		{"hit storage limit", "metric.*", 1000, 0, true},   // the number of matching series is 3, which exceeds the storage limit
		{"request limit unset", "metric.*", 0, 0, true},    // the number of matching series is 3, which exceeds the storage limit
		{"at storage limit", "metric[12]", 1000, 2, false}, // the number of matching series is 2.
		{"hit request limit", "metric[12]", 1, 0, true},    // the number of matching series is 2, which exceeds the request limit
		{"at request limit", "metric1", 1, 1, false},       // the number of matching series is 1
	} {
		t.Run(tc.name, func(t *testing.T) {
			sq := storage.NewSearchQuery(timestamp-1000, timestamp+1000, [][]storage.TagFilter{{{
				Value: []byte(tc.filter), IsRegexp: true,
			}}}, tc.limit)
			sr, count, err := vms.GetSearch(context.Background(), nil, sq)
			if err != nil {
				t.Fatalf("cannot initialize search: %s", err)
			}
			defer vms.PutSearch(sr)
			err = sr.Error()
			if tc.wantError {
				if err == nil || !strings.Contains(err.Error(), "the number of matching timeseries exceeds") {
					t.Fatalf("expected series limit error, got %v", err)
				}
			} else if err != nil {
				t.Fatalf("unexpected search error: %s", err)
			}
			if count != tc.wantCount {
				t.Fatalf("unexpected series count: got %d, want %d", count, tc.wantCount)
			}
		})
	}
}
