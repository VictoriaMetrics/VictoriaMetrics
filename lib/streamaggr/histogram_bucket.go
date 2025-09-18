package streamaggr

import (
	"github.com/VictoriaMetrics/metrics"
)

// histogramBucketAggrValue calculates output=histogram_bucket, e.g. VictoriaMetrics histogram over input samples.
type histogramBucketAggrValue struct {
	h      metrics.Histogram
	shared *metrics.Histogram
}

func (av *histogramBucketAggrValue) pushSample(_ aggrConfig, sample *pushSample, _ string, _ int64) {
	av.h.Update(sample.value)
}

func (av *histogramBucketAggrValue) flush(c aggrConfig, ctx *flushCtx, key string, _ bool) {
	ac := c.(*histogramBucketAggrConfig)
	shared := av.shared
	if ac.useSharedState {
		shared.Merge(&av.h)
		av.h.Reset()
	} else {
		shared = &av.h
	}
	shared.VisitNonZeroBuckets(func(vmrange string, count uint64) {
		ctx.appendSeriesWithExtraLabel(key, "histogram_bucket", float64(count), "vmrange", vmrange)
	})
}

func (av *histogramBucketAggrValue) state() any {
	return av.shared
}

func newHistogramBucketAggrConfig(useSharedState bool) aggrConfig {
	return &histogramBucketAggrConfig{
		useSharedState: useSharedState,
	}
}

type histogramBucketAggrConfig struct {
	useSharedState bool
}

func (ac *histogramBucketAggrConfig) getValue(s any) aggrValue {
	var shared *metrics.Histogram
	if ac.useSharedState {
		if s == nil {
			shared = &metrics.Histogram{}
		} else {
			shared = s.(*metrics.Histogram)
		}
	}
	return &histogramBucketAggrValue{
		shared: shared,
	}
}
