package streamaggr

import (
	"math"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
)

// newtotalAggrState calculates output=newtotal, e.g. the summary counter over input counters.
type newtotalAggrState struct {
	m sync.Map

	ignoreInputDeadline uint64
	stalenessSecs       uint64
}

type newtotalStateValue struct {
	mu             sync.Mutex
	lastValues     map[string]*lastValueState
	total          float64
	deleteDeadline uint64
	deleted        bool
}

func newnewtotalAggrState(interval time.Duration, stalenessInterval time.Duration) *newtotalAggrState {
	currentTime := fasttime.UnixTimestamp()
	intervalSecs := roundDurationToSecs(interval)
	stalenessSecs := roundDurationToSecs(stalenessInterval)
	return &newtotalAggrState{
		ignoreInputDeadline: currentTime + intervalSecs,
		stalenessSecs:       stalenessSecs,
	}
}

func (as *newtotalAggrState) pushSample(inputKey, outputKey string, value float64) {
	currentTime := fasttime.UnixTimestamp()
	deleteDeadline := currentTime + as.stalenessSecs

again:
	v, ok := as.m.Load(outputKey)
	if !ok {
		// The entry is missing in the map. Try creating it.
		v = &newtotalStateValue{
			lastValues: make(map[string]*lastValueState),
		}
		vNew, loaded := as.m.LoadOrStore(outputKey, v)
		if loaded {
			// Use the entry created by a concurrent goroutine.
			v = vNew
		}
	}
	sv := v.(*newtotalStateValue)
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

func (as *newtotalAggrState) removeOldEntries(currentTime uint64) {
	m := &as.m
	m.Range(func(k, v interface{}) bool {
		sv := v.(*newtotalStateValue)

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

func (as *newtotalAggrState) appendSeriesForFlush(ctx *flushCtx) {
	currentTime := fasttime.UnixTimestamp()
	currentTimeMsec := int64(currentTime) * 1000

	as.removeOldEntries(currentTime)

	m := &as.m
	m.Range(func(k, v interface{}) bool {
		sv := v.(*newtotalStateValue)
		sv.mu.Lock()
		total := sv.total
		if math.Abs(sv.total) >= (1 << 53) {
			// It is time to reset the entry, since it starts losing float64 precision
			sv.total = 0
		}
		deleted := sv.deleted
		sv.mu.Unlock()
		if !deleted {
			key := k.(string)
			ctx.appendSeries(key, as.getOutputName(), currentTimeMsec, total)
		}
		return true
	})
}

func (as *newtotalAggrState) getOutputName() string {
	return "newtotal"
}

func (as *newtotalAggrState) getStateRepresentation(suffix string) []aggrStateRepresentation {
	result := make([]aggrStateRepresentation, 0)
	as.m.Range(func(k, v any) bool {
		value := v.(*newtotalStateValue)
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
