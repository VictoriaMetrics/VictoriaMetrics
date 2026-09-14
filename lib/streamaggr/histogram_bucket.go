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

func (av *histogramBucketAggrValue) flush(_ aggrConfig, ctx *flushCtx, key string, _ bool) {
	av.shared.Merge(&av.h)
	av.h.Reset()
	av.shared.VisitNonZeroBuckets(func(vmrange string, count uint64) {
		ctx.appendSeriesWithExtraLabel(key, "histogram_bucket", float64(count), "vmrange", vmrange)
	})
}

func (av *histogramBucketAggrValue) state() any {
	return av.shared
}

func newHistogramBucketAggrConfig() aggrConfig {
	return &histogramBucketAggrConfig{}
}

type histogramBucketAggrConfig struct{}

func (*histogramBucketAggrConfig) getValue(s any) aggrValue {
	if s == nil {
		s = &metrics.Histogram{}
	}
	return &histogramBucketAggrValue{
		shared: s.(*metrics.Histogram),
	}
}
