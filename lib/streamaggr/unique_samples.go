package streamaggr

import (
	"sync"
)

// uniqueSamplesAggrState calculates output=unique_samples, e.g. the number of unique sample values.
type uniqueSamplesAggrState struct {
	m sync.Map
}

type uniqueSamplesStateValue struct {
	mu      sync.Mutex
	m       map[int64]map[float64]struct{}
	deleted bool
}

func newUniqueSamplesAggrState() *uniqueSamplesAggrState {
	return &uniqueSamplesAggrState{}
}

func (as *uniqueSamplesAggrState) pushSamples(windows map[int64][]pushSample) {
	for ts, samples := range windows {
		for i := range samples {
			s := &samples[i]
			outputKey := getOutputKey(s.key)

		again:
			v, ok := as.m.Load(outputKey)
			if !ok {
				// The entry is missing in the map. Try creating it.
				v = &uniqueSamplesStateValue{
					m: map[int64]map[float64]struct{}{
						ts: map[float64]struct{}{
							s.value: {},
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
			sv := v.(*uniqueSamplesStateValue)
			sv.mu.Lock()
			deleted := sv.deleted
			if !deleted {
				if _, ok := sv.m[ts]; !ok {
					sv.m[ts] = map[float64]struct{}{
						s.value: {},
					}
					sv.mu.Unlock()
					continue
				}
				if _, ok = sv.m[ts][s.value]; !ok {
					sv.m[ts][s.value] = struct{}{}
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

func (as *uniqueSamplesAggrState) flushState(ctx *flushCtx, flushTimestamp int64) {
	m := &as.m
	fn := func(states map[int64]map[float64]struct{}) map[int64]map[float64]struct{} {
		output := make(map[int64]map[float64]struct{})
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
		sv := v.(*uniqueSamplesStateValue)
		sv.mu.Lock()
		states := fn(sv.m)
		sv.mu.Unlock()
		for ts, state := range states {
			key := k.(string)
			ctx.appendSeries(key, "unique_samples", ts, float64(len(state)))
		}
		sv.mu.Lock()
		if len(sv.m) == 0 {
			// Mark the entry as deleted, so it won't be updated anymore by concurrent pushSample() calls.
			sv.mu.Lock()
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
