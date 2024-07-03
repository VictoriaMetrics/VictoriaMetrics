package streamaggr

import (
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
)

// sumSamplesAggrState calculates output=sum_samples, e.g. the sum over input samples.
type sumSamplesAggrState struct {
	m sync.Map
}

type sumSamplesStateValue struct {
	mu             sync.Mutex
	state          [aggrStateSize]sumState
	deleted        bool
	deleteDeadline int64
}

type sumState struct {
	sum    float64
	exists bool
}

func newSumSamplesAggrState() *sumSamplesAggrState {
	return &sumSamplesAggrState{}
}

func (as *sumSamplesAggrState) pushSamples(samples []pushSample, deleteDeadline int64, idx int) {
	for i := range samples {
		s := &samples[i]
		outputKey := getOutputKey(s.key)

	again:
		v, ok := as.m.Load(outputKey)
		if !ok {
			// The entry is missing in the map. Try creating it.
			v = &sumSamplesStateValue{}
			outputKey = bytesutil.InternString(outputKey)
			vNew, loaded := as.m.LoadOrStore(outputKey, v)
			if loaded {
				// Update the entry created by a concurrent goroutine.
				v = vNew
			}
		}
		sv := v.(*sumSamplesStateValue)
		sv.mu.Lock()
		deleted := sv.deleted
		if !deleted {
			sv.state[idx].sum += s.value
			sv.state[idx].exists = true
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

func (as *sumSamplesAggrState) flushState(ctx *flushCtx, flushTimestamp int64, idx int) {
	m := &as.m
	m.Range(func(k, v interface{}) bool {
		sv := v.(*sumSamplesStateValue)
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
		sv.state[idx] = sumState{}
		sv.mu.Unlock()
		if state.exists {
			key := k.(string)
			ctx.appendSeries(key, "sum_samples", flushTimestamp, state.sum)
		}
		return true
	})
}
