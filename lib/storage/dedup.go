package storage

import (
	"flag"

	"github.com/VictoriaMetrics/metrics"
)

var minScrapeInterval = flag.Duration("dedup.minScrapeInterval", 0, "Remove superflouos samples from time series if they are located closer to each other than this duration. "+
	"This may be useful for reducing overhead when multiple identically configured Prometheus instances write data to the same VictoriaMetrics. "+
	"Deduplication is disabled if the -dedup.minScrapeInterval is 0")

func getMinDelta() int64 {
	// Divide minScrapeInterval by 2 in order to preserve proper data points.
	// For instance, if minScrapeInterval=10, the following time series:
	//    10 15 19 25 30 34 41
	// Would be unexpectedly converted to:
	//    10 25 41
	// When dividing minScrapeInterval by 2, it will be converted to the expected:
	//    10 19 30 41
	return minScrapeInterval.Milliseconds() / 2
}

// DeduplicateSamples removes samples from src* if they are closer to each other than minScrapeInterval.
func DeduplicateSamples(srcTimestamps []int64, srcValues []float64) ([]int64, []float64) {
	if *minScrapeInterval <= 0 {
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
	dedups := 0
	for i := 1; i < len(srcTimestamps); i++ {
		ts := srcTimestamps[i]
		if ts-prevTimestamp < minDelta {
			dedups++
			continue
		}
		dstTimestamps = append(dstTimestamps, ts)
		dstValues = append(dstValues, srcValues[i])
		prevTimestamp = ts
	}
	dedupsDuringSelect.Add(dedups)
	return dstTimestamps, dstValues
}

var dedupsDuringSelect = metrics.NewCounter(`deduplicated_samples_total{type="select"}`)

func deduplicateSamplesDuringMerge(srcTimestamps []int64, srcValues []int64) ([]int64, []int64) {
	if *minScrapeInterval <= 0 {
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
	dedups := 0
	for i := 1; i < len(srcTimestamps); i++ {
		ts := srcTimestamps[i]
		if ts-prevTimestamp < minDelta {
			dedups++
			continue
		}
		dstTimestamps = append(dstTimestamps, ts)
		dstValues = append(dstValues, srcValues[i])
		prevTimestamp = ts
	}
	dedupsDuringMerge.Add(dedups)
	return dstTimestamps, dstValues
}

var dedupsDuringMerge = metrics.NewCounter(`deduplicated_samples_total{type="merge"}`)

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
