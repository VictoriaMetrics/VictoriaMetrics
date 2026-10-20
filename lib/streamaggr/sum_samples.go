package streamaggr

import (
	"math"
)

type sumSamplesAggrValueShared struct {
	total float64
}

type sumSamplesAggrValue struct {
	delta  float64
	shared *sumSamplesAggrValueShared
}

func (av *sumSamplesAggrValue) pushSample(_ aggrConfig, sample *pushSample, _ string, _ int64) {
	av.delta += sample.value
}

func (av *sumSamplesAggrValue) flush(c aggrConfig, ctx *flushCtx, key string, _ bool) {
	ac := c.(*sumSamplesAggrConfig)
	if ac.resetTotalOnFlush {
		ctx.appendSeries(key, "sum_samples", av.delta)
		av.delta = 0
		return
	}
	total := av.shared.total + av.delta
	av.delta = 0
	if math.Abs(total) >= (1 << 53) {
		// It is time to reset the entry, since it starts losing float64 precision
		av.shared.total = 0
	} else {
		av.shared.total = total
	}
	ctx.appendSeries(key, "sum_samples_total", total)
}

func (av *sumSamplesAggrValue) state() any {
	return av.shared
}

func newSumSamplesAggrConfig(resetTotalOnFlush bool) aggrConfig {
	return &sumSamplesAggrConfig{
		resetTotalOnFlush: resetTotalOnFlush,
	}
}

type sumSamplesAggrConfig struct {
	resetTotalOnFlush bool
}

func (*sumSamplesAggrConfig) getValue(s any) aggrValue {
	var shared *sumSamplesAggrValueShared
	if s == nil {
		shared = &sumSamplesAggrValueShared{}
	} else {
		shared = s.(*sumSamplesAggrValueShared)
	}
	return &sumSamplesAggrValue{
		shared: shared,
	}
}
