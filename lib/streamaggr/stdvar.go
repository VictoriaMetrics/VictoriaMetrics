package streamaggr

import (
	"sync"
)

// stdvarAggrState calculates output=stdvar, e.g. the average value over input samples.
type stdvarAggrState struct {
	m sync.Map
}

type stdvarStateValue struct {
	mu      sync.Mutex
	stdvar  map[int64]*stdvarState
	deleted bool
}

type stdvarState struct {
	count float64
	avg   float64
	q     float64
}

func newStdvarAggrState() *stdvarAggrState {
	return &stdvarAggrState{}
}

func (as *stdvarAggrState) pushSamples(windows map[int64][]pushSample) {
	for ts, samples := range windows {
		for i := range samples {
			s := &samples[i]
			outputKey := getOutputKey(s.key)

		again:
			v, ok := as.m.Load(outputKey)
			if !ok {
				// The entry is missing in the map. Try creating it.
				v = &stdvarStateValue{
					stdvar: map[int64]*stdvarState{
						ts: &stdvarState{},
					},
				}
				vNew, loaded := as.m.LoadOrStore(outputKey, v)
				if loaded {
					// Use the entry created by a concurrent goroutine.
					v = vNew
				}
			}
			sv := v.(*stdvarStateValue)
			sv.mu.Lock()
			deleted := sv.deleted
			if !deleted {
				// See `Rapid calculation methods` at https://en.wikipedia.org/wiki/Standard_deviation
				if _, ok := sv.stdvar[ts]; !ok {
					sv.stdvar[ts] = &stdvarState{}
				}
				v := sv.stdvar[ts]
				v.count++
				avg := v.avg + (s.value-v.avg)/v.count
				v.q += (s.value - v.avg) * (s.value - avg)
				v.avg = avg
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

func (as *stdvarAggrState) flushState(ctx *flushCtx, flushTimestamp int64) {
	m := &as.m
	fn := func(states map[int64]*stdvarState) map[int64]*stdvarState {
		output := make(map[int64]*stdvarState)
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
		sv := v.(*stdvarStateValue)
		sv.mu.Lock()
		states := fn(sv.stdvar)
		sv.mu.Unlock()
		for ts, state := range states {
			stdvar := state.q / state.count
			sv.mu.Lock()
			delete(sv.stdvar, ts)
			sv.mu.Unlock()
			key := k.(string)
			ctx.appendSeries(key, "stdvar", ts, stdvar)
		}
		sv.mu.Lock()
		if len(sv.stdvar) == 0 {
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
