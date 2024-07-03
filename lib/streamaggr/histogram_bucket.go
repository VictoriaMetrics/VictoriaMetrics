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
	state          [aggrStateSize]metrics.Histogram
	total          metrics.Histogram
	deleted        bool
	deleteDeadline int64
}

func newHistogramBucketAggrState() *histogramBucketAggrState {
	return &histogramBucketAggrState{}
}

func (as *histogramBucketAggrState) pushSamples(samples []pushSample, deleteDeadline int64, idx int) {
	for i := range samples {
		s := &samples[i]
		outputKey := getOutputKey(s.key)

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
			sv.state[idx].Update(s.value)
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

func (as *histogramBucketAggrState) flushState(ctx *flushCtx, flushTimestamp int64, idx int) {
	m := &as.m
	var staleOutputSamples int
	m.Range(func(k, v interface{}) bool {
		sv := v.(*histogramBucketStateValue)
		sv.mu.Lock()

		// check for stale entries
		deleted := flushTimestamp > sv.deleteDeadline
		if deleted {
			// Mark the current entry as deleted
			sv.deleted = deleted
			staleOutputSamples++
			sv.mu.Unlock()
			m.Delete(k)
			return true
		}
		sv.total.Merge(&sv.state[idx])
		total := &sv.total
		sv.state[idx] = metrics.Histogram{}
		sv.mu.Unlock()
		key := k.(string)
		total.VisitNonZeroBuckets(func(vmrange string, count uint64) {
			ctx.appendSeriesWithExtraLabel(key, "histogram_bucket", flushTimestamp, float64(count), "vmrange", vmrange)
		})
		return true
	})
	ctx.a.staleOutputSamples["histogram_bucket"].Add(staleOutputSamples)
}
