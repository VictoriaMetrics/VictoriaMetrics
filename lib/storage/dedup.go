package storage

import (
	"time"
)

// SetDedupInterval sets the deduplication interval, which is applied to raw samples during data ingestion and querying.
//
// De-duplication is disabled if dedupInterval is 0.
//
// This function must be called before initializing the storage.
func SetDedupInterval(dedupInterval time.Duration) {
	globalDedupInterval = dedupInterval.Milliseconds()
}

// GetDedupInterval returns the dedup interval in milliseconds, which has been set via SetDedupInterval.
func GetDedupInterval() int64 {
	return globalDedupInterval
}

var globalDedupInterval int64

func isDedupEnabled() bool {
	return globalDedupInterval > 0
}

// DeduplicateSamples removes samples from src* if they are closer to each other than dedupInterval in millseconds.
func DeduplicateSamples(srcTimestamps []int64, srcValues []float64, dedupInterval int64) ([]int64, []float64) {
	if !needsDedup(srcTimestamps, dedupInterval) {
		// Fast path - nothing to deduplicate
		return srcTimestamps, srcValues
	}
	return deduplicateInternal(srcTimestamps, srcValues, dedupInterval)
}

func deduplicateInternal(srcTimestamps []int64, srcValues []float64, dedupInterval int64) ([]int64, []float64) {
	tsNext := (srcTimestamps[0] - srcTimestamps[0]%dedupInterval) + dedupInterval
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
		tsNext += dedupInterval
		if ts >= tsNext {
			// Slow path for updating ts.
			tsNext = (ts - ts%dedupInterval) + dedupInterval
		}
	}
	return dstTimestamps, dstValues
}

func deduplicateSamplesDuringMerge(srcTimestamps, srcValues []int64, dedupInterval int64) ([]int64, []int64) {
	if !needsDedup(srcTimestamps, dedupInterval) {
		// Fast path - nothing to deduplicate
		return srcTimestamps, srcValues
	}
	return deduplicateDuringMergeInternal(srcTimestamps, srcValues, dedupInterval)
}

func deduplicateDuringMergeInternal(srcTimestamps, srcValues []int64, dedupInterval int64) ([]int64, []int64) {
	tsNext := (srcTimestamps[0] - srcTimestamps[0]%dedupInterval) + dedupInterval
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
		tsNext += dedupInterval
		if ts >= tsNext {
			// Slow path for updating ts.
			tsNext = (ts - ts%dedupInterval) + dedupInterval
		}
	}
	return dstTimestamps, dstValues
}

func needsDedup(timestamps []int64, dedupInterval int64) bool {
	if len(timestamps) == 0 || dedupInterval <= 0 {
		return false
	}
	tsNext := (timestamps[0] - timestamps[0]%dedupInterval) + dedupInterval
	for _, ts := range timestamps[1:] {
		if ts < tsNext {
			return true
		}
		tsNext += dedupInterval
		if ts >= tsNext {
			tsNext = (ts - ts%dedupInterval) + dedupInterval
		}
	}
	return false
}
