package streamaggr

import (
	"math"
)

type sumSamplesAggrValue struct {
	sum float64
}

func (av *sumSamplesAggrValue) pushSample(_ aggrConfig, sample *pushSample, _ string, _ int64) {
	if math.Abs(av.sum) >= (1 << 53) {
		// It is time to reset the entry, since it starts losing float64 precision
		av.sum = 0
	}
	av.sum += sample.value
}

func (av *sumSamplesAggrValue) flush(c aggrConfig, ctx *flushCtx, key string, _ bool) {
	ac := c.(*sumSamplesAggrConfig)
	if ac.resetTotalOnFlush {
		ctx.appendSeries(key, "sum_samples", av.sum)
		av.sum = 0
		return
	}
	ctx.appendSeries(key, "sum_samples_total", av.sum)
}

func (*sumSamplesAggrValue) state() any {
	return nil
}

func newSumSamplesAggrConfig(resetTotalOnFlush bool) aggrConfig {
	return &sumSamplesAggrConfig{
		resetTotalOnFlush: resetTotalOnFlush,
	}
}

type sumSamplesAggrConfig struct {
	resetTotalOnFlush bool
}

func (*sumSamplesAggrConfig) getValue(_ any) aggrValue {
	return &sumSamplesAggrValue{}
}
