package streamaggr

import (
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
)

// increasePureAggrState calculates output=increase_pure, e.g. the increasePure over input counters.
type increasePureAggrState struct {
	m                 sync.Map
	intervalSecs      uint64
	stalenessSecs     uint64
	lastPushTimestamp uint64
}

type increasePureStateValue struct {
	mu             sync.Mutex
	lastValues     map[string]*lastValueState
	total          float64
	samplesCount   uint64
	deleteDeadline uint64
	deleted        bool
}

func newIncreasePureAggrState(interval time.Duration, stalenessInterval time.Duration) *increasePureAggrState {
	return &increasePureAggrState{
		intervalSecs:  roundDurationToSecs(interval),
		stalenessSecs: roundDurationToSecs(stalenessInterval),
	}
}

func (as *increasePureAggrState) pushSample(inputKey, outputKey string, value float64) {
	currentTime := fasttime.UnixTimestamp()
	deleteDeadline := currentTime + as.stalenessSecs

again:
	v, ok := as.m.Load(outputKey)
	if !ok {
		// The entry is missing in the map. Try creating it.
		v = &increasePureStateValue{
			lastValues: make(map[string]*lastValueState),
		}
		vNew, loaded := as.m.LoadOrStore(outputKey, v)
		if loaded {
			// Use the entry created by a concurrent goroutine.
			v = vNew
		}
	}
	sv := v.(*increasePureStateValue)
	sv.mu.Lock()
	deleted := sv.deleted
	if !deleted {
		lv, ok := sv.lastValues[inputKey]
		if !ok {
			lv = &lastValueState{}
			sv.lastValues[inputKey] = lv
		}
		d := value
		if ok && lv.value <= value {
			d = value - lv.value
		}
		sv.total += d
		lv.value = value
		lv.deleteDeadline = deleteDeadline
		sv.deleteDeadline = deleteDeadline
		sv.samplesCount++
	}
	sv.mu.Unlock()
	if deleted {
		// The entry has been deleted by the concurrent call to appendSeriesForFlush
		// Try obtaining and updating the entry again.
		goto again
	}
}

func (as *increasePureAggrState) removeOldEntries(currentTime uint64) {
	m := &as.m
	m.Range(func(k, v interface{}) bool {
		sv := v.(*increasePureStateValue)

		sv.mu.Lock()
		deleted := currentTime > sv.deleteDeadline
		if deleted {
			// Mark the current entry as deleted
			sv.deleted = deleted
		} else {
			// Delete outdated entries in sv.lastValues
			m := sv.lastValues
			for k1, v1 := range m {
				if currentTime > v1.deleteDeadline {
					delete(m, k1)
				}
			}
		}
		sv.mu.Unlock()

		if deleted {
			m.Delete(k)
		}
		return true
	})
}

func (as *increasePureAggrState) appendSeriesForFlush(ctx *flushCtx) {
	currentTime := fasttime.UnixTimestamp()
	currentTimeMsec := int64(currentTime) * 1000

	as.removeOldEntries(currentTime)

	m := &as.m
	m.Range(func(k, v interface{}) bool {
		sv := v.(*increasePureStateValue)
		sv.mu.Lock()
		increasePure := sv.total
		sv.total = 0
		deleted := sv.deleted
		sv.mu.Unlock()
		if !deleted {
			key := k.(string)
			ctx.appendSeries(key, as.getOutputName(), currentTimeMsec, increasePure)
		}
		return true
	})

	as.lastPushTimestamp = currentTime
}

func (as *increasePureAggrState) getOutputName() string {
	return "increase_pure"
}

func (as *increasePureAggrState) getStateRepresentation(suffix string) []aggrStateRepresentation {
	result := make([]aggrStateRepresentation, 0)
	as.m.Range(func(k, v any) bool {
		value := v.(*increasePureStateValue)
		value.mu.Lock()
		defer value.mu.Unlock()
		if value.deleted {
			return true
		}
		result = append(result, aggrStateRepresentation{
			metric:            getLabelsStringFromKey(k.(string), suffix, as.getOutputName()),
			currentValue:      value.total,
			lastPushTimestamp: as.lastPushTimestamp,
			nextPushTimestamp: as.lastPushTimestamp + as.intervalSecs,
			samplesCount:      value.samplesCount,
		})
		return true
	})
	return result
}
