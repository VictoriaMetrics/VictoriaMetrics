package storage

import (
	"reflect"
	"testing"
	"time"
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

func TestDeduplicateSamplesWithIdenticalTimestamps(t *testing.T) {
	f := func(scrapeInterval time.Duration, timestamps []int64, values []float64, timestampsExpected []int64, valuesExpected []float64) {
		t.Helper()
		timestampsCopy := append([]int64{}, timestamps...)

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
	f(time.Second, []int64{1000, 1000}, []float64{2, 1}, []int64{1000}, []float64{2})
	f(time.Second, []int64{1001, 1001}, []float64{2, 1}, []int64{1001}, []float64{2})
	f(time.Second, []int64{1000, 1001, 1001, 1001, 2001}, []float64{1, 2, 5, 3, 0}, []int64{1000, 1001, 2001}, []float64{1, 5, 0})
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
