package streamaggr

import (
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/metrics"
)

// histogramBucketAggrState calculates output=histogram_bucket, e.g. VictoriaMetrics histogram over input samples.
type histogramBucketAggrState struct {
	m sync.Map

	stalenessMsecs int64
}

type histogramBucketStateValue struct {
	mu             sync.Mutex
	h              map[int64]*metrics.Histogram
	deleteDeadline int64
	deleted        bool
}

func newHistogramBucketAggrState(stalenessInterval time.Duration) *histogramBucketAggrState {
	stalenessMsecs := durationToMsecs(stalenessInterval)
	return &histogramBucketAggrState{
		stalenessMsecs: stalenessMsecs,
	}
}

func (as *histogramBucketAggrState) pushSamples(windows map[int64][]pushSample) {
	currentTime := fasttime.UnixMilli()
	deleteDeadline := currentTime + as.stalenessMsecs
	for ts, samples := range windows {
		for i := range samples {
			s := &samples[i]
			outputKey := getOutputKey(s.key)

		again:
			v, ok := as.m.Load(outputKey)
			if !ok {
				// The entry is missing in the map. Try creating it.
				v = &histogramBucketStateValue{
					h: make(map[int64]*metrics.Histogram),
				}
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
				if _, ok := sv.h[ts]; !ok {
					sv.h[ts] = &metrics.Histogram{}
				}
				sv.h[ts].Update(s.value)
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
}

func (as *histogramBucketAggrState) removeOldEntries(currentTime int64) {
	m := &as.m
	m.Range(func(k, v interface{}) bool {
		sv := v.(*histogramBucketStateValue)

		sv.mu.Lock()
		deleted := currentTime > sv.deleteDeadline || len(sv.h) == 0
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

func (as *histogramBucketAggrState) flushState(ctx *flushCtx, flushTimestamp int64) {
	as.removeOldEntries(flushTimestamp)
	m := &as.m
	fn := func(states map[int64]*metrics.Histogram) map[int64]*metrics.Histogram {
		output := make(map[int64]*metrics.Histogram)
		if flushTimestamp == -1 {
			for ts, state := range states {
				output[ts] = state
				delete(states, ts)
			}
		} else if state, ok := states[flushTimestamp]; ok {
			output[flushTimestamp] = state
			delete(states, flushTimestamp)
		}
		return output
	}
	m.Range(func(k, v interface{}) bool {
		sv := v.(*histogramBucketStateValue)
		sv.mu.Lock()
		if sv.deleted {
			sv.mu.Unlock()
			return true
		}
		states := fn(sv.h)
		sv.mu.Unlock()
		for ts, state := range states {
			key := k.(string)
			state.VisitNonZeroBuckets(func(vmrange string, count uint64) {
				ctx.appendSeriesWithExtraLabel(key, "histogram_bucket", ts, float64(count), "vmrange", vmrange)
			})
		}
		return true
	})
}

func durationToMsecs(d time.Duration) int64 {
	if d < 0 {
		return 0
	}
	return d.Milliseconds()
}
