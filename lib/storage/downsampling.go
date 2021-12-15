package storage

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/metricsql"
)

// SetDownsamplingPeriods configures downsampling.
//
// The function must be called before opening or creating any storage.
func SetDownsamplingPeriods(periods []string, dedupInterval time.Duration) error {
	dsps, err := parseDownsamplingPeriods(periods)
	if err != nil {
		return err
	}
	dedupIntervalMs := dedupInterval.Milliseconds()
	if dedupIntervalMs > 0 {
		if len(dsps) > 0 && dsps[len(dsps)-1].Offset == 0 {
			return fmt.Errorf("-dedup.minScrapeInterval=%s cannot be used if -downsampling.period=%s contains zero offset", dedupInterval, periods)
		}
		// Deduplication is a special case of downsampling with zero offset.
		dsps = append(dsps, DownsamplingPeriod{
			Offset:   0,
			Interval: dedupIntervalMs,
		})
	}
	downsamplingPeriods = dsps
	return nil
}

// DownsamplingPeriod describes downsampling period
type DownsamplingPeriod struct {
	// Offset in milliseconds from the current time when the downsampling with the given interval must be applied
	Offset int64
	// Interval for downsampling - only a single sample is left per each interval
	Interval int64
}

// String implements interface
func (dsp DownsamplingPeriod) String() string {
	offset := time.Duration(dsp.Offset) * time.Millisecond
	interval := time.Duration(dsp.Interval) * time.Millisecond
	return fmt.Sprintf("%s:%s", offset, interval)
}

func (dsp *DownsamplingPeriod) parse(s string) error {
	idx := strings.Index(s, ":")
	if idx <= 0 {
		return fmt.Errorf("incorrect format for downsampling period: %s, want `offset:interval` format", s)
	}
	offsetStr, intervalStr := s[:idx], s[idx+1:]
	interval, err := metricsql.DurationValue(intervalStr, 0)
	if err != nil {
		return fmt.Errorf("incorrect interval: %s format for downsampling interval: %s err: %w", intervalStr, s, err)
	}
	offset, err := metricsql.DurationValue(offsetStr, 0)
	if err != nil {
		return fmt.Errorf("incorrect duration: %s format for downsampling offset: %s err: %w", offsetStr, s, err)
	}
	dsp.Interval = interval
	dsp.Offset = offset
	// sanity check
	if offset > 0 && interval > offset {
		return fmt.Errorf("downsampling interval=%d cannot exceed offset=%d", dsp.Interval, dsp.Offset)
	}
	return nil
}

var downsamplingPeriods []DownsamplingPeriod

// GetDedupInterval returns dedup interval, which must be applied to samples with the given timestamp.
func GetDedupInterval(timestamp int64) int64 {
	dsp := getDownsamplingPeriod(timestamp)
	return dsp.Interval
}

// getDownsamplingPeriod returns downsampling period, which must be used for the given timestamp
func getDownsamplingPeriod(timestamp int64) DownsamplingPeriod {
	offset := int64(fasttime.UnixTimestamp())*1000 - timestamp
	for _, dsp := range downsamplingPeriods {
		if offset >= dsp.Offset {
			return dsp
		}
	}
	return DownsamplingPeriod{}
}

func parseDownsamplingPeriods(periods []string) ([]DownsamplingPeriod, error) {
	if len(periods) == 0 {
		return nil, nil
	}
	var dsps []DownsamplingPeriod
	for _, period := range periods {
		var dsp DownsamplingPeriod
		if err := dsp.parse(period); err != nil {
			return nil, fmt.Errorf("cannot parse downsampling period %q: %w", period, err)
		}
		dsps = append(dsps, dsp)
	}
	sort.Slice(dsps, func(i, j int) bool {
		return dsps[i].Offset > dsps[j].Offset
	})
	dspPrev := dsps[0]
	// sanity checks.
	for _, dsp := range dsps[1:] {
		if dspPrev.Interval <= dsp.Interval {
			return nil, fmt.Errorf("prev downsampling interval %d must be bigger than the next interval %d", dspPrev.Interval, dsp.Interval)
		}
		if dspPrev.Offset == dsp.Offset {
			return nil, fmt.Errorf("duplicate downsampling offset: %d", dsp.Offset)
		}
		if dspPrev.Interval%dsp.Interval != 0 {
			return nil, fmt.Errorf("downsamping intervals must be multiples; prev: %d, current: %d", dspPrev.Interval, dsp.Interval)
		}
		dspPrev = dsp
	}
	return dsps, nil
}
