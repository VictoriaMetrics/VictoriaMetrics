package streamaggr

import (
	"fmt"

	"github.com/VictoriaMetrics/metrics"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
)

type increaseLastValue struct {
	value          float64
	timestamp      int64
	deleteDeadline int64
}

type increaseAggrConfig struct {
	keepFirstSample bool

	// The first sample per each new series is ignored until this unix timestamp deadline in seconds even if keepFirstSample is set.
	// This allows avoiding an initial spike of the output values at startup when new time series
	// cannot be distinguished from already existing series. This is tracked with ignoreFirstSampleDeadline.
	ignoreFirstSampleDeadline uint64
	counterResetsTotal        *metrics.Counter
}

type increaseAggrValue struct {
	total  *float64
	shared map[string]increaseLastValue
}

func (av *increaseAggrValue) pushSample(c aggrConfig, sample *pushSample, key string, deleteDeadline int64) {
	if av.total == nil {
		av.total = new(float64)
	}
	ac := c.(*increaseAggrConfig)
	currentTime := fasttime.UnixTimestamp()
	keepFirstSample := ac.keepFirstSample && currentTime >= ac.ignoreFirstSampleDeadline
	lv, ok := av.shared[key]
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
			*av.total += sample.value - lv.value
		} else {
			// counter reset
			*av.total += sample.value
			ac.counterResetsTotal.Inc()
		}
	} else if keepFirstSample {
		*av.total += sample.value
	}
	lv.value = sample.value
	lv.timestamp = sample.timestamp
	lv.deleteDeadline = deleteDeadline
	key = bytesutil.InternString(key)
	av.shared[key] = lv
}

func (av *increaseAggrValue) flush(c aggrConfig, ctx *flushCtx, key string, isLast bool) {
	ac := c.(*increaseAggrConfig)
	for lk, lv := range av.shared {
		if ctx.flushTimestamp > lv.deleteDeadline || isLast {
			delete(av.shared, lk)
		}
	}
	if av.total == nil {
		return
	}
	total := *av.total
	av.total = nil
	ctx.appendSeries(key, ac.getSuffix(), total)
}

func (av *increaseAggrValue) state() any {
	return av.shared
}

func newIncreaseAggrConfig(ms *metrics.Set, metricLabels string, ignoreFirstSampleIntervalSecs uint64, keepFirstSample bool) aggrConfig {
	ignoreFirstSampleDeadline := fasttime.UnixTimestamp() + ignoreFirstSampleIntervalSecs
	cfg := &increaseAggrConfig{
		keepFirstSample:           keepFirstSample,
		ignoreFirstSampleDeadline: ignoreFirstSampleDeadline,
	}
	cfg.counterResetsTotal = ms.NewCounter(fmt.Sprintf(`vm_streamaggr_counter_resets_total{%s}`, metricLabels))
	return cfg
}

func (*increaseAggrConfig) getValue(s any) aggrValue {
	var shared map[string]increaseLastValue
	if s == nil {
		shared = make(map[string]increaseLastValue)
	} else {
		shared = s.(map[string]increaseLastValue)
	}
	return &increaseAggrValue{
		shared: shared,
	}
}

func (ac *increaseAggrConfig) getSuffix() string {
	if ac.keepFirstSample {
		return "increase"
	}
	return "increase_prometheus"
}
