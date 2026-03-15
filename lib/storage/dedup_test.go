package storage

import (
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/decimal"
)

func TestNeedsDedup(t *testing.T) {
	f := func(interval int64, timestamps []int64, expectedResult bool) {
		t.Helper()
		result := needsDedup(timestamps, interval)
		if result != expectedResult {
			t.Fatalf("unexpected result for needsDedup(%d, %d); got %v; want %v", timestamps, interval, result, expectedResult)
		}
	}
	f(-1, nil, false)
	f(-1, []int64{1}, false)
	f(0, []int64{1, 2}, false)
	f(10, []int64{1}, false)
	f(10, []int64{1, 2}, true)
	f(10, []int64{9, 11}, false)
	f(10, []int64{10, 11}, false)
	f(10, []int64{0, 10, 11}, false)
	f(10, []int64{9, 10}, true)
	f(10, []int64{0, 10, 19}, false)
	f(10, []int64{9, 19}, false)
	f(10, []int64{0, 11, 19}, true)
	f(10, []int64{0, 11, 20}, true)
	f(10, []int64{0, 11, 21}, false)
	f(10, []int64{0, 19}, false)
	f(10, []int64{0, 30, 40}, false)
	f(10, []int64{0, 31, 40}, true)
	f(10, []int64{0, 31, 41}, false)
	f(10, []int64{0, 31, 49}, false)
}

func equalWithNans(a, b []float64) bool {
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if decimal.IsStaleNaN(v) && decimal.IsStaleNaN(b[i]) {
			continue
		}
		if v != b[i] {
			return false
		}
	}
	return true
}

func TestDeduplicateSamplesWithIdenticalTimestamps(t *testing.T) {
	f := func(scrapeInterval time.Duration, timestamps []int64, values []float64, timestampsExpected []int64, valuesExpected []float64) {
		t.Helper()
		timestampsCopy := append([]int64{}, timestamps...)

		dedupInterval := scrapeInterval.Milliseconds()
		timestampsCopy, values = DeduplicateSamples(timestampsCopy, values, dedupInterval)
		if !reflect.DeepEqual(timestampsCopy, timestampsExpected) {
			t.Fatalf("invalid DeduplicateSamples(%v) timestamps;\ngot\n%v\nwant\n%v", timestamps, timestampsCopy, timestampsExpected)
		}
		if !equalWithNans(values, valuesExpected) {
			t.Fatalf("invalid DeduplicateSamples(%v) values;\ngot\n%v\nwant\n%v", timestamps, values, valuesExpected)
		}

		// Verify that the second call to DeduplicateSamples doesn't modify samples.
		valuesCopy := append([]float64{}, values...)
		timestampsCopy, valuesCopy = DeduplicateSamples(timestampsCopy, valuesCopy, dedupInterval)
		if !reflect.DeepEqual(timestampsCopy, timestampsExpected) {
			t.Fatalf("invalid DeduplicateSamples(%v) timestamps for the second call;\ngot\n%v\nwant\n%v", timestamps, timestampsCopy, timestampsExpected)
		}
		if !equalWithNans(valuesCopy, values) {
			t.Fatalf("invalid DeduplicateSamples(%v) values for the second call;\ngot\n%v\nwant\n%v", timestamps, values, valuesCopy)
		}
	}
	f(time.Second, []int64{1000, 1000}, []float64{2, 1}, []int64{1000}, []float64{2})
	f(time.Second, []int64{1001, 1001}, []float64{2, 1}, []int64{1001}, []float64{2})
	f(time.Second, []int64{1000, 1001, 1001, 1001, 2001}, []float64{1, 2, 5, 3, 0}, []int64{1000, 1001, 2001}, []float64{1, 5, 0})

	// verify decimal.StaleNaN is NOT preferred on timestamp conflicts
	// see https://github.com/VictoriaMetrics/VictoriaMetrics/issues/10196
	f(time.Second, []int64{1000, 1000}, []float64{2, decimal.StaleNaN}, []int64{1000}, []float64{2})
	f(time.Second, []int64{1000, 1000}, []float64{decimal.StaleNaN, 2}, []int64{1000}, []float64{2})
	f(time.Second, []int64{1000, 1000, 1000}, []float64{1, decimal.StaleNaN, 2}, []int64{1000}, []float64{2})
	// compare with Inf values
	f(time.Second, []int64{1000, 1000}, []float64{math.Inf(1), decimal.StaleNaN}, []int64{1000}, []float64{math.Inf(1)})
	f(time.Second, []int64{1000, 1000, 1000}, []float64{math.Inf(1), decimal.StaleNaN, math.Inf(-1)}, []int64{1000}, []float64{math.Inf(1)})
	f(time.Second, []int64{1000, 1000, 2000, 2000}, []float64{1, decimal.StaleNaN, 2, 3}, []int64{1000, 2000}, []float64{1, 3})
	f(time.Second, []int64{1000, 1000, 2000, 2000}, []float64{decimal.StaleNaN, decimal.StaleNaN, 2, 3}, []int64{1000, 2000}, []float64{decimal.StaleNaN, 3})
	f(time.Second, []int64{1000, 1000, 1000, 2000, 2000}, []float64{1, decimal.StaleNaN, 6, 2, 3}, []int64{1000, 2000}, []float64{6, 3})
}

func TestDeduplicateSamplesDuringMergeWithIdenticalTimestamps(t *testing.T) {
	f := func(scrapeInterval time.Duration, timestamps, values, timestampsExpected, valuesExpected []int64) {
		t.Helper()
		timestampsCopy := append([]int64{}, timestamps...)

		dedupInterval := scrapeInterval.Milliseconds()
		timestampsCopy, values = deduplicateSamplesDuringMerge(timestampsCopy, values, dedupInterval)
		if !reflect.DeepEqual(timestampsCopy, timestampsExpected) {
			t.Fatalf("invalid deduplicateSamplesDuringMerge(%v) timestamps;\ngot\n%v\nwant\n%v", timestamps, timestampsCopy, timestampsExpected)
		}
		if !reflect.DeepEqual(values, valuesExpected) {
			t.Fatalf("invalid deduplicateSamplesDuringMerge(%v) values;\ngot\n%v\nwant\n%v", timestamps, values, valuesExpected)
		}

		// Verify that the second call to deduplicateSamplesDuringMerge doesn't modify samples.
		valuesCopy := append([]int64{}, values...)
		timestampsCopy, valuesCopy = deduplicateSamplesDuringMerge(timestampsCopy, valuesCopy, dedupInterval)
		if !reflect.DeepEqual(timestampsCopy, timestampsExpected) {
			t.Fatalf("invalid deduplicateSamplesDuringMerge(%v) timestamps for the second call;\ngot\n%v\nwant\n%v", timestamps, timestampsCopy, timestampsExpected)
		}
		if !reflect.DeepEqual(valuesCopy, values) {
			t.Fatalf("invalid deduplicateSamplesDuringMerge(%v) values for the second call;\ngot\n%v\nwant\n%v", timestamps, values, valuesCopy)
		}
	}
	f(time.Second, []int64{1000, 1000}, []int64{2, 1}, []int64{1000}, []int64{2})
	f(time.Second, []int64{1001, 1001}, []int64{2, 1}, []int64{1001}, []int64{2})
	f(time.Second, []int64{1000, 1001, 1001, 1001, 2001}, []int64{1, 2, 5, 3, 0}, []int64{1000, 1001, 2001}, []int64{1, 5, 0})

	// verify decimal.StaleNaN is NOT preferred on timestamp conflicts
	// see https://github.com/VictoriaMetrics/VictoriaMetrics/issues/10196
	staleNaN, _ := decimal.FromFloat(decimal.StaleNaN)
	f(time.Second, []int64{1000, 1000}, []int64{2, staleNaN}, []int64{1000}, []int64{2})
	f(time.Second, []int64{1000, 1000}, []int64{staleNaN, 2}, []int64{1000}, []int64{2})
	f(time.Second, []int64{1000, 1000, 1000}, []int64{1, staleNaN, 2}, []int64{1000}, []int64{2})
	// compare with max values
	f(time.Second, []int64{1000, 1000}, []int64{math.MaxInt64, staleNaN}, []int64{1000}, []int64{math.MaxInt64})
	f(time.Second, []int64{1000, 1000, 1000}, []int64{math.MaxInt64, staleNaN, math.MaxInt64}, []int64{1000}, []int64{math.MaxInt64})
	f(time.Second, []int64{1000, 1000, 2000}, []int64{1, staleNaN, 2}, []int64{1000, 2000}, []int64{1, 2})
	f(time.Second, []int64{1000, 1000, 2000, 2000}, []int64{1, staleNaN, 2, 3}, []int64{1000, 2000}, []int64{1, 3})
	f(time.Second, []int64{1000, 1000, 1000, 2000, 2000}, []int64{1, staleNaN, math.MaxInt64, 2, 3}, []int64{1000, 2000}, []int64{math.MaxInt64, 3})
}

func TestDeduplicateSamples(t *testing.T) {
	// Disable deduplication before exit, since the rest of tests expect disabled dedup.

	f := func(scrapeInterval time.Duration, timestamps, timestampsExpected []int64, valuesExpected []float64) {
		t.Helper()
		timestampsCopy := make([]int64, len(timestamps))
		values := make([]float64, len(timestamps))
		for i, ts := range timestamps {
			timestampsCopy[i] = ts
			values[i] = float64(i)
		}
		dedupInterval := scrapeInterval.Milliseconds()
		timestampsCopy, values = DeduplicateSamples(timestampsCopy, values, dedupInterval)
		if !reflect.DeepEqual(timestampsCopy, timestampsExpected) {
			t.Fatalf("invalid DeduplicateSamples(%v) timestamps;\ngot\n%v\nwant\n%v", timestamps, timestampsCopy, timestampsExpected)
		}
		if !reflect.DeepEqual(values, valuesExpected) {
			t.Fatalf("invalid DeduplicateSamples(%v) values;\ngot\n%v\nwant\n%v", timestamps, values, valuesExpected)
		}

		// Verify that the second call to DeduplicateSamples doesn't modify samples.
		valuesCopy := append([]float64{}, values...)
		timestampsCopy, valuesCopy = DeduplicateSamples(timestampsCopy, valuesCopy, dedupInterval)
		if !reflect.DeepEqual(timestampsCopy, timestampsExpected) {
			t.Fatalf("invalid DeduplicateSamples(%v) timestamps for the second call;\ngot\n%v\nwant\n%v", timestamps, timestampsCopy, timestampsExpected)
		}
		if !reflect.DeepEqual(valuesCopy, values) {
			t.Fatalf("invalid DeduplicateSamples(%v) values for the second call;\ngot\n%v\nwant\n%v", timestamps, values, valuesCopy)
		}
	}
	f(time.Millisecond, nil, []int64{}, []float64{})
	f(time.Millisecond, []int64{123}, []int64{123}, []float64{0})
	f(time.Millisecond, []int64{123, 456}, []int64{123, 456}, []float64{0, 1})
	f(time.Millisecond, []int64{0, 0, 0, 1, 1, 2, 3, 3, 3, 4}, []int64{0, 1, 2, 3, 4}, []float64{2, 4, 5, 8, 9})
	f(0, []int64{0, 0, 0, 1, 1, 2, 3, 3, 3, 4}, []int64{0, 0, 0, 1, 1, 2, 3, 3, 3, 4}, []float64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9})
	f(100*time.Millisecond, []int64{0, 100, 100, 101, 150, 180, 205, 300, 1000}, []int64{0, 100, 180, 300, 1000}, []float64{0, 2, 5, 7, 8})
	f(10*time.Second, []int64{10e3, 13e3, 21e3, 22e3, 30e3, 33e3, 39e3, 45e3}, []int64{10e3, 13e3, 30e3, 39e3, 45e3}, []float64{0, 1, 4, 6, 7})
}

func TestDeduplicateSamplesDuringMerge(t *testing.T) {
	// Disable deduplication before exit, since the rest of tests expect disabled dedup.

	f := func(scrapeInterval time.Duration, timestamps, timestampsExpected, valuesExpected []int64) {
		t.Helper()
		timestampsCopy := make([]int64, len(timestamps))
		values := make([]int64, len(timestamps))
		for i, ts := range timestamps {
			timestampsCopy[i] = ts
			values[i] = int64(i)
		}
		dedupInterval := scrapeInterval.Milliseconds()
		timestampsCopy, values = deduplicateSamplesDuringMerge(timestampsCopy, values, dedupInterval)
		if !reflect.DeepEqual(timestampsCopy, timestampsExpected) {
			t.Fatalf("invalid deduplicateSamplesDuringMerge(%v) timestamps;\ngot\n%v\nwant\n%v", timestamps, timestampsCopy, timestampsExpected)
		}
		if !reflect.DeepEqual(values, valuesExpected) {
			t.Fatalf("invalid DeduplicateSamples(%v) values;\ngot\n%v\nwant\n%v", timestamps, values, valuesExpected)
		}

		// Verify that the second call to DeduplicateSamples doesn't modify samples.
		valuesCopy := append([]int64{}, values...)
		timestampsCopy, valuesCopy = deduplicateSamplesDuringMerge(timestampsCopy, valuesCopy, dedupInterval)
		if !reflect.DeepEqual(timestampsCopy, timestampsExpected) {
			t.Fatalf("invalid deduplicateSamplesDuringMerge(%v) timestamps for the second call;\ngot\n%v\nwant\n%v", timestamps, timestampsCopy, timestampsExpected)
		}
		if !reflect.DeepEqual(valuesCopy, values) {
			t.Fatalf("invalid deduplicateSamplesDuringMerge(%v) values for the second call;\ngot\n%v\nwant\n%v", timestamps, values, valuesCopy)
		}
	}
	f(time.Millisecond, nil, []int64{}, []int64{})
	f(time.Millisecond, []int64{123}, []int64{123}, []int64{0})
	f(time.Millisecond, []int64{123, 456}, []int64{123, 456}, []int64{0, 1})
	f(time.Millisecond, []int64{0, 0, 0, 1, 1, 2, 3, 3, 3, 4}, []int64{0, 1, 2, 3, 4}, []int64{2, 4, 5, 8, 9})
	f(100*time.Millisecond, []int64{0, 100, 100, 101, 150, 180, 200, 300, 1000}, []int64{0, 100, 200, 300, 1000}, []int64{0, 2, 6, 7, 8})
	f(10*time.Second, []int64{10e3, 13e3, 21e3, 22e3, 30e3, 33e3, 39e3, 45e3}, []int64{10e3, 13e3, 30e3, 39e3, 45e3}, []int64{0, 1, 4, 6, 7})
}

func TestDeduplicateSamples_KeepsFirstAndLast(t *testing.T) {
	f := func(dedupInterval time.Duration, timestamps []int64, values []float64, timestampsExpected []int64, valuesExpected []float64) {
		t.Helper()
		tsCopy := append([]int64{}, timestamps...)
		vCopy := append([]float64{}, values...)

		tsCopy, vCopy = DeduplicateSamples(tsCopy, vCopy, dedupInterval.Milliseconds())

		// Original boundary checks for clarity and safety
		if len(tsCopy) == 0 {
			t.Fatalf("deduplication removed all samples for timestamps %v", timestamps)
		}
		if tsCopy[0] != timestamps[0] || !equalWithNans([]float64{vCopy[0]}, []float64{values[0]}) {
			t.Fatalf("first sample lost; got (%d,%v) want (%d,%v)", tsCopy[0], vCopy[0], timestamps[0], values[0])
		}
		if tsCopy[len(tsCopy)-1] != timestamps[len(timestamps)-1] || !equalWithNans([]float64{vCopy[len(vCopy)-1]}, []float64{values[len(values)-1]}) {
			t.Fatalf("last sample lost; got (%d,%v) want (%d,%v)", tsCopy[len(tsCopy)-1], vCopy[len(vCopy)-1], timestamps[len(timestamps)-1], values[len(values)-1])
		}

		// Exact-set comparisons suggested for clarity
		if !reflect.DeepEqual(tsCopy, timestampsExpected) {
			t.Fatalf("unexpected timestamps after dedup;\ngot  %v\nwant %v", tsCopy, timestampsExpected)
		}
		if !equalWithNans(vCopy, valuesExpected) {
			t.Fatalf("unexpected values after dedup;\ngot  %v\nwant %v", vCopy, valuesExpected)
		}
	}

	// duplicates around edges
	f(time.Second,
		[]int64{0, 200, 400, 800, 1000, 1200, 1500, 2100, 2300, 2500, 2500},
		[]float64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
		[]int64{0, 1000, 1500, 2500},
		[]float64{0, 4, 6, 10},
	)

	// heavy duplication in first and last intervals
	f(time.Second,
		[]int64{0, 100, 200, 300, 700, 1000, 1600, 1700, 1800, 2300, 2400, 2500},
		[]float64{10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21},
		[]int64{0, 1000, 1800, 2500},
		[]float64{10, 15, 18, 21},
	)

	// single sample case
	f(time.Second,
		[]int64{1000},
		[]float64{42},
		[]int64{1000},
		[]float64{42},
	)

	// two samples case (different intervals, nothing to drop)
	f(time.Second,
		[]int64{0, 2000},
		[]float64{1, 2},
		[]int64{0, 2000},
		[]float64{1, 2},
	)

	// many duplicates at start
	f(time.Second,
		[]int64{0, 100, 200, 300, 400, 500, 1500, 2000},
		[]float64{1, 2, 3, 4, 5, 6, 7, 8},
		[]int64{0, 500, 2000},
		[]float64{1, 6, 8},
	)

	// many duplicates at end
	f(time.Second,
		[]int64{0, 1000, 2000, 2100, 2200, 2300, 2400, 2500},
		[]float64{1, 2, 3, 4, 5, 6, 7, 8},
		[]int64{0, 1000, 2000, 2500},
		[]float64{1, 2, 3, 8},
	)
}

func TestDeduplicateSamplesDuringMerge_KeepsFirstAndLast(t *testing.T) {
	f := func(dedupInterval time.Duration, timestamps []int64, values []int64, timestampsExpected []int64, valuesExpected []int64) {
		t.Helper()
		tsCopy := append([]int64{}, timestamps...)
		vCopy := append([]int64{}, values...)

		tsCopy, vCopy = deduplicateSamplesDuringMerge(tsCopy, vCopy, dedupInterval.Milliseconds())

		// Original boundary checks
		if len(tsCopy) == 0 {
			t.Fatalf("deduplication removed all samples for timestamps %v", timestamps)
		}
		if tsCopy[0] != timestamps[0] || vCopy[0] != values[0] {
			t.Fatalf("first sample lost; got (%d,%d) want (%d,%d)", tsCopy[0], vCopy[0], timestamps[0], values[0])
		}
		if tsCopy[len(tsCopy)-1] != timestamps[len(timestamps)-1] || vCopy[len(vCopy)-1] != values[len(values)-1] {
			t.Fatalf("last sample lost; got (%d,%d) want (%d,%d)", tsCopy[len(tsCopy)-1], vCopy[len(vCopy)-1], timestamps[len(timestamps)-1], values[len(values)-1])
		}

		// Exact-set comparisons
		if !reflect.DeepEqual(tsCopy, timestampsExpected) {
			t.Fatalf("unexpected timestamps after dedup;\ngot  %v\nwant %v", tsCopy, timestampsExpected)
		}
		if !reflect.DeepEqual(vCopy, valuesExpected) {
			t.Fatalf("unexpected values after dedup;\ngot  %v\nwant %v", vCopy, valuesExpected)
		}
	}

	// duplicates around edges
	f(time.Second,
		[]int64{0, 200, 400, 800, 1000, 1300, 1500, 2100, 2400, 2500, 2500},
		[]int64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
		[]int64{0, 1000, 1500, 2500},
		[]int64{0, 4, 6, 10},
	)

	// heavy duplication in first and last intervals
	f(time.Second,
		[]int64{0, 100, 200, 300, 700, 1000, 1600, 1700, 1800, 2300, 2400, 2500},
		[]int64{10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21},
		[]int64{0, 1000, 1800, 2500},
		[]int64{10, 15, 18, 21},
	)

	// single sample case
	f(time.Second,
		[]int64{1000},
		[]int64{42},
		[]int64{1000},
		[]int64{42},
	)

	// two samples case
	f(time.Second,
		[]int64{0, 2000},
		[]int64{1, 2},
		[]int64{0, 2000},
		[]int64{1, 2},
	)

	// many duplicates at start
	f(time.Second,
		[]int64{0, 100, 200, 300, 400, 500, 1500, 2000},
		[]int64{1, 2, 3, 4, 5, 6, 7, 8},
		[]int64{0, 500, 2000},
		[]int64{1, 6, 8},
	)

	// many duplicates at end
	f(time.Second,
		[]int64{0, 1000, 2000, 2100, 2200, 2300, 2400, 2500},
		[]int64{1, 2, 3, 4, 5, 6, 7, 8},
		[]int64{0, 1000, 2000, 2500},
		[]int64{1, 2, 3, 8},
	)
}
