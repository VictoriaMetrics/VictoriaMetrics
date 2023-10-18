package streamaggr

import (
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
)

// countSeriesAggrState calculates output=count_series, e.g. the number of unique series.
type countSeriesAggrState struct {
	m                 sync.Map
	intervalSecs      uint64
	stalenessSecs     uint64
	lastPushTimestamp uint64
}

type countSeriesStateValue struct {
	mu             sync.Mutex
	countedSeries  map[string]struct{}
	n              uint64
	samplesCount   uint64
	deleted        bool
	deleteDeadline uint64
}

func newCountSeriesAggrState(interval time.Duration, stalenessInterval time.Duration) *countSeriesAggrState {
	return &countSeriesAggrState{
		intervalSecs:  roundDurationToSecs(interval),
		stalenessSecs: roundDurationToSecs(stalenessInterval),
	}
}

func (as *countSeriesAggrState) pushSample(inputKey, outputKey string, _ float64) {
	currentTime := fasttime.UnixTimestamp()
	deleteDeadline := currentTime + as.stalenessSecs

again:
	v, ok := as.m.Load(outputKey)
	if !ok {
		// The entry is missing in the map. Try creating it.
		v = &countSeriesStateValue{
			countedSeries: map[string]struct{}{
				inputKey: {},
			},
			n: 1,
		}
		vNew, loaded := as.m.LoadOrStore(outputKey, v)
		if !loaded {
			// The entry has been added to the map.
			return
		}
		// Update the entry created by a concurrent goroutine.
		v = vNew
	}
	sv := v.(*countSeriesStateValue)
	sv.mu.Lock()
	deleted := sv.deleted
	if !deleted {
		if _, ok := sv.countedSeries[inputKey]; !ok {
			sv.countedSeries[inputKey] = struct{}{}
			sv.n++
			sv.deleteDeadline = deleteDeadline
		}
		sv.samplesCount++
	}
	sv.mu.Unlock()
	if deleted {
		// The entry has been deleted by the concurrent call to appendSeriesForFlush
		// Try obtaining and updating the entry again.
		goto again
	}
}

func (as *countSeriesAggrState) removeOldEntries(currentTime uint64) {
	m := &as.m
	m.Range(func(k, v interface{}) bool {
		sv := v.(*countSeriesStateValue)

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

func (as *countSeriesAggrState) appendSeriesForFlush(ctx *flushCtx) {
	currentTime := fasttime.UnixTimestamp()
	currentTimeMsec := int64(currentTime) * 1000

	as.removeOldEntries(currentTime)

	m := &as.m
	m.Range(func(k, v interface{}) bool {
		sv := v.(*countSeriesStateValue)
		sv.mu.Lock()
		n := sv.n
		sv.n = 0
		// todo: use builtin function clear after switching to go 1.21
		for csk := range sv.countedSeries {
			delete(sv.countedSeries, csk)
		}
		sv.mu.Unlock()
		key := k.(string)
		ctx.appendSeries(key, as.getOutputName(), currentTimeMsec, float64(n))
		return true
	})

	as.lastPushTimestamp = currentTime
}

func (as *countSeriesAggrState) getOutputName() string {
	return "count_series"
}

func (as *countSeriesAggrState) getStateRepresentation(suffix string) []aggrStateRepresentation {
	result := make([]aggrStateRepresentation, 0)
	as.m.Range(func(k, v any) bool {
		value := v.(*countSeriesStateValue)
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
			samplesCount:      value.samplesCount,
		})
		return true
	})
	return result
}
