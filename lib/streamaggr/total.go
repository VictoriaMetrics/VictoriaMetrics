package streamaggr

import (
	"math"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
)

func totalInitFn(ignoreFirstSampleIntervalSecs uint64, resetTotalOnFlush, keepFirstSample bool) aggrValuesFn {
	ignoreFirstSampleDeadline := fasttime.UnixTimestamp() + ignoreFirstSampleIntervalSecs
	return func(v *aggrValues, enableWindows bool) {
		shared := &totalAggrValueShared{
			lastValues: make(map[string]totalLastValue),
		}
		v.blue = append(v.green, &totalAggrValue{
			keepFirstSample:           keepFirstSample,
			resetTotalOnFlush:         resetTotalOnFlush,
			shared:                    shared,
			ignoreFirstSampleDeadline: ignoreFirstSampleDeadline,
		})
		if enableWindows {
			v.green = append(v.green, &totalAggrValue{
				keepFirstSample:           keepFirstSample,
				resetTotalOnFlush:         resetTotalOnFlush,
				shared:                    shared,
				ignoreFirstSampleDeadline: ignoreFirstSampleDeadline,
			})
		}
	}
}

type totalLastValue struct {
	value          float64
	timestamp      int64
	deleteDeadline int64
}

type totalAggrValueShared struct {
	lastValues map[string]totalLastValue
	total      float64
}

type totalAggrValue struct {
	total             float64
	resetTotalOnFlush bool
	shared            *totalAggrValueShared

	// Whether to take into account the first sample in new time series when calculating the output value.
	keepFirstSample bool

	// The first sample per each new series is ignored until this unix timestamp deadline in seconds even if keepFirstSample is set.
	// This allows avoiding an initial spike of the output values at startup when new time series
	// cannot be distinguished from already existing series. This is tracked with ignoreFirstSampleDeadline.
	ignoreFirstSampleDeadline uint64
}

func (av *totalAggrValue) pushSample(inputKey string, sample *pushSample, deleteDeadline int64) {
	shared := av.shared
	currentTime := fasttime.UnixTimestamp()
	keepFirstSample := av.keepFirstSample && currentTime >= av.ignoreFirstSampleDeadline
	lv, ok := shared.lastValues[inputKey]
	if ok || keepFirstSample {
		if sample.timestamp < lv.timestamp {
			// Skip out of order sample
			return
		}
		if sample.value >= lv.value {
			av.total += sample.value - lv.value
		} else {
			// counter reset
			av.total += sample.value
		}
	}
	lv.value = sample.value
	lv.timestamp = sample.timestamp
	lv.deleteDeadline = deleteDeadline

	inputKey = bytesutil.InternString(inputKey)
	shared.lastValues[inputKey] = lv
}

func (av *totalAggrValue) flush(ctx *flushCtx, key string) {
	suffix := av.getSuffix()
	// check for stale entries
	total := av.shared.total + av.total
	av.total = 0
	lvs := av.shared.lastValues
	for lk, lv := range lvs {
		if ctx.flushTimestamp > lv.deleteDeadline {
			delete(lvs, lk)
		}
	}
	if av.resetTotalOnFlush {
		av.shared.total = 0
	} else if math.Abs(total) >= (1 << 53) {
		// It is time to reset the entry, since it starts losing float64 precision
		av.shared.total = 0
	} else {
		av.shared.total = total
	}
	ctx.appendSeries(key, suffix, total)
}

func (av *totalAggrValue) getSuffix() string {
	// Note: this function is at hot path, so it shouldn't allocate.
	if av.resetTotalOnFlush {
		if av.keepFirstSample {
			return "increase"
		}
		return "increase_prometheus"
	}
	if av.keepFirstSample {
		return "total"
	}
	return "total_prometheus"
}
