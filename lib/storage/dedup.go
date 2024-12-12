package storage

import (
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/decimal"
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

// DeduplicateSamples removes samples from src* if they are closer to each other than dedupInterval in milliseconds.
// DeduplicateSamples treats StaleNaN (Prometheus stale markers) as values and doesn't skip them on purpose - see
// https://github.com/VictoriaMetrics/VictoriaMetrics/issues/5587
func DeduplicateSamples(srcTimestamps []int64, srcValues []float64, dedupInterval int64) ([]int64, []float64) {
	if !needsDedup(srcTimestamps, dedupInterval) {
		// Fast path - nothing to deduplicate
		return srcTimestamps, srcValues
	}
	tsNext := srcTimestamps[0] + dedupInterval - 1
	tsNext -= tsNext % dedupInterval
	dstTimestamps := srcTimestamps[:0]
	dstValues := srcValues[:0]
	for i, ts := range srcTimestamps[1:] {
		if ts <= tsNext {
			continue
		}
		// Choose the maximum value with the timestamp equal to tsPrev.
		// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3333
		j := i
		tsPrev := srcTimestamps[j]
		vPrev := srcValues[j]
		for j > 0 && srcTimestamps[j-1] == tsPrev {
			j--
			if decimal.IsStaleNaN(srcValues[j]) {
				// always prefer decimal.IsStaleNaN to avoid inconsistency when comparing values
				// see https://github.com/VictoriaMetrics/VictoriaMetrics/issues/7674
				vPrev = srcValues[j]
				break
			}
			if srcValues[j] > vPrev {
				vPrev = srcValues[j]
			}
		}
		dstTimestamps = append(dstTimestamps, tsPrev)
		dstValues = append(dstValues, vPrev)
		tsNext += dedupInterval
		if tsNext < ts {
			tsNext = ts + dedupInterval - 1
			tsNext -= tsNext % dedupInterval
		}
	}
	j := len(srcTimestamps) - 1
	tsPrev := srcTimestamps[j]
	vPrev := srcValues[j]
	// Invariant: vPrev > srcValues[j]
	for j > 0 && srcTimestamps[j-1] == tsPrev {
		j--
		if decimal.IsStaleNaN(srcValues[j]) {
			// always prefer decimal.IsStaleNaN to avoid inconsistency when comparing values
			// see https://github.com/VictoriaMetrics/VictoriaMetrics/issues/7674
			vPrev = srcValues[j]
			break
		}
		if srcValues[j] > vPrev {
			vPrev = srcValues[j]
		}
	}
	dstTimestamps = append(dstTimestamps, tsPrev)
	dstValues = append(dstValues, vPrev)
	return dstTimestamps, dstValues
}

func deduplicateSamplesDuringMerge(srcTimestamps, srcValues []int64, dedupInterval int64) ([]int64, []int64) {
	if !needsDedup(srcTimestamps, dedupInterval) {
		// Fast path - nothing to deduplicate
		return srcTimestamps, srcValues
	}
	tsNext := srcTimestamps[0] + dedupInterval - 1
	tsNext -= tsNext % dedupInterval
	dstTimestamps := srcTimestamps[:0]
	dstValues := srcValues[:0]
	for i, ts := range srcTimestamps[1:] {
		if ts <= tsNext {
			continue
		}
		// Choose the maximum value with the timestamp equal to tsPrev.
		// See https://github.com/VictoriaMetrics/VictoriaMetrics/issues/3333
		j := i
		tsPrev := srcTimestamps[j]
		vPrev := srcValues[j]
		for j > 0 && srcTimestamps[j-1] == tsPrev {
			j--
			if decimal.IsStaleNaNInt64(srcValues[j]) {
				// always prefer decimal.IsStaleNaN to avoid inconsistency when comparing values
				// see https://github.com/VictoriaMetrics/VictoriaMetrics/issues/7674
				vPrev = srcValues[j]
				break
			}
			if srcValues[j] > vPrev {
				vPrev = srcValues[j]
			}
		}
		dstTimestamps = append(dstTimestamps, tsPrev)
		dstValues = append(dstValues, vPrev)
		tsNext += dedupInterval
		if tsNext < ts {
			tsNext = ts + dedupInterval - 1
			tsNext -= tsNext % dedupInterval
		}
	}
	j := len(srcTimestamps) - 1
	tsPrev := srcTimestamps[j]
	vPrev := srcValues[j]
	if decimal.IsStaleNaNInt64(vPrev) {
		// fast path - decimal.StaleNaN is always preferred to other values on interval
		dstTimestamps = append(dstTimestamps, tsPrev)
		dstValues = append(dstValues, vPrev)
		return dstTimestamps, dstValues
	}
	for j > 0 && srcTimestamps[j-1] == tsPrev {
		j--
		if decimal.IsStaleNaNInt64(srcValues[j]) {
			// always prefer decimal.IsStaleNaN to avoid inconsistency when comparing values
			// see https://github.com/VictoriaMetrics/VictoriaMetrics/issues/7674
			vPrev = srcValues[j]
			break
		}
		if srcValues[j] > vPrev {
			vPrev = srcValues[j]
		}
	}
	dstTimestamps = append(dstTimestamps, tsPrev)
	dstValues = append(dstValues, vPrev)
	return dstTimestamps, dstValues
}

func needsDedup(timestamps []int64, dedupInterval int64) bool {
	if len(timestamps) < 2 || dedupInterval <= 0 {
		return false
	}
	tsNext := timestamps[0] + dedupInterval - 1
	tsNext -= tsNext % dedupInterval
	for _, ts := range timestamps[1:] {
		if ts <= tsNext {
			return true
		}
		tsNext += dedupInterval
		if tsNext < ts {
			tsNext = ts + dedupInterval - 1
			tsNext -= tsNext % dedupInterval
		}
	}
	return false
}
