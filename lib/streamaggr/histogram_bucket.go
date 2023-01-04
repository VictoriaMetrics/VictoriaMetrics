package streamaggr

import (
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/metrics"
)

// histogramBucketAggrState calculates output=histogramBucket, e.g. VictoriaMetrics histogram over input samples.
type histogramBucketAggrState struct {
	m sync.Map

	intervalSecs uint64
}

type histogramBucketStateValue struct {
	mu             sync.Mutex
	h              metrics.Histogram
	deleteDeadline uint64
	deleted        bool
}

func newHistogramBucketAggrState(interval time.Duration) *histogramBucketAggrState {
	intervalSecs := uint64(interval.Seconds() + 1)
	return &histogramBucketAggrState{
		intervalSecs: intervalSecs,
	}
}

func (as *histogramBucketAggrState) pushSample(inputKey, outputKey string, value float64) {
	currentTime := fasttime.UnixTimestamp()
	deleteDeadline := currentTime + 2*as.intervalSecs

again:
	v, ok := as.m.Load(outputKey)
	if !ok {
		// The entry is missing in the map. Try creating it.
		v = &histogramBucketStateValue{}
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
		sv.h.Update(value)
		sv.deleteDeadline = deleteDeadline
	}
	sv.mu.Unlock()
	if deleted {
		// The entry has been deleted by the concurrent call to appendSeriesForFlush
		// Try obtaining and updating the entry again.
		goto again
	}
}

func (as *histogramBucketAggrState) removeOldEntries(currentTime uint64) {
	m := &as.m
	m.Range(func(k, v interface{}) bool {
		sv := v.(*histogramBucketStateValue)

		sv.mu.Lock()
		deleted := currentTime > sv.deleteDeadline
		if deleted {
			// Mark the current entry as deleted
			sv.deleted = deleted
		}
		sv.mu.Unlock()

		if deleted {
			m.Delete(k)
		}
		return true
	})
}

func (as *histogramBucketAggrState) appendSeriesForFlush(ctx *flushCtx) {
	currentTime := fasttime.UnixTimestamp()
	currentTimeMsec := int64(currentTime) * 1000

	as.removeOldEntries(currentTime)

	m := &as.m
	m.Range(func(k, v interface{}) bool {
		sv := v.(*histogramBucketStateValue)
		sv.mu.Lock()
		if !sv.deleted {
			key := k.(string)
			sv.h.VisitNonZeroBuckets(func(vmrange string, count uint64) {
				ctx.appendSeriesWithExtraLabel(key, "histogram_bucket", currentTimeMsec, float64(count), "vmrange", vmrange)
			})
		}
		sv.mu.Unlock()
		return true
	})
}
