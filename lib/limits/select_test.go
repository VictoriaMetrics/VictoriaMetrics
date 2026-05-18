package limits

import (
	"math"
	"runtime"
	"testing"
)

func TestCalculateMaxUniqueTimeseries(t *testing.T) {
	f := func(maxConcurrentRequests, remainingMemory, want int) {
		t.Helper()
		got := calculateMaxUniqueTimeseries(maxConcurrentRequests, remainingMemory)
		if got != want {
			t.Fatalf("unexpected maxUniqueTimeseries: got %d, want %d", got, want)
		}
	}

	// Skip when GOARCH=386
	if runtime.GOARCH != "386" {
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

func TestMaxMetrics(t *testing.T) {
	originalMaxUniqueTimeseries := *maxUniqueTimeseries
	defer func() {
		*maxUniqueTimeseries = originalMaxUniqueTimeseries
	}()
	f := func(searchQueryLimit, flagLimit, want int) {
		t.Helper()
		*maxUniqueTimeseries = flagLimit
		got := MaxMetrics(searchQueryLimit)
		if got != want {
			t.Fatalf("unexpected maxMetrics: got %d, want %d", got, want)
		}
	}

	f(0, 1e6, 1e6)
	f(2e6, 0, 2e6)
	f(2e6, 1e6, 1e6)
}
