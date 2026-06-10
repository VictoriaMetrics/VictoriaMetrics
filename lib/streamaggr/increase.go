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

type increaseAggrValueShared struct {
	lastValues map[string]increaseLastValue
}

type increaseAggrValue struct {
	total  float64
	shared *increaseAggrValueShared
}

func (av *increaseAggrValue) pushSample(c aggrConfig, sample *pushSample, key string, deleteDeadline int64) {
	ac := c.(*increaseAggrConfig)
	currentTime := fasttime.UnixTimestamp()
	keepFirstSample := ac.keepFirstSample && currentTime >= ac.ignoreFirstSampleDeadline
	lv, ok := av.shared.lastValues[key]
	if ok || keepFirstSample {
		if sample.timestamp < lv.timestamp {
			// Skip out of order sample
			return
		}
		if !sample.stateOnly {
			if sample.value >= lv.value {
				av.total += sample.value - lv.value
			} else {
				// counter reset
				av.total += sample.value
				ac.counterResetsTotal.Inc()
			}
		}
	}
	lv.value = sample.value
	lv.timestamp = sample.timestamp
	lv.deleteDeadline = deleteDeadline
	key = bytesutil.InternString(key)
	av.shared.lastValues[key] = lv
}

func (av *increaseAggrValue) flush(c aggrConfig, ctx *flushCtx, key string, isLast bool) {
	ac := c.(*increaseAggrConfig)
	suffix := ac.getSuffix()
	total := av.total
	av.total = 0
	lvs := av.shared.lastValues
	for lk, lv := range lvs {
		if ctx.flushTimestamp > lv.deleteDeadline || isLast {
			delete(lvs, lk)
		}
	}
	ctx.appendSeries(key, suffix, total)
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

type increaseAggrConfig struct {
	keepFirstSample bool

	// The first sample per each new series is ignored until this unix timestamp deadline in seconds even if keepFirstSample is set.
	ignoreFirstSampleDeadline uint64
	counterResetsTotal        *metrics.Counter
}

func (*increaseAggrConfig) getValue(s any) aggrValue {
	var shared *increaseAggrValueShared
	if s == nil {
		shared = &increaseAggrValueShared{
			lastValues: make(map[string]increaseLastValue),
		}
	} else {
		shared = s.(*increaseAggrValueShared)
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
