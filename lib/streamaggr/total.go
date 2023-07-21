package streamaggr

import (
	"math"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
)

// totalAggrState calculates output=total, e.g. the summary counter over input counters.
type totalAggrState struct {
	m sync.Map

	ignoreInputDeadline uint64
	stalenessSecs       uint64
}

type totalStateValue struct {
	mu             sync.Mutex
	lastValues     map[string]*lastValueState
	total          float64
	deleteDeadline uint64
	deleted        bool
}

type lastValueState struct {
	value          float64
	deleteDeadline uint64
}

func newTotalAggrState(interval time.Duration, stalenessInterval time.Duration) *totalAggrState {
	currentTime := fasttime.UnixTimestamp()
	intervalSecs := roundDurationToSecs(interval)
	stalenessSecs := roundDurationToSecs(stalenessInterval)
	return &totalAggrState{
		ignoreInputDeadline: currentTime + intervalSecs,
		stalenessSecs:       stalenessSecs,
	}
}

func (as *totalAggrState) pushSample(inputKey, outputKey string, value float64) {
	currentTime := fasttime.UnixTimestamp()
	deleteDeadline := currentTime + as.stalenessSecs

again:
	v, ok := as.m.Load(outputKey)
	if !ok {
		// The entry is missing in the map. Try creating it.
		v = &totalStateValue{
			lastValues: make(map[string]*lastValueState),
		}
		vNew, loaded := as.m.LoadOrStore(outputKey, v)
		if loaded {
			// Use the entry created by a concurrent goroutine.
			v = vNew
		}
	}
	sv := v.(*totalStateValue)
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
		if ok || currentTime > as.ignoreInputDeadline {
			sv.total += d
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

func (as *totalAggrState) removeOldEntries(currentTime uint64) {
	m := &as.m
	m.Range(func(k, v interface{}) bool {
		sv := v.(*totalStateValue)

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

func (as *totalAggrState) appendSeriesForFlush(ctx *flushCtx) {
	currentTime := fasttime.UnixTimestamp()
	currentTimeMsec := int64(currentTime) * 1000

	as.removeOldEntries(currentTime)

	m := &as.m
	m.Range(func(k, v interface{}) bool {
		sv := v.(*totalStateValue)
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
			ctx.appendSeries(key, "total", currentTimeMsec, total)
		}
		return true
	})
}
