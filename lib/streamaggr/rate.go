package streamaggr

import (
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
)

func rateInitFn(isAvg bool) aggrValuesFn {
	return func(v *aggrValues, enableWindows bool) {
		shared := &rateAggrValueShared{
			lastValues: make(map[string]rateLastValue),
		}
		v.blue = append(v.blue, &rateAggrValue{
			isAvg:  isAvg,
			shared: shared,
			state:  make(map[string]rateAggrValueState),
		})
		if enableWindows {
			v.green = append(v.green, &rateAggrValue{
				isAvg:  isAvg,
				shared: shared,
				state:  make(map[string]rateAggrValueState),
			})
		}
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

func (av *rateAggrValue) pushSample(inputKey string, sample *pushSample, deleteDeadline int64) {
	sv := av.state[inputKey]
	lv, ok := av.shared.lastValues[inputKey]
	if ok {
		if sample.timestamp < sv.timestamp {
			// Skip out of order sample
			return
		}
		if sample.value >= lv.value {
			sv.increase += sample.value - lv.value
		} else {
			// counter reset
			sv.increase += sample.value
		}
	} else {
		lv.prevTimestamp = sample.timestamp
	}
	lv.value = sample.value
	lv.deleteDeadline = deleteDeadline
	sv.timestamp = sample.timestamp
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
