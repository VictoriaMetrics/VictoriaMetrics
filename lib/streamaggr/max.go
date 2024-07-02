package streamaggr

import (
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
)

// maxAggrState calculates output=max, e.g. the maximum value over input samples.
type maxAggrState struct {
	m sync.Map
}

type maxStateValue struct {
	mu             sync.Mutex
	state          [aggrStateSize]maxState
	deleted        bool
	deleteDeadline int64
}

type maxState struct {
	max    float64
	exists bool
}

func newMaxAggrState() *maxAggrState {
	return &maxAggrState{}
}

func (as *maxAggrState) pushSamples(samples []pushSample, deleteDeadline int64, idx int) {
	for i := range samples {
		s := &samples[i]
		outputKey := getOutputKey(s.key)

	again:
		v, ok := as.m.Load(outputKey)
		if !ok {
			// The entry is missing in the map. Try creating it.
			v = &maxStateValue{}
			outputKey = bytesutil.InternString(outputKey)
			vNew, loaded := as.m.LoadOrStore(outputKey, v)
			if loaded {
				// Use the entry created by a concurrent goroutine.
				v = vNew
			}
		}
		sv := v.(*maxStateValue)
		sv.mu.Lock()
		deleted := sv.deleted
		if !deleted {
			state := &sv.state[idx]
			if !state.exists {
				state.max = s.value
				state.exists = true
			} else if s.value > state.max {
				state.max = s.value
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

func (as *maxAggrState) flushState(ctx *flushCtx, flushTimestamp int64, idx int) {
	m := &as.m
	m.Range(func(k, v interface{}) bool {
		sv := v.(*maxStateValue)
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
		sv.state[idx] = maxState{}
		sv.mu.Unlock()
		if state.exists {
			key := k.(string)
			ctx.appendSeries(key, "max", flushTimestamp, state.max)
		}
		return true
	})
}
