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
	minScrapeInterval = interval.Milliseconds()
}

var minScrapeInterval = int64(0)

// DeduplicateSamples removes samples from src* if they are closer to each other than minScrapeInterval.
func DeduplicateSamples(srcTimestamps []int64, srcValues []float64) ([]int64, []float64) {
	if minScrapeInterval <= 0 {
		return srcTimestamps, srcValues
	}
	if !needsDedup(srcTimestamps, minScrapeInterval) {
		// Fast path - nothing to deduplicate
		return srcTimestamps, srcValues
	}
	return deduplicateInternal(minScrapeInterval, srcTimestamps, srcValues)
}

func deduplicateInternal(interval int64, srcTimestamps []int64, srcValues []float64) ([]int64, []float64) {
	tsNext := (srcTimestamps[0] - srcTimestamps[0]%interval) + interval
	dstTimestamps := srcTimestamps[:1]
	dstValues := srcValues[:1]
	for i := 1; i < len(srcTimestamps); i++ {
		ts := srcTimestamps[i]
		if ts < tsNext {
			continue
		}
		dstTimestamps = append(dstTimestamps, ts)
		dstValues = append(dstValues, srcValues[i])

		// Update tsNext
		tsNext += interval
		if ts >= tsNext {
			// Slow path for updating ts.
			tsNext = (ts - ts%interval) + interval
		}
	}
	return dstTimestamps, dstValues
}

func deduplicateSamplesDuringMerge(srcTimestamps, srcValues []int64) ([]int64, []int64) {
	if minScrapeInterval <= 0 {
		return srcTimestamps, srcValues
	}
	if !needsDedup(srcTimestamps, minScrapeInterval) {
		// Fast path - nothing to deduplicate
		return srcTimestamps, srcValues
	}
	return deduplicateDuringMergeInternal(minScrapeInterval, srcTimestamps, srcValues)
}

func deduplicateDuringMergeInternal(interval int64, srcTimestamps, srcValues []int64) ([]int64, []int64) {
	tsNext := (srcTimestamps[0] - srcTimestamps[0]%interval) + interval
	dstTimestamps := srcTimestamps[:1]
	dstValues := srcValues[:1]
	for i := 1; i < len(srcTimestamps); i++ {
		ts := srcTimestamps[i]
		if ts < tsNext {
			continue
		}
		dstTimestamps = append(dstTimestamps, ts)
		dstValues = append(dstValues, srcValues[i])

		// Update tsNext
		tsNext += interval
		if ts >= tsNext {
			// Slow path for updating ts.
			tsNext = (ts - ts%interval) + interval
		}
	}
	return dstTimestamps, dstValues
}

func needsDedup(timestamps []int64, interval int64) bool {
	if len(timestamps) == 0 || interval <= 0 {
		return false
	}
	tsNext := (timestamps[0] - timestamps[0]%interval) + interval
	for _, ts := range timestamps[1:] {
		if ts < tsNext {
			return true
		}
		tsNext += interval
		if ts >= tsNext {
			tsNext = (ts - ts%interval) + interval
		}
	}
	return false
}
