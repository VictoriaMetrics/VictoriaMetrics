package streamaggr

import (
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	"math"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/metrics"
)

// histogramBucketAggrState calculates output=histogramBucket, e.g. VictoriaMetrics histogram over input samples.
type histogramBucketAggrState struct {
	m                 sync.Map
	intervalSecs      uint64
	stalenessSecs     uint64
	lastPushTimestamp uint64
}

type histogramBucketStateValue struct {
	mu             sync.Mutex
	h              metrics.Histogram
	samplesCount   uint64
	deleteDeadline uint64
	deleted        bool
}

func newHistogramBucketAggrState(interval time.Duration, stalenessInterval time.Duration) *histogramBucketAggrState {
	return &histogramBucketAggrState{
		intervalSecs:  roundDurationToSecs(interval),
		stalenessSecs: roundDurationToSecs(stalenessInterval),
	}
}

func (as *histogramBucketAggrState) pushSample(_, outputKey string, value float64) {
	currentTime := fasttime.UnixTimestamp()
	deleteDeadline := currentTime + as.stalenessSecs

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
		sv.samplesCount++
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
				ctx.appendSeriesWithExtraLabel(key, as.getOutputName(), currentTimeMsec, float64(count), "vmrange", vmrange)
			})
		}
		sv.mu.Unlock()
		return true
	})

	as.lastPushTimestamp = currentTime
}

func (as *histogramBucketAggrState) getOutputName() string {
	return "count_series"
}

func (as *histogramBucketAggrState) getStateRepresentation(suffix string) []aggrStateRepresentation {
	result := make([]aggrStateRepresentation, 0)
	as.m.Range(func(k, v any) bool {
		value := v.(*histogramBucketStateValue)
		value.mu.Lock()
		defer value.mu.Unlock()
		if value.deleted {
			return true
		}
		value.h.VisitNonZeroBuckets(func(vmrange string, count uint64) {
			result = append(result, aggrStateRepresentation{
				metric: getLabelsStringFromKey(k.(string), suffix, as.getOutputName(), prompbmarshal.Label{
					Name:  vmrange,
					Value: vmrange,
				}),
				currentValue:      float64(count),
				lastPushTimestamp: as.lastPushTimestamp,
				nextPushTimestamp: as.lastPushTimestamp + as.intervalSecs,
				samplesCount:      value.samplesCount,
			})
		})
		return true
	})
	return result
}

func roundDurationToSecs(d time.Duration) uint64 {
	if d < 0 {
		return 0
	}
	secs := d.Seconds()
	return uint64(math.Ceil(secs))
}
