package streamaggr

import (
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/metrics"
)

// histogramBucketAggrState calculates output=histogram_bucket, e.g. VictoriaMetrics histogram over input samples.
type histogramBucketAggrState struct {
	m sync.Map
}

type histogramBucketStateValue struct {
	mu             sync.Mutex
	h              metrics.Histogram
	deleteDeadline int64
	deleted        bool
}

func newHistogramBucketAggrState() *histogramBucketAggrState {
	return &histogramBucketAggrState{}
}

func (as *histogramBucketAggrState) pushSamples(samples []pushSample, deleteDeadline int64, includeInputKey bool) {
	for i := range samples {
		s := &samples[i]
		outputKey := getOutputKey(s.key, includeInputKey)

	again:
		v, ok := as.m.Load(outputKey)
		if !ok {
			// The entry is missing in the map. Try creating it.
			v = &histogramBucketStateValue{}
			outputKey = bytesutil.InternString(outputKey)
			vNew, loaded := as.m.LoadOrStore(outputKey, v)
			if loaded {
				// Use the entry created by a concurrent goroutine.
				v = vNew
			}
		}
		sv := v.(*histogramBucketStateValue)
		sv.mu.Lock()
		deleted := sv.deleted
		if !deleted {
			sv.h.Update(s.value)
			sv.deleteDeadline = deleteDeadline
		}
		sv.mu.Unlock()
		if deleted {
			// The entry has been deleted by the concurrent call to flushState
			// Try obtaining and updating the entry again.
			goto again
		}
	}
}

func (as *histogramBucketAggrState) removeOldEntries(ctx *flushCtx) {
	m := &as.m
	m.Range(func(k, v any) bool {
		sv := v.(*histogramBucketStateValue)

		sv.mu.Lock()
		deleted := ctx.flushTimestamp > sv.deleteDeadline
		if deleted {
			// Mark the current entry as deleted
			sv.deleted = deleted
		}
		sv.mu.Unlock()

		if deleted {
			key := k.(string)
			ctx.a.lc.Delete(bytesutil.ToUnsafeBytes(key), ctx.flushTimestamp)
			m.Delete(k)
		}
		return true
	})
}

func (as *histogramBucketAggrState) flushState(ctx *flushCtx) {
	as.removeOldEntries(ctx)

	m := &as.m
	m.Range(func(k, v any) bool {
		sv := v.(*histogramBucketStateValue)
		sv.mu.Lock()
		if !sv.deleted {
			key := k.(string)
			sv.h.VisitNonZeroBuckets(func(vmrange string, count uint64) {
				ctx.appendSeriesWithExtraLabel(key, "histogram_bucket", float64(count), "vmrange", vmrange)
			})
		}
		sv.mu.Unlock()
		return true
	})
}
