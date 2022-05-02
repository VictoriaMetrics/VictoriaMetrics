package storage

func isDedupEnabled() bool {
	return len(downsamplingPeriods) > 0
}

// DeduplicateSamples removes samples from src* if they are closer to each other than dedupInterval in millseconds.
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
		dstTimestamps = append(dstTimestamps, srcTimestamps[i])
		dstValues = append(dstValues, srcValues[i])
		tsNext += dedupInterval
		if tsNext < ts {
			tsNext = ts + dedupInterval - 1
			tsNext -= tsNext % dedupInterval
		}
	}
	dstTimestamps = append(dstTimestamps, srcTimestamps[len(srcTimestamps)-1])
	dstValues = append(dstValues, srcValues[len(srcValues)-1])
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
		dstTimestamps = append(dstTimestamps, srcTimestamps[i])
		dstValues = append(dstValues, srcValues[i])
		tsNext += dedupInterval
		if tsNext < ts {
			tsNext = ts + dedupInterval - 1
			tsNext -= tsNext % dedupInterval
		}
	}
	dstTimestamps = append(dstTimestamps, srcTimestamps[len(srcTimestamps)-1])
	dstValues = append(dstValues, srcValues[len(srcValues)-1])
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
