package streamaggr

import (
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
)

// countSamplesTotalAggrState calculates output=countSamplesTotal, e.g. the count of input samples.
type countSamplesTotalAggrState struct {
	m                 sync.Map
	intervalSecs      uint64
	stalenessSecs     uint64
	lastPushTimestamp uint64
}

type countSamplesTotalStateValue struct {
	mu             sync.Mutex
	n              uint64
	deleted        bool
	deleteDeadline uint64
}

func newCountSamplesTotalAggrState(interval time.Duration, stalenessInterval time.Duration) *countSamplesTotalAggrState {
	return &countSamplesTotalAggrState{
		intervalSecs:  roundDurationToSecs(interval),
		stalenessSecs: roundDurationToSecs(stalenessInterval),
	}
}

func (as *countSamplesTotalAggrState) pushSample(_, outputKey string, _ float64) {
	currentTime := fasttime.UnixTimestamp()
	deleteDeadline := currentTime + as.stalenessSecs

again:
	v, ok := as.m.Load(outputKey)
	if !ok {
		// The entry is missing in the map. Try creating it.
		v = &countSamplesTotalStateValue{
			n: 1,
		}
		vNew, loaded := as.m.LoadOrStore(outputKey, v)
		if !loaded {
			// The new entry has been successfully created.
			return
		}
		// Use the entry created by a concurrent goroutine.
		v = vNew
	}
	sv := v.(*countSamplesTotalStateValue)
	sv.mu.Lock()
	deleted := sv.deleted
	if !deleted {
		sv.n++
		sv.deleteDeadline = deleteDeadline
	}
	sv.mu.Unlock()
	if deleted {
		// The entry has been deleted by the concurrent call to appendSeriesForFlush
		// Try obtaining and updating the entry again.
		goto again
	}
}

func (as *countSamplesTotalAggrState) removeOldEntries(currentTime uint64) {
	m := &as.m
	m.Range(func(k, v interface{}) bool {
		sv := v.(*countSamplesTotalStateValue)

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

func (as *countSamplesTotalAggrState) appendSeriesForFlush(ctx *flushCtx) {
	currentTime := fasttime.UnixTimestamp()
	currentTimeMsec := int64(currentTime) * 1000

	as.removeOldEntries(currentTime)

	m := &as.m
	m.Range(func(k, v interface{}) bool {
		sv := v.(*countSamplesTotalStateValue)
		sv.mu.Lock()
		n := sv.n
		sv.mu.Unlock()
		key := k.(string)
		ctx.appendSeries(key, as.getOutputName(), currentTimeMsec, float64(n))
		return true
	})
	as.lastPushTimestamp = currentTime
}

func (as *countSamplesTotalAggrState) getOutputName() string {
	return "count_samples_total"
}

func (as *countSamplesTotalAggrState) getStateRepresentation(suffix string) []aggrStateRepresentation {
	result := make([]aggrStateRepresentation, 0)
	as.m.Range(func(k, v any) bool {
		value := v.(*countSamplesTotalStateValue)
		value.mu.Lock()
		defer value.mu.Unlock()
		if value.deleted {
			return true
		}
		result = append(result, aggrStateRepresentation{
			metric:            getLabelsStringFromKey(k.(string), suffix, as.getOutputName()),
			currentValue:      float64(value.n),
			lastPushTimestamp: as.lastPushTimestamp,
			nextPushTimestamp: as.lastPushTimestamp + as.intervalSecs,
			samplesCount:      value.n,
		})
		return true
	})
	return result
}
