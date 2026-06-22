package streamaggr

import (
	"fmt"
	"math"

	"github.com/VictoriaMetrics/metrics"

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
	// The last value is stale, reset it.
	if ok && lv.deleteDeadline < int64(currentTime)*1000 {
		ok = false
	}
	if ok {
		if sample.timestamp < lv.timestamp {
			// Skip out of order sample
			return
		}
		if sample.value >= lv.value {
			av.total += sample.value - lv.value
		} else {
			// counter reset
			av.total += sample.value
			ac.counterResetsTotal.Inc()
		}
	} else if keepFirstSample {
		av.total += sample.value
	}
	lv.value = sample.value
	lv.timestamp = sample.timestamp
	lv.deleteDeadline = deleteDeadline
	key = bytesutil.InternString(key)
	av.shared.lastValues[key] = lv
}

func (av *totalAggrValue) flush(c aggrConfig, ctx *flushCtx, key string, isLast bool) {
	ac := c.(*totalAggrConfig)
	total := av.shared.total + av.total
	av.total = 0
	for lk, lv := range av.shared.lastValues {
		if ctx.flushTimestamp > lv.deleteDeadline || isLast {
			delete(av.shared.lastValues, lk)
		}
	}
	if math.Abs(total) >= (1 << 53) {
		// It is time to reset the entry, since it starts losing float64 precision
		av.shared.total = 0
	} else {
		av.shared.total = total
	}
	ctx.appendSeries(key, ac.getSuffix(), total)
}

func (av *totalAggrValue) state() any {
	return av.shared
}

func newTotalAggrConfig(ms *metrics.Set, metricLabels string, ignoreFirstSampleIntervalSecs uint64, keepFirstSample bool) aggrConfig {
	ignoreFirstSampleDeadline := fasttime.UnixTimestamp() + ignoreFirstSampleIntervalSecs
	cfg := &totalAggrConfig{
		keepFirstSample:           keepFirstSample,
		ignoreFirstSampleDeadline: ignoreFirstSampleDeadline,
	}
	cfg.counterResetsTotal = ms.NewCounter(fmt.Sprintf(`vm_streamaggr_counter_resets_total{%s}`, metricLabels))
	return cfg
}

type totalAggrConfig struct {
	// Whether to take into account the first sample in new time series when calculating the output value.
	keepFirstSample bool

	// The first sample per each new series is ignored until this unix timestamp deadline in seconds even if keepFirstSample is set.
	// This allows avoiding an initial spike of the output values at startup when new time series
	// cannot be distinguished from already existing series. This is tracked with ignoreFirstSampleDeadline.
	ignoreFirstSampleDeadline uint64
	counterResetsTotal        *metrics.Counter
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
	if ac.keepFirstSample {
		return "total"
	}
	return "total_prometheus"
}
