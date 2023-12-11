package streamaggr

import (
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
)

// totalPureAggrState calculates output=total_pure, e.g. the summary counter over input counters.
type totalPureAggrState struct {
	m                 sync.Map
	intervalSecs      uint64
	stalenessSecs     uint64
	lastPushTimestamp atomic.Uint64
}

type totalPureStateValue struct {
	mu             sync.Mutex
	lastValues     map[string]*lastValueState
	total          float64
	samplesCount   uint64
	deleteDeadline uint64
	deleted        bool
}

func newTotalPureAggrState(interval time.Duration, stalenessInterval time.Duration) *totalPureAggrState {
	return &totalPureAggrState{
		intervalSecs:  roundDurationToSecs(interval),
		stalenessSecs: roundDurationToSecs(stalenessInterval),
	}
}

func (as *totalPureAggrState) pushSample(inputKey, outputKey string, value float64) {
	currentTime := fasttime.UnixTimestamp()
	deleteDeadline := currentTime + as.stalenessSecs

again:
	v, ok := as.m.Load(outputKey)
	if !ok {
		// The entry is missing in the map. Try creating it.
		v = &totalPureStateValue{
			lastValues: make(map[string]*lastValueState),
		}
		vNew, loaded := as.m.LoadOrStore(outputKey, v)
		if loaded {
			// Use the entry created by a concurrent goroutine.
			v = vNew
		}
	}
	sv := v.(*totalPureStateValue)
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

func (as *totalPureAggrState) removeOldEntries(currentTime uint64) {
	m := &as.m
	m.Range(func(k, v interface{}) bool {
		sv := v.(*totalPureStateValue)

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

func (as *totalPureAggrState) appendSeriesForFlush(ctx *flushCtx) {
	currentTime := fasttime.UnixTimestamp()
	currentTimeMsec := int64(currentTime) * 1000

	as.removeOldEntries(currentTime)

	m := &as.m
	m.Range(func(k, v interface{}) bool {
		sv := v.(*totalPureStateValue)
		sv.mu.Lock()
		totalPure := sv.total
		if math.Abs(sv.total) >= (1 << 53) {
			// It is time to reset the entry, since it starts losing float64 precision
			sv.total = 0
		}
		deleted := sv.deleted
		sv.mu.Unlock()
		if !deleted {
			key := k.(string)
			ctx.appendSeries(key, as.getOutputName(), currentTimeMsec, totalPure)
		}
		return true
	})
	as.lastPushTimestamp.Store(currentTime)
}

func (as *totalPureAggrState) getOutputName() string {
	return "total_pure"
}

func (as *totalPureAggrState) getStateRepresentation(suffix string) aggrStateRepresentation {
	metrics := make([]aggrStateRepresentationMetric, 0)
	as.m.Range(func(k, v any) bool {
		value := v.(*totalPureStateValue)
		value.mu.Lock()
		defer value.mu.Unlock()
		if value.deleted {
			return true
		}
		metrics = append(metrics, aggrStateRepresentationMetric{
			metric:       getLabelsStringFromKey(k.(string), suffix, as.getOutputName()),
			currentValue: value.total,
			samplesCount: value.samplesCount,
		})
		return true
	})
	return aggrStateRepresentation{
		intervalSecs:      as.intervalSecs,
		lastPushTimestamp: as.lastPushTimestamp.Load(),
		metrics:           metrics,
	}
}
