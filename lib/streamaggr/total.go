package streamaggr

import (
	"math"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
)

func totalInitFn(ignoreFirstSampleInterval time.Duration, resetTotalOnFlush, keepFirstSample bool) aggrValuesInitFn {
	ignoreFirstSampleDeadline := time.Now().Add(ignoreFirstSampleInterval)
	return func(values []aggrValue) []aggrValue {
		shared := &totalAggrValueShared{
			lastValues: make(map[string]totalLastValue),
		}
		for i := range values {
			values[i] = &totalAggrValue{
				keepFirstSample:           keepFirstSample,
				resetTotalOnFlush:         resetTotalOnFlush,
				shared:                    shared,
				ignoreFirstSampleDeadline: uint64(ignoreFirstSampleDeadline.Unix()),
			}
		}
		return values
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

func (av *totalAggrValue) pushSample(ctx *pushSampleCtx) {
	shared := av.shared
	inputKey := ctx.inputKey
	currentTime := fasttime.UnixTimestamp()
	keepFirstSample := av.keepFirstSample && currentTime >= av.ignoreFirstSampleDeadline
	lv, ok := shared.lastValues[inputKey]
	if ok || keepFirstSample {
		if ctx.sample.timestamp < lv.timestamp {
			// Skip out of order sample
			return
		}
		if ctx.sample.value >= lv.value {
			av.total += ctx.sample.value - lv.value
		} else {
			// counter reset
			av.total += ctx.sample.value
		}
	}
	lv.value = ctx.sample.value
	lv.timestamp = ctx.sample.timestamp
	lv.deleteDeadline = ctx.deleteDeadline

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
