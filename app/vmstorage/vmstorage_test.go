package main

import (
	"math"
	"strconv"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/cgroup"
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
	}()

	maxConcurrentRequests := 2 * cgroup.AvailableCPUs()
	f := func(searchQueryLimit, storageMaxUniqueTimeseries, expect int) {
		t.Helper()
		*maxUniqueTimeseries = storageMaxUniqueTimeseries
		s := &storage.Storage{}
		vmstorage := newVMStorage(s, maxConcurrentRequests)
		maxMetrics := vmstorage.getMaxMetrics(searchQueryLimit)
		if maxMetrics != expect {
			t.Fatalf("unexpected max metrics: got %d, want %d", maxMetrics, expect)
		}
	}

	f(0, 1e6, 1e6)
	f(2e6, 0, 2e6)
	f(2e6, 1e6, 1e6)
}
