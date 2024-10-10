package streamaggr

import (
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
)

func rateInitFn(isAvg bool) aggrValuesInitFn {
	return func(values []aggrValue) []aggrValue {
		shared := &rateAggrValueShared{
			lastValues: make(map[string]rateLastValue),
		}
		for i := range values {
			values[i] = &rateAggrValue{
				isAvg:  isAvg,
				shared: shared,
				state:  make(map[string]rateAggrValueState),
			}
		}
		return values
	}
}

// rateLastValue calculates output=rate_avg and rate_sum, e.g. the average per-second increase rate for counter metrics.
type rateLastValue struct {
	value          float64
	deleteDeadline int64

	// prevTimestamp is the timestamp of the last registered sample in the previous aggregation interval
	prevTimestamp int64
}

type rateAggrValueShared struct {
	lastValues map[string]rateLastValue
}

type rateAggrValueState struct {
	// increase stores cumulative increase for the current time series on the current aggregation interval
	increase  float64
	timestamp int64
}

type rateAggrValue struct {
	shared *rateAggrValueShared
	state  map[string]rateAggrValueState
	isAvg  bool
}

func (av *rateAggrValue) pushSample(ctx *pushSampleCtx) {
	sv := av.state[ctx.inputKey]
	inputKey := ctx.inputKey
	lv, ok := av.shared.lastValues[ctx.inputKey]
	if ok {
		if ctx.sample.timestamp < sv.timestamp {
			// Skip out of order sample
			return
		}
		if ctx.sample.value >= lv.value {
			sv.increase += ctx.sample.value - lv.value
		} else {
			// counter reset
			sv.increase += ctx.sample.value
		}
	} else {
		lv.prevTimestamp = ctx.sample.timestamp
	}
	lv.value = ctx.sample.value
	lv.deleteDeadline = ctx.deleteDeadline
	sv.timestamp = ctx.sample.timestamp
	inputKey = bytesutil.InternString(inputKey)
	av.state[inputKey] = sv
	av.shared.lastValues[inputKey] = lv
}

func (av *rateAggrValue) flush(ctx *flushCtx, key string) {
	suffix := av.getSuffix()
	rate := 0.0
	countSeries := 0
	lvs := av.shared.lastValues
	for lk, lv := range lvs {
		if ctx.flushTimestamp > lv.deleteDeadline {
			delete(lvs, lk)
			continue
		}
	}
	for sk, sv := range av.state {
		lv := lvs[sk]
		if lv.prevTimestamp == 0 {
			continue
		}
		d := float64(sv.timestamp-lv.prevTimestamp) / 1000
		if d > 0 {
			rate += sv.increase / d
			countSeries++
		}
		lv.prevTimestamp = sv.timestamp
		lvs[sk] = lv
		delete(av.state, sk)
	}
	if countSeries == 0 {
		return
	}
	if av.isAvg {
		rate /= float64(countSeries)
	}
	if rate > 0 {
		ctx.appendSeries(key, suffix, rate)
	}
}

func (av *rateAggrValue) getSuffix() string {
	if av.isAvg {
		return "rate_avg"
	}
	return "rate_sum"
}
