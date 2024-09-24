package streamaggr

import (
	"github.com/VictoriaMetrics/metrics"
)

// histogramBucketAggrValue calculates output=histogram_bucket, e.g. VictoriaMetrics histogram over input samples.
type histogramBucketAggrValue struct {
	h     metrics.Histogram
	state metrics.Histogram
}

func (sv *histogramBucketAggrValue) pushSample(ctx *pushSampleCtx) {
	sv.h.Update(ctx.sample.value)
}

func (sv *histogramBucketAggrValue) flush(ctx *flushCtx, key string) {
	total := &sv.state
	total.Merge(&sv.h)
	total.VisitNonZeroBuckets(func(vmrange string, count uint64) {
		ctx.appendSeriesWithExtraLabel(key, "histogram_bucket", float64(count), "vmrange", vmrange)
	})
	total.Reset()
}
