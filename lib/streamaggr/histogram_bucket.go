package streamaggr

import (
	"unsafe"

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

// sizeBytes returns an approximate, lower-bound size of av's state.
//
// metrics.Histogram lazily allocates its per-decimal bucket arrays and doesn't expose how many
// of them are currently allocated, so this only accounts for the fixed part of the struct
// (including the array of bucket pointers) and doesn't count the allocated bucket arrays themselves.
func (av *histogramBucketAggrValue) sizeBytes() uint64 {
	// unsafe.Sizeof(*av) already accounts for av.h, since it's an embedded value, not a pointer.
	return uint64(unsafe.Sizeof(*av)) + uint64(unsafe.Sizeof(*av.shared))
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
