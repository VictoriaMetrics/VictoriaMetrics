package streamaggr

import (
	"math"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
)

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
	total  float64
	shared *totalAggrValueShared
}

func (av *totalAggrValue) pushSample(c aggrConfig, sample *pushSample, key string, deleteDeadline int64) {
	ac := c.(*totalAggrConfig)
	currentTime := fasttime.UnixTimestamp()
	keepFirstSample := ac.keepFirstSample && currentTime >= ac.ignoreFirstSampleDeadline
	lv, ok := av.shared.lastValues[key]
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
	key = bytesutil.InternString(key)
	av.shared.lastValues[key] = lv
}

func (av *totalAggrValue) flush(c aggrConfig, ctx *flushCtx, key string, isLast bool) {
	ac := c.(*totalAggrConfig)
	suffix := ac.getSuffix()
	// check for stale entries
	total := av.shared.total + av.total
	av.total = 0
	lvs := av.shared.lastValues
	for lk, lv := range lvs {
		if ctx.flushTimestamp > lv.deleteDeadline || isLast {
			delete(lvs, lk)
		}
	}
	if ac.resetTotalOnFlush {
		av.shared.total = 0
	} else if math.Abs(total) >= (1 << 53) {
		// It is time to reset the entry, since it starts losing float64 precision
		av.shared.total = 0
	} else {
		av.shared.total = total
	}
	ctx.appendSeries(key, suffix, total)
}

func (av *totalAggrValue) state() any {
	return av.shared
}

func newTotalAggrConfig(ignoreFirstSampleIntervalSecs uint64, resetTotalOnFlush, keepFirstSample bool) aggrConfig {
	ignoreFirstSampleDeadline := fasttime.UnixTimestamp() + ignoreFirstSampleIntervalSecs
	return &totalAggrConfig{
		keepFirstSample:           keepFirstSample,
		resetTotalOnFlush:         resetTotalOnFlush,
		ignoreFirstSampleDeadline: ignoreFirstSampleDeadline,
	}
}

type totalAggrConfig struct {
	resetTotalOnFlush bool

	// Whether to take into account the first sample in new time series when calculating the output value.
	keepFirstSample bool

	// The first sample per each new series is ignored until this unix timestamp deadline in seconds even if keepFirstSample is set.
	// This allows avoiding an initial spike of the output values at startup when new time series
	// cannot be distinguished from already existing series. This is tracked with ignoreFirstSampleDeadline.
	ignoreFirstSampleDeadline uint64
}

func (*totalAggrConfig) getValue(s any) aggrValue {
	var shared *totalAggrValueShared
	if s == nil {
		shared = &totalAggrValueShared{
			lastValues: make(map[string]totalLastValue),
		}
	} else {
		shared = s.(*totalAggrValueShared)
	}
	return &totalAggrValue{
		shared: shared,
	}
}

func (ac *totalAggrConfig) getSuffix() string {
	if ac.resetTotalOnFlush {
		if ac.keepFirstSample {
			return "increase"
		}
		return "increase_prometheus"
	}
	if ac.keepFirstSample {
		return "total"
	}
	return "total_prometheus"
}
