package streamaggr

import (
	"sync"
)

// lastAggrState calculates output=last, e.g. the last value over input samples.
type lastAggrState struct {
	m sync.Map
}

type lastStateValue struct {
	mu      sync.Mutex
	last    map[int64]*lastState
	deleted bool
}

type lastState struct {
	value     float64
	timestamp int64
}

func newLastAggrState() *lastAggrState {
	return &lastAggrState{}
}

func (as *lastAggrState) pushSamples(windows map[int64][]pushSample) {
	for ts, samples := range windows {
		for i := range samples {
			s := &samples[i]
			outputKey := getOutputKey(s.key)

		again:
			v, ok := as.m.Load(outputKey)
			if !ok {
				// The entry is missing in the map. Try creating it.
				v = &lastStateValue{
					last: map[int64]*lastState{
						ts: &lastState{
							value:     s.value,
							timestamp: s.timestamp,
						},
					},
				}
				vNew, loaded := as.m.LoadOrStore(outputKey, v)
				if !loaded {
					// The new entry has been successfully created.
					continue
				}
				// Use the entry created by a concurrent goroutine.
				v = vNew
			}
			sv := v.(*lastStateValue)
			sv.mu.Lock()
			deleted := sv.deleted
			if !deleted {
				if last, ok := sv.last[ts]; !ok {
					sv.last[ts] = &lastState{
						value:     s.value,
						timestamp: s.timestamp,
					}
				} else if s.timestamp >= last.timestamp {
					last.value = s.value
					last.timestamp = s.timestamp
				}
			}
			sv.mu.Unlock()
			if deleted {
				// The entry has been deleted by the concurrent call to flushState
				// Try obtaining and updating the entry again.
				goto again
			}
		}
	}
}

func (as *lastAggrState) flushState(ctx *flushCtx, flushTimestamp int64) {
	m := &as.m
	fn := func(states map[int64]*lastState) map[int64]*lastState {
		output := make(map[int64]*lastState)
		if flushTimestamp == -1 {
			for ts, state := range states {
				output[ts] = state
				delete(states, ts)
			}
		} else if state, ok := states[flushTimestamp]; ok {
			output[flushTimestamp] = state
			delete(states, flushTimestamp)
		}
		return output
	}
	m.Range(func(k, v interface{}) bool {
		sv := v.(*lastStateValue)
		sv.mu.Lock()
		states := fn(sv.last)
		sv.mu.Unlock()
		for ts, state := range states {
			key := k.(string)
			ctx.appendSeries(key, "last", ts, state.value)
		}
		sv.mu.Lock()
		if len(sv.last) == 0 {
			// Mark the entry as deleted, so it won't be updated anymore by concurrent pushSample() calls.
			sv.deleted = true
			sv.mu.Unlock()
			// Atomically delete the entry from the map, so new entry is created for the next flush.
			m.Delete(k)
		} else {
			sv.mu.Unlock()
		}
		return true
	})
}
