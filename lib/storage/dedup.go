package storage

import (
	"time"
)

// SetMinScrapeIntervalForDeduplication sets the minimum interval for data points during de-duplication.
//
// De-duplication is disabled if interval is 0.
//
// This function must be called before initializing the storage.
func SetMinScrapeIntervalForDeduplication(interval time.Duration) {
	minScrapeInterval = interval
}

var minScrapeInterval = time.Duration(0)

func getMinDelta() int64 {
	// Use 7/8 of minScrapeInterval in order to preserve proper data points.
	// For instance, if minScrapeInterval=10, the following time series:
	//    10 15 19 25 30 34 41
	// Would be unexpectedly converted to if using 100% of minScrapeInterval:
	//    10 25 41
	// When using 7/8 of minScrapeInterval, it will be converted to the expected:
	//    10 19 30 41
	return (minScrapeInterval.Milliseconds() / 8) * 7
}

// DeduplicateSamples removes samples from src* if they are closer to each other than minScrapeInterval.
func DeduplicateSamples(srcTimestamps []int64, srcValues []float64) ([]int64, []float64) {
	if minScrapeInterval <= 0 {
		return srcTimestamps, srcValues
	}
	minDelta := getMinDelta()
	if !needsDedup(srcTimestamps, minDelta) {
		// Fast path - nothing to deduplicate
		return srcTimestamps, srcValues
	}

	// Slow path - dedup data points.
	prevTimestamp := srcTimestamps[0]
	dstTimestamps := srcTimestamps[:1]
	dstValues := srcValues[:1]
	for i := 1; i < len(srcTimestamps); i++ {
		ts := srcTimestamps[i]
		if ts-prevTimestamp < minDelta {
			continue
		}
		dstTimestamps = append(dstTimestamps, ts)
		dstValues = append(dstValues, srcValues[i])
		prevTimestamp = ts
	}
	return dstTimestamps, dstValues
}

func deduplicateSamplesDuringMerge(srcTimestamps []int64, srcValues []int64) ([]int64, []int64) {
	if minScrapeInterval <= 0 {
		return srcTimestamps, srcValues
	}
	if len(srcTimestamps) < 32 {
		// Do not de-duplicate small number of samples during merge
		// in order to improve deduplication accuracy on later stages.
		return srcTimestamps, srcValues
	}
	minDelta := getMinDelta()
	if !needsDedup(srcTimestamps, minDelta) {
		// Fast path - nothing to deduplicate
		return srcTimestamps, srcValues
	}

	// Slow path - dedup data points.
	prevTimestamp := srcTimestamps[0]
	dstTimestamps := srcTimestamps[:1]
	dstValues := srcValues[:1]
	for i := 1; i < len(srcTimestamps); i++ {
		ts := srcTimestamps[i]
		if ts-prevTimestamp < minDelta {
			continue
		}
		dstTimestamps = append(dstTimestamps, ts)
		dstValues = append(dstValues, srcValues[i])
		prevTimestamp = ts
	}
	return dstTimestamps, dstValues
}

func needsDedup(timestamps []int64, minDelta int64) bool {
	if len(timestamps) == 0 {
		return false
	}
	prevTimestamp := timestamps[0]
	for _, ts := range timestamps[1:] {
		if ts-prevTimestamp < minDelta {
			return true
		}
		prevTimestamp = ts
	}
	return false
}
