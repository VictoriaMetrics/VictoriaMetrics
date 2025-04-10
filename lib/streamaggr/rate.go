package streamaggr

import (
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"sync"
)

var rateAggrSharedValuePool sync.Pool

func putRateAggrSharedValue(v *rateAggrSharedValue) {
	v.reset()
	rateAggrSharedValuePool.Put(v)
}

func getRateAggrSharedValue(isGreen bool) *rateAggrSharedValue {
	v := rateAggrSharedValuePool.Get()
	if v == nil {
		v = &rateAggrSharedValue{}
	}
	av := v.(*rateAggrSharedValue)
	if isGreen {
		av.green = getRateAggrStateValue()
	} else {
		av.blue = getRateAggrStateValue()
	}
	return av
}

var rateAggrStateValuePool sync.Pool

func putRateAggrStateValue(v *rateAggrStateValue) {
	v.timestamp = 0
	v.increase = 0
	rateAggrStateValuePool.Put(v)
}

func getRateAggrStateValue() *rateAggrStateValue {
	v := rateAggrStateValuePool.Get()
	if v == nil {
		return &rateAggrStateValue{}
	}
	return v.(*rateAggrStateValue)
}

// rateAggrSharedValue calculates output=rate_avg and rate_sum, e.g. the average per-second increase rate for counter metrics.
type rateAggrSharedValue struct {
	value          float64
	deleteDeadline int64

	// prevTimestamp is the timestamp of the last registered sample in the previous aggregation interval
	prevTimestamp int64
	blue          *rateAggrStateValue
	green         *rateAggrStateValue
}

func (v *rateAggrSharedValue) getState(isGreen bool) *rateAggrStateValue {
	if isGreen {
		if v.green == nil {
			v.green = getRateAggrStateValue()
		}
		return v.green
	}
	if v.blue == nil {
		v.blue = getRateAggrStateValue()
	}
	return v.blue
}

func (v *rateAggrSharedValue) reset() {
	v.value = 0
	v.deleteDeadline = 0
	v.prevTimestamp = 0
	if v.blue != nil {
		putRateAggrStateValue(v.blue)
		v.blue = nil
	}
	if v.green != nil {
		putRateAggrStateValue(v.green)
		v.green = nil
	}
}

type rateAggrStateValue struct {
	// increase stores cumulative increase for the current time series on the current aggregation interval
	increase  float64
	timestamp int64
}

type rateAggrValue struct {
	shared  map[string]*rateAggrSharedValue
	isGreen bool
}

func (av *rateAggrValue) pushSample(_ aggrConfig, sample *pushSample, key string, deleteDeadline int64) {
	var state *rateAggrStateValue
	sv, ok := av.shared[key]
	if ok {
		state = sv.getState(av.isGreen)
		if sample.timestamp < state.timestamp {
			// Skip out of order sample
			return
		}
		if sample.value >= sv.value {
			state.increase += sample.value - sv.value
		} else {
			// counter reset
			state.increase += sample.value
		}
	} else {
		sv = getRateAggrSharedValue(av.isGreen)
		sv.prevTimestamp = sample.timestamp
		key = bytesutil.InternString(key)
		av.shared[key] = sv
		state = sv.getState(av.isGreen)
	}
	sv.value = sample.value
	sv.deleteDeadline = deleteDeadline
	state.timestamp = sample.timestamp
}

func (av *rateAggrValue) flush(c aggrConfig, ctx *flushCtx, key string, isLast bool) {
	ac := c.(*rateAggrConfig)
	var state *rateAggrStateValue
	suffix := ac.getSuffix()
	rate := 0.0
	countSeries := 0
	for sk, sv := range av.shared {
		if ctx.flushTimestamp > sv.deleteDeadline {
			delete(av.shared, sk)
			putRateAggrSharedValue(sv)
			continue
		}
		if sv.prevTimestamp == 0 {
			continue
		}
		state = sv.getState(av.isGreen)
		d := float64(state.timestamp-sv.prevTimestamp) / 1000
		if d > 0 {
			rate += state.increase / d
			countSeries++
		}
		sv.prevTimestamp = state.timestamp
		state.timestamp = 0
		state.increase = 0
		if isLast {
			delete(av.shared, sk)
			putRateAggrSharedValue(sv)
		} else {
			av.shared[sk] = sv
		}
	}

	if countSeries == 0 {
		return
	}
	if ac.isAvg {
		rate /= float64(countSeries)
	}
	ctx.appendSeries(key, suffix, rate)
}

func (av *rateAggrValue) state() any {
	return av.shared
}

func newRateAggrConfig(isAvg bool) aggrConfig {
	return &rateAggrConfig{
		isAvg: isAvg,
	}
}

type rateAggrConfig struct {
	isAvg bool
}

func (*rateAggrConfig) getValue(s any) aggrValue {
	var shared map[string]*rateAggrSharedValue
	if s == nil {
		shared = make(map[string]*rateAggrSharedValue)
	} else {
		shared = s.(map[string]*rateAggrSharedValue)
	}
	return &rateAggrValue{
		shared:  shared,
		isGreen: s != nil,
	}
}

func (ac *rateAggrConfig) getSuffix() string {
	if ac.isAvg {
		return "rate_avg"
	}
	return "rate_sum"
}
