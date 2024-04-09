package streamaggr

import (
	"math"
	"sync"
)

// stddevAggrState calculates output=stddev, e.g. the average value over input samples.
type stddevAggrState struct {
	m sync.Map
}

type stddevStateValue struct {
	mu      sync.Mutex
	stddev  map[int64]*stddevState
	deleted bool
}

type stddevState struct {
	count float64
	avg   float64
	q     float64
}

func newStddevAggrState() *stddevAggrState {
	return &stddevAggrState{}
}

func (as *stddevAggrState) pushSamples(windows map[int64][]pushSample) {
	for ts, samples := range windows {
		for i := range samples {
			s := &samples[i]
			outputKey := getOutputKey(s.key)

		again:
			v, ok := as.m.Load(outputKey)
			if !ok {
				// The entry is missing in the map. Try creating it.
				v = &stddevStateValue{
					stddev: map[int64]*stddevState{
						ts: &stddevState{},
					},
				}
				vNew, loaded := as.m.LoadOrStore(outputKey, v)
				if loaded {
					// Use the entry created by a concurrent goroutine.
					v = vNew
				}
			}
			sv := v.(*stddevStateValue)
			sv.mu.Lock()
			deleted := sv.deleted
			if !deleted {
				// See `Rapid calculation methods` at https://en.wikipedia.org/wiki/Standard_deviation
				if _, ok := sv.stddev[ts]; !ok {
					sv.stddev[ts] = &stddevState{}
				}
				v := sv.stddev[ts]
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

func (as *stddevAggrState) flushState(ctx *flushCtx, flushTimestamp int64) {
	m := &as.m
	fn := func(states map[int64]*stddevState) map[int64]*stddevState {
		output := make(map[int64]*stddevState)
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
		sv := v.(*stddevStateValue)
		sv.mu.Lock()
		states := fn(sv.stddev)
		sv.mu.Unlock()
		for ts, state := range states {
			stddev := math.Sqrt(state.q / state.count)
			key := k.(string)
			ctx.appendSeries(key, "stddev", ts, stddev)
		}
		sv.mu.Lock()
		if len(sv.stddev) == 0 {
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
