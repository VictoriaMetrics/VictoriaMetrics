package streamaggr

import (
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
)

// avgAggrState calculates output=avg, e.g. the average value over input samples.
type avgAggrState struct {
	m sync.Map
}

type avgState struct {
	sum   float64
	count float64
}

type avgStateValue struct {
	mu             sync.Mutex
	state          [aggrStateSize]avgState
	deleted        bool
	deleteDeadline int64
}

func newAvgAggrState() *avgAggrState {
	return &avgAggrState{}
}

func (as *avgAggrState) pushSamples(samples []pushSample, deleteDeadline int64, idx int) {
	for i := range samples {
		s := &samples[i]
		outputKey := getOutputKey(s.key)

	again:
		v, ok := as.m.Load(outputKey)
		if !ok {
			// The entry is missing in the map. Try creating it.
			v = &avgStateValue{}
			outputKey = bytesutil.InternString(outputKey)
			vNew, loaded := as.m.LoadOrStore(outputKey, v)
			if loaded {
				// Use the entry created by a concurrent goroutine.
				v = vNew
			}
		}
		sv := v.(*avgStateValue)
		sv.mu.Lock()
		deleted := sv.deleted
		if !deleted {
			sv.state[idx].sum += s.value
			sv.state[idx].count++
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

func (as *avgAggrState) flushState(ctx *flushCtx) {
	m := &as.m
	m.Range(func(k, v any) bool {
		// Atomically delete the entry from the map, so new entry is created for the next flush.
		sv := v.(*avgStateValue)
		sv.mu.Lock()

		// check for stale entries
		deleted := ctx.flushTimestamp > sv.deleteDeadline
		if deleted {
			// Mark the current entry as deleted
			sv.deleted = deleted
			sv.mu.Unlock()
			m.Delete(k)
			return true
		}
		state := sv.state[ctx.idx]
		sv.state[ctx.idx] = avgState{}
		sv.mu.Unlock()
		if state.count > 0 {
			key := k.(string)
			avg := state.sum / state.count
			ctx.appendSeries(key, "avg", avg)
		}
		return true
	})
}
