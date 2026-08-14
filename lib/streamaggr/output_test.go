package streamaggr

import (
	"testing"

	"github.com/VictoriaMetrics/metrics"
)

func TestAggrOutputsStateSizeAndItemsCount(t *testing.T) {
	ao := &aggrOutputs{
		configs: []aggrConfig{newAvgAggrConfig(), newCountSeriesAggrConfig()},
	}

	if n := ao.stateItemsCount(); n != 0 {
		t.Fatalf("unexpected items count for empty state; got %d; want 0", n)
	}
	for i := range ao.configs {
		if n := ao.stateSizeBytes(i); n != 0 {
			t.Fatalf("unexpected size for empty state at index %d; got %d; want 0", i, n)
		}
	}

	const future = int64(1) << 62

	newEntry := func() *aggrValues {
		av := &aggrValues{
			blue:           make([]aggrValue, len(ao.configs)),
			deleteDeadline: future,
		}
		for idx, ac := range ao.configs {
			av.blue[idx] = ac.getValue(nil)
		}
		return av
	}

	e1 := newEntry()
	e2 := newEntry()
	// Grow the count_series (index 1) state for e2 only, so its size differs from e1's.
	e2.blue[1].pushSample(ao.configs[1], &pushSample{}, "series-a", future)
	e2.blue[1].pushSample(ao.configs[1], &pushSample{}, "series-b", future)

	ao.m.Store("key1", e1)
	ao.m.Store("key2", e2)

	if n := ao.stateItemsCount(); n != 2 {
		t.Fatalf("unexpected items count; got %d; want 2", n)
	}

	avgSize := ao.stateSizeBytes(0)
	wantAvgSize := e1.blue[0].sizeBytes() + e2.blue[0].sizeBytes()
	if avgSize != wantAvgSize {
		t.Fatalf("unexpected avg size; got %d; want %d", avgSize, wantAvgSize)
	}

	countSeriesSize := ao.stateSizeBytes(1)
	wantCountSeriesSize := e1.blue[1].sizeBytes() + e2.blue[1].sizeBytes()
	if countSeriesSize != wantCountSeriesSize {
		t.Fatalf("unexpected count_series size; got %d; want %d", countSeriesSize, wantCountSeriesSize)
	}
	if e2.blue[1].sizeBytes() <= e1.blue[1].sizeBytes() {
		t.Fatalf("expected count_series state with tracked series to be bigger than an empty one; got=%d empty=%d",
			e2.blue[1].sizeBytes(), e1.blue[1].sizeBytes())
	}

	// Entries marked as deleted must be excluded from both items count and size.
	e1.deleteDeadline = -1
	if n := ao.stateItemsCount(); n != 1 {
		t.Fatalf("unexpected items count after marking an entry deleted; got %d; want 1", n)
	}
	if n := ao.stateSizeBytes(1); n != e2.blue[1].sizeBytes() {
		t.Fatalf("unexpected count_series size after marking an entry deleted; got %d; want %d", n, e2.blue[1].sizeBytes())
	}
}

func TestAggrOutputsStateSizeBytesWithSharedState(t *testing.T) {
	ao := &aggrOutputs{
		configs:        []aggrConfig{newCountSeriesAggrConfig()},
		useSharedState: true,
	}

	const future = int64(1) << 62
	blue := ao.configs[0].getValue(nil)
	green := ao.configs[0].getValue(blue.state())
	av := &aggrValues{
		blue:           []aggrValue{blue},
		green:          []aggrValue{green},
		deleteDeadline: future,
	}
	ao.m.Store("key1", av)

	blue.pushSample(ao.configs[0], &pushSample{}, "series-a", future)
	green.pushSample(ao.configs[0], &pushSample{}, "series-b", future)

	got := ao.stateSizeBytes(0)
	want := blue.sizeBytes() + green.sizeBytes()
	if got != want {
		t.Fatalf("unexpected size with shared state; got %d; want %d", got, want)
	}
}

func TestAggrValueSizeBytes(t *testing.T) {
	ms := metrics.NewSet()
	const future = int64(1) << 62

	f := func(name string, ac aggrConfig, key string) {
		t.Helper()
		t.Run(name, func(t *testing.T) {
			av := ac.getValue(nil)
			emptySize := av.sizeBytes()
			if emptySize == 0 {
				t.Fatalf("expected non-zero sizeBytes() for a freshly created value")
			}
			av.pushSample(ac, &pushSample{value: 1, timestamp: 1000}, key, future)
			if n := av.sizeBytes(); n == 0 {
				t.Fatalf("expected non-zero sizeBytes() after pushSample()")
			}
		})
	}

	f("avg", newAvgAggrConfig(), "s1")
	f("count_samples", newCountSamplesAggrConfig(), "s1")
	f("count_series", newCountSeriesAggrConfig(), "s1")
	f("last", newLastAggrConfig(), "s1")
	f("max", newMaxAggrConfig(), "s1")
	f("min", newMinAggrConfig(), "s1")
	f("stddev", newStddevAggrConfig(), "s1")
	f("stdvar", newStdvarAggrConfig(), "s1")
	f("sum_samples", newSumSamplesAggrConfig(false), "s1")
	f("unique_samples", newUniqueSamplesAggrConfig(), "s1")
	f("quantiles", newQuantilesAggrConfig([]float64{0.5}), "s1")
	f("histogram_bucket", newHistogramBucketAggrConfig(), "s1")
	f("increase", newIncreaseAggrConfig(ms, `output="increase"`, 0, false), "s1")
	f("total", newTotalAggrConfig(ms, `output="total"`, 0, false), "s1")
	f("rate_sum", newRateAggrConfig(ms, `output="rate_sum"`, false), "s1")
}
