package streamaggr

import (
	"math"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
)

// newincreaseAggrState calculates output=newincrease, e.g. the newincrease over input counters.
type newincreaseAggrState struct {
	m sync.Map

	ignoreInputDeadline uint64
	stalenessSecs       uint64
}

type newincreaseStateValue struct {
	mu             sync.Mutex
	lastValues     map[string]*lastValueState
	total          float64
	first          float64
	deleteDeadline uint64
	deleted        bool
}

func newnewincreaseAggrState(interval time.Duration, stalenessInterval time.Duration) *newincreaseAggrState {
	currentTime := fasttime.UnixTimestamp()
	intervalSecs := roundDurationToSecs(interval)
	stalenessSecs := roundDurationToSecs(stalenessInterval)
	return &newincreaseAggrState{
		ignoreInputDeadline: currentTime + intervalSecs,
		stalenessSecs:       stalenessSecs,
	}
}

func (as *newincreaseAggrState) pushSample(inputKey, outputKey string, value float64) {
	currentTime := fasttime.UnixTimestamp()
	deleteDeadline := currentTime + as.stalenessSecs

again:
	v, ok := as.m.Load(outputKey)
	if !ok {
		// The entry is missing in the map. Try creating it.
		v = &newincreaseStateValue{
			lastValues: make(map[string]*lastValueState),
		}
		vNew, loaded := as.m.LoadOrStore(outputKey, v)
		if loaded {
			// Use the entry created by a concurrent goroutine.
			v = vNew
		}
	}
	sv := v.(*newincreaseStateValue)
	sv.mu.Lock()
	deleted := sv.deleted
	if !deleted {
		lv, ok := sv.lastValues[inputKey]
		if !ok {
			lv = &lastValueState{}
			lv.firstValue = value
			lv.value = value
			lv.correction = 0
			sv.lastValues[inputKey] = lv
		}

		// process counter reset
		delta := value - lv.value
		if delta < 0 {
			if (-delta * 8) < lv.value {
				lv.correction += lv.value - value
			} else {
				lv.correction += lv.value
			}
		}

		// process increasing counter
		correctedValue := value + lv.correction
		correctedDelta := correctedValue - lv.firstValue
		if ok && math.Abs(correctedValue) < 10*(math.Abs(correctedDelta)+1) {
			correctedDelta = correctedValue
		}
		if ok || currentTime > as.ignoreInputDeadline {
			sv.total = correctedDelta
		}
		lv.value = value
		lv.deleteDeadline = deleteDeadline
		sv.deleteDeadline = deleteDeadline
	}
	sv.mu.Unlock()
	if deleted {
		// The entry has been deleted by the concurrent call to appendSeriesForFlush
		// Try obtaining and updating the entry again.
		goto again
	}
}

func (as *newincreaseAggrState) removeOldEntries(currentTime uint64) {
	m := &as.m
	m.Range(func(k, v interface{}) bool {
		sv := v.(*newincreaseStateValue)

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

func (as *newincreaseAggrState) appendSeriesForFlush(ctx *flushCtx) {
	currentTime := fasttime.UnixTimestamp()
	currentTimeMsec := int64(currentTime) * 1000

	as.removeOldEntries(currentTime)

	m := &as.m
	m.Range(func(k, v interface{}) bool {
		sv := v.(*newincreaseStateValue)
		sv.mu.Lock()
		newincrease := sv.total
		sv.total = 0
		deleted := sv.deleted
		sv.mu.Unlock()
		if !deleted {
			key := k.(string)
			ctx.appendSeries(key, as.getOutputName(), currentTimeMsec, newincrease)
		}
		return true
	})
}

func (as *newincreaseAggrState) getOutputName() string {
	return "newincrease"
}

func (as *newincreaseAggrState) getStateRepresentation(suffix string) []aggrStateRepresentation {
	result := make([]aggrStateRepresentation, 0)
	as.m.Range(func(k, v any) bool {
		value := v.(*newincreaseStateValue)
		value.mu.Lock()
		defer value.mu.Unlock()
		if value.deleted {
			return true
		}
		result = append(result, aggrStateRepresentation{
			metric: getLabelsStringFromKey(k.(string), suffix, as.getOutputName()),
			value:  value.total,
		})
		return true
	})
	return result
}
