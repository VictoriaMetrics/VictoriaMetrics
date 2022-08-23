package storage

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
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

var meta []DownSamplingMeta

func GetDownSamplingMeta() []DownSamplingMeta {
	return meta
}

func SetDownsampling(downsampling string) {
	if downsampling != "" {
		//downsampling like 30d:5m,180d:30m
		aggregates := strings.Split(downsampling, ",")
		if len(aggregates) > 0 {
			for _, v := range aggregates {
				chunks := strings.Split(v, ":")
				if len(chunks) > 0 {
					duration, e1 := convertDuration(chunks[0])
					downsamplingInterval, e2 := convertDuration(chunks[1])
					if e1 == nil && e2 == nil {
						if downsamplingInterval > 1 {
							dm := DownSamplingMeta{Duration: duration,
								DownsamplingInterval: downsamplingInterval}
							meta = append(meta, dm)
						}
					}
				}
			}
		}
		if len(meta) > 1 {
			sort.Slice(meta, func(i, j int) bool {
				return meta[i].Duration > meta[j].Duration
			})
		}
	}
}

func convertDuration(duration string) (time.Duration, error) {
	/*
		Golang's time library doesn't support many different
		string formats (year, month, week, day) because they
		aren't consistent ranges. But Java's library _does_.
		Consequently, we'll need to handle all the custom
		time ranges, and, to make the internal API call consistent,
		we'll need to allow for durations that Go supports, too.
		The nice thing is all the "broken" time ranges are > 1 hour,
		so we can just make assumptions to convert them to a range in hours.
		They aren't *good* assumptions, but they're reasonable
		for this function.
	*/
	var actualDuration time.Duration
	var err error
	var timeValue int
	if strings.HasSuffix(duration, "y") {
		timeValue, err = strconv.Atoi(strings.Trim(duration, "y"))
		if err != nil {
			return 0, fmt.Errorf("invalid time range: %q", duration)
		}
		timeValue = timeValue * 365 * 24
		actualDuration, err = time.ParseDuration(fmt.Sprintf("%vh", timeValue))
		if err != nil {
			return 0, fmt.Errorf("invalid time range: %q", duration)
		}
	} else if strings.HasSuffix(duration, "w") {
		timeValue, err = strconv.Atoi(strings.Trim(duration, "w"))
		if err != nil {
			return 0, fmt.Errorf("invalid time range: %q", duration)
		}
		timeValue = timeValue * 7 * 24
		actualDuration, err = time.ParseDuration(fmt.Sprintf("%vh", timeValue))
		if err != nil {
			return 0, fmt.Errorf("invalid time range: %q", duration)
		}
	} else if strings.HasSuffix(duration, "d") {
		timeValue, err = strconv.Atoi(strings.Trim(duration, "d"))
		if err != nil {
			return 0, fmt.Errorf("invalid time range: %q", duration)
		}
		timeValue = timeValue * 24
		actualDuration, err = time.ParseDuration(fmt.Sprintf("%vh", timeValue))
		if err != nil {
			return 0, fmt.Errorf("invalid time range: %q", duration)
		}
	} else if strings.HasSuffix(duration, "h") || strings.HasSuffix(duration, "m") || strings.HasSuffix(duration, "s") || strings.HasSuffix(duration, "ms") {
		actualDuration, err = time.ParseDuration(duration)
		if err != nil {
			return 0, fmt.Errorf("invalid time range: %q", duration)
		}
	} else {
		return 0, fmt.Errorf("invalid time duration string: %q", duration)
	}
	return actualDuration, nil
}

type DownSamplingMeta struct {
	Duration             time.Duration
	DownsamplingInterval time.Duration
}

func isDedupEnabled() bool {
	return globalDedupInterval > 0
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
