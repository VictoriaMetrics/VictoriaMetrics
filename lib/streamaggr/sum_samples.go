package streamaggr

import (
	"math"
)

// sumSamplesAggrValueShared holds the cumulative total shared between
// the blue and green windows when enable_windows is true.
type sumSamplesAggrValueShared struct {
	total float64
}

type sumSamplesAggrValue struct {
	// delta accumulates samples within the current flush window.
	delta float64
	// shared holds the cross-window cumulative total (non-nil when enable_windows is true).
	shared *sumSamplesAggrValueShared
}

func (av *sumSamplesAggrValue) pushSample(_ aggrConfig, sample *pushSample, _ string, _ int64) {
	if av.shared != nil {
		if math.Abs(av.shared.total+av.delta) >= (1 << 53) {
			av.shared.total = 0
			av.delta = 0
		}
	} else {
		if math.Abs(av.delta) >= (1 << 53) {
			av.delta = 0
		}
	}
	av.delta += sample.value
}

func (av *sumSamplesAggrValue) flush(c aggrConfig, ctx *flushCtx, key string, _ bool) {
	ac := c.(*sumSamplesAggrConfig)
	if ac.resetTotalOnFlush {
		ctx.appendSeries(key, "sum_samples", av.delta)
		av.delta = 0
		return
	}
	if av.shared != nil {
		// enable_windows: accumulate delta into shared total, output the combined value.
		total := av.shared.total + av.delta
		av.delta = 0
		av.shared.total = total
		ctx.appendSeries(key, "sum_samples_total", total)
	} else {
		ctx.appendSeries(key, "sum_samples_total", av.delta)
	}
}

func (av *sumSamplesAggrValue) state() any {
	if av.shared != nil {
		return av.shared
	}
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

func (ac *sumSamplesAggrConfig) getValue(s any) aggrValue {
	if ac.resetTotalOnFlush {
		// sum_samples: no shared state needed.
		return &sumSamplesAggrValue{}
	}
	// sum_samples_total: share cumulative total between windows.
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
