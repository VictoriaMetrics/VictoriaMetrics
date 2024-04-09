package streamaggr

import (
	"sync"
)

// sumSamplesAggrState calculates output=sum_samples, e.g. the sum over input samples.
type sumSamplesAggrState struct {
	m sync.Map
}

type sumSamplesStateValue struct {
	mu      sync.Mutex
	sum     map[int64]float64
	deleted bool
}

func newSumSamplesAggrState() *sumSamplesAggrState {
	return &sumSamplesAggrState{}
}

func (as *sumSamplesAggrState) pushSamples(windows map[int64][]pushSample) {
	for ts, samples := range windows {
		for i := range samples {
			s := &samples[i]
			outputKey := getOutputKey(s.key)

		again:
			v, ok := as.m.Load(outputKey)
			if !ok {
				// The entry is missing in the map. Try creating it.
				v = &sumSamplesStateValue{
					sum: map[int64]float64{
						ts: s.value,
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
			sv := v.(*sumSamplesStateValue)
			sv.mu.Lock()
			deleted := sv.deleted
			if !deleted {
				sv.sum[ts] += s.value
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

func (as *sumSamplesAggrState) flushState(ctx *flushCtx, flushTimestamp int64) {
	m := &as.m
	fn := func(states map[int64]float64) map[int64]float64 {
		output := make(map[int64]float64)
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
		sv := v.(*sumSamplesStateValue)
		sv.mu.Lock()
		states := fn(sv.sum)
		sv.mu.Unlock()
		for ts, state := range states {
			key := k.(string)
			ctx.appendSeries(key, "sum_samples", ts, state)
		}
		sv.mu.Lock()
		if len(sv.sum) == 0 {
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
