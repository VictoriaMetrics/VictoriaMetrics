package streamaggr

import (
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
)

// countSamplesAggrState calculates output=count_samples, e.g. the count of input samples.
type countSamplesAggrState struct {
	m sync.Map
}

type countSamplesStateValue struct {
	mu             sync.Mutex
	state          [aggrStateSize]uint64
	deleted        bool
	deleteDeadline int64
}

func newCountSamplesAggrState() *countSamplesAggrState {
	return &countSamplesAggrState{}
}

func (as *countSamplesAggrState) pushSamples(samples []pushSample, deleteDeadline int64, idx int) {
	for i := range samples {
		s := &samples[i]
		outputKey := getOutputKey(s.key)

	again:
		v, ok := as.m.Load(outputKey)
		if !ok {
			// The entry is missing in the map. Try creating it.
			v = &countSamplesStateValue{}
			outputKey = bytesutil.InternString(outputKey)
			vNew, loaded := as.m.LoadOrStore(outputKey, v)
			if loaded {
				// Use the entry created by a concurrent goroutine.
				v = vNew
			}
		}
		sv := v.(*countSamplesStateValue)
		sv.mu.Lock()
		deleted := sv.deleted
		if !deleted {
			sv.state[idx]++
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

func (as *countSamplesAggrState) flushState(ctx *flushCtx, flushTimestamp int64, idx int) {
	m := &as.m
	m.Range(func(k, v interface{}) bool {
		sv := v.(*countSamplesStateValue)
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
		sv.state[idx] = 0
		sv.mu.Unlock()
		if state > 0 {
			key := k.(string)
			ctx.appendSeries(key, "count_samples", flushTimestamp, float64(state))
		}
		return true
	})
}
