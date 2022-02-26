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

// SetMetricPointInterval sets the metrics points interval, which is applied to raw samples during data ingestion and querying.
//
// De-duplication is disabled if dedupMetricPointInterval is 0.
//
// This function must be called before initializing the storage.
func SetMetricPointInterval(dedupMetricPointInterval time.Duration) {
	globalDedupMetricPointInterval = dedupMetricPointInterval.Milliseconds()
}

// GetDedupMetricPointInterval returns the dedup interval in milliseconds, which has been set via SetMetricPointInterval.
func GetDedupMetricPointInterval() int64 {
	return globalDedupMetricPointInterval
}

var globalDedupInterval int64

var globalDedupMetricPointInterval int64

func isDedupEnabled() bool {
	return globalDedupInterval > 0
}

func isDedupMetricPointEnabled() bool {
	return globalDedupMetricPointInterval > 0
}

// DeduplicateSamples removes samples from src* if they are closer to each other than dedupInterval in millseconds.
func DeduplicateSamples(srcTimestamps []int64, srcValues []float64, dedupInterval int64, dedupMetricPointInterval int64) ([]int64, []float64) {
	if needsMetricPointDedup(srcTimestamps, dedupMetricPointInterval) {
		// only do deduplicate but not return because maybe conf args :-dedup.minScrapeInterval so shoud do other logic
		srcTimestamps, srcValues = deduplicateMetricPointInternal(srcTimestamps, srcValues, dedupMetricPointInterval)
	}
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

func deduplicateMetricPointInternal(srcTimestamps []int64, srcValues []float64, dedupMetricPointInterval int64) ([]int64, []float64) {
	tsPre := srcTimestamps[0]
	dstTimestamps := srcTimestamps[:1]
	dstValues := srcValues[:1]
	for i := 1; i < len(srcTimestamps); i++ {
		ts := srcTimestamps[i]
		if ts-tsPre < dedupMetricPointInterval {
			continue
		}
		dstTimestamps = append(dstTimestamps, ts)
		dstValues = append(dstValues, srcValues[i])
		tsPre = ts
	}
	return dstTimestamps, dstValues
}

func deduplicateSamplesDuringMerge(srcTimestamps, srcValues []int64, dedupInterval int64, dedupMetricPointInterval int64) ([]int64, []int64) {
	if needsMetricPointDedup(srcTimestamps, dedupMetricPointInterval) {
		// only do deduplicate but not return because maybe conf args :-dedup.minScrapeInterval so shoud do other logic
		srcTimestamps, srcValues = deduplicateMetricPointDuringMergeInternal(srcTimestamps, srcValues, dedupMetricPointInterval)
	}
	if !needsDedup(srcTimestamps, dedupInterval) {
		// Fast path - nothing to deduplicate
		return srcTimestamps, srcValues
	}
	return deduplicateDuringMergeInternal(srcTimestamps, srcValues, dedupInterval)
}

func deduplicateMetricPointDuringMergeInternal(srcTimestamps, srcValues []int64, dedupMetricPointInterval int64) ([]int64, []int64) {
	tsPre := srcTimestamps[0]
	dstTimestamps := srcTimestamps[:1]
	dstValues := srcValues[:1]
	for i := 1; i < len(srcTimestamps); i++ {
		ts := srcTimestamps[i]
		if ts-tsPre < dedupMetricPointInterval {
			continue
		}
		dstTimestamps = append(dstTimestamps, ts)
		dstValues = append(dstValues, srcValues[i])

		tsPre = ts
	}
	return dstTimestamps, dstValues
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

func needsMetricPointDedup(timestamps []int64, dedupMetricPointInterval int64) bool {
	if len(timestamps) == 0 || dedupMetricPointInterval <= 0 {
		return false
	}
	tsPre := timestamps[0]
	for _, ts := range timestamps[1:] {
		if ts-tsPre < dedupMetricPointInterval {
			return true
		}
		tsPre = ts
	}
	return false
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
