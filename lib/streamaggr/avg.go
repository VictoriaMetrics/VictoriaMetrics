package streamaggr

import (
	"sync"
)

// avgAggrState calculates output=avg, e.g. the average value over input samples.
type avgAggrState struct {
	m sync.Map
}

type avgStateValue struct {
	mu      sync.Mutex
	avg     map[int64]*avgState
	deleted bool
}

type avgState struct {
	sum   float64
	count int64
}

func newAvgAggrState() *avgAggrState {
	return &avgAggrState{}
}

func (as *avgAggrState) pushSamples(windows map[int64][]pushSample) {
	for ts, samples := range windows {
		for i := range samples {
			s := &samples[i]
			outputKey := getOutputKey(s.key)

		again:
			v, ok := as.m.Load(outputKey)
			if !ok {
				// The entry is missing in the map. Try creating it.
				v = &avgStateValue{
					avg: map[int64]*avgState{
						ts: &avgState{
							sum:   s.value,
							count: 1,
						},
					},
				}
				vNew, loaded := as.m.LoadOrStore(outputKey, v)
				if !loaded {
					// The entry has been successfully stored
					continue
				}
				// Update the entry created by a concurrent goroutine.
				v = vNew
			}
			sv := v.(*avgStateValue)
			sv.mu.Lock()
			deleted := sv.deleted
			if !deleted {
				avg := sv.avg[ts]
				avg.sum += s.value
				avg.count++
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

func (as *avgAggrState) flushState(ctx *flushCtx, flushTimestamp int64) {
	m := &as.m
	fn := func(states map[int64]*avgState) map[int64]*avgState {
		output := make(map[int64]*avgState)
		if flushTimestamp == -1 {
			for ts, state := range states {
				output[ts] = state
				delete(states, ts)
			}
		} else if avg, ok := states[flushTimestamp]; ok {
			output[flushTimestamp] = avg
			delete(states, flushTimestamp)
		}
		return output
	}
	m.Range(func(k, v interface{}) bool {
		sv := v.(*avgStateValue)
		sv.mu.Lock()
		states := fn(sv.avg)
		sv.mu.Unlock()
		for ts, state := range states {
			avg := state.sum / float64(state.count)
			key := k.(string)
			ctx.appendSeries(key, "avg", ts, avg)
		}
		sv.mu.Lock()
		if len(sv.avg) == 0 {
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
