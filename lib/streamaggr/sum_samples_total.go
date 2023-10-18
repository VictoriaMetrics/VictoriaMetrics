package streamaggr

import (
	"math"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
)

// sumSamplesTotalAggrState calculates output=sum_samples, e.g. the sum over input samples.
type sumSamplesTotalAggrState struct {
	m                 sync.Map
	intervalSecs      uint64
	stalenessSecs     uint64
	lastPushTimestamp uint64
}

type sumSamplesTotalStateValue struct {
	mu             sync.Mutex
	sum            float64
	samplesCount   uint64
	deleted        bool
	deleteDeadline uint64
}

func newSumSamplesTotalAggrState(interval time.Duration, stalenessInterval time.Duration) *sumSamplesTotalAggrState {
	return &sumSamplesTotalAggrState{
		intervalSecs:  roundDurationToSecs(interval),
		stalenessSecs: roundDurationToSecs(stalenessInterval),
	}
}

func (as *sumSamplesTotalAggrState) pushSample(_, outputKey string, value float64) {
	currentTime := fasttime.UnixTimestamp()
	deleteDeadline := currentTime + as.stalenessSecs

again:
	v, ok := as.m.Load(outputKey)
	if !ok {
		// The entry is missing in the map. Try creating it.
		v = &sumSamplesTotalStateValue{
			sum: value,
		}
		vNew, loaded := as.m.LoadOrStore(outputKey, v)
		if !loaded {
			// The new entry has been successfully created.
			return
		}
		// Use the entry created by a concurrent goroutine.
		v = vNew
	}
	sv := v.(*sumSamplesTotalStateValue)
	sv.mu.Lock()
	deleted := sv.deleted
	if !deleted {
		sv.sum += value
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

func (as *sumSamplesTotalAggrState) removeOldEntries(currentTime uint64) {
	m := &as.m
	m.Range(func(k, v interface{}) bool {
		sv := v.(*sumSamplesTotalStateValue)

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

func (as *sumSamplesTotalAggrState) appendSeriesForFlush(ctx *flushCtx) {
	currentTime := fasttime.UnixTimestamp()
	currentTimeMsec := int64(currentTime) * 1000

	as.removeOldEntries(currentTime)

	m := &as.m
	m.Range(func(k, v interface{}) bool {
		sv := v.(*sumSamplesTotalStateValue)
		sv.mu.Lock()
		sum := sv.sum
		if math.Abs(sv.sum) >= (1 << 53) {
			// It is time to reset the entry, since it starts losing float64 precision
			sv.sum = 0
		}
		sv.mu.Unlock()
		key := k.(string)
		ctx.appendSeries(key, as.getOutputName(), currentTimeMsec, sum)
		return true
	})
	as.lastPushTimestamp = currentTime
}

func (as *sumSamplesTotalAggrState) getOutputName() string {
	return "sum_samples_total"
}

func (as *sumSamplesTotalAggrState) getStateRepresentation(suffix string) []aggrStateRepresentation {
	result := make([]aggrStateRepresentation, 0)
	as.m.Range(func(k, v any) bool {
		value := v.(*sumSamplesTotalStateValue)
		value.mu.Lock()
		defer value.mu.Unlock()
		if value.deleted {
			return true
		}
		result = append(result, aggrStateRepresentation{
			metric:            getLabelsStringFromKey(k.(string), suffix, as.getOutputName()),
			currentValue:      value.sum,
			lastPushTimestamp: as.lastPushTimestamp,
			nextPushTimestamp: as.lastPushTimestamp + as.intervalSecs,
			samplesCount:      value.samplesCount,
		})
		return true
	})
	return result
}
