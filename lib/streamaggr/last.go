package streamaggr

import (
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
)

// lastAggrState calculates output=last, e.g. the last value over input samples.
type lastAggrState struct {
	m sync.Map
}

type lastStateValue struct {
	mu             sync.Mutex
	state          [aggrStateSize]lastState
	deleted        bool
	deleteDeadline int64
}

type lastState struct {
	last      float64
	timestamp int64
}

func newLastAggrState() *lastAggrState {
	return &lastAggrState{}
}

func (as *lastAggrState) pushSamples(samples []pushSample, deleteDeadline int64, idx int) {
	for i := range samples {
		s := &samples[i]
		outputKey := getOutputKey(s.key)

	again:
		v, ok := as.m.Load(outputKey)
		if !ok {
			// The entry is missing in the map. Try creating it.
			v = &lastStateValue{}
			outputKey = bytesutil.InternString(outputKey)
			vNew, loaded := as.m.LoadOrStore(outputKey, v)
			if loaded {
				// Update the entry created by a concurrent goroutine.
				v = vNew
			}
		}
		sv := v.(*lastStateValue)
		sv.mu.Lock()
		deleted := sv.deleted
		if !deleted {
			if s.timestamp >= sv.state[idx].timestamp {
				sv.state[idx].last = s.value
				sv.state[idx].timestamp = s.timestamp
			}
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

func (as *lastAggrState) flushState(ctx *flushCtx, flushTimestamp int64, idx int) {
	m := &as.m
	m.Range(func(k, v interface{}) bool {
		sv := v.(*lastStateValue)
		sv.mu.Lock()

		// check for stale entries
		deleted := flushTimestamp > sv.deleteDeadline
		if deleted {
			// Mark the current entry as deleted
			sv.deleted = deleted
			sv.mu.Unlock()
			m.Delete(k)
			return true
		}
		state := sv.state[idx]
		sv.state[idx] = lastState{}
		sv.mu.Unlock()
		if state.timestamp > 0 {
			key := k.(string)
			ctx.appendSeries(key, "last", flushTimestamp, state.last)
		}
		return true
	})
}
