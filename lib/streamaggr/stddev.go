package streamaggr

import (
	"math"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
)

// stddevAggrState calculates output=stddev, e.g. the average value over input samples.
type stddevAggrState struct {
	m sync.Map
}

type stddevStateValue struct {
	mu             sync.Mutex
	state          [aggrStateSize]stddevState
	deleted        bool
	deleteDeadline int64
}

type stddevState struct {
	count float64
	avg   float64
	q     float64
}

func newStddevAggrState() *stddevAggrState {
	return &stddevAggrState{}
}

func (as *stddevAggrState) pushSamples(samples []pushSample, deleteDeadline int64, idx int) {
	for i := range samples {
		s := &samples[i]
		outputKey := getOutputKey(s.key)

	again:
		v, ok := as.m.Load(outputKey)
		if !ok {
			// The entry is missing in the map. Try creating it.
			v = &stddevStateValue{}
			outputKey = bytesutil.InternString(outputKey)
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
			state := &sv.state[idx]
			state.count++
			avg := state.avg + (s.value-state.avg)/state.count
			state.q += (s.value - state.avg) * (s.value - avg)
			state.avg = avg
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

func (as *stddevAggrState) flushState(ctx *flushCtx, flushTimestamp int64, idx int) {
	m := &as.m
	m.Range(func(k, v interface{}) bool {
		sv := v.(*stddevStateValue)
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
		sv.state[idx] = stddevState{}
		sv.mu.Unlock()
		if state.count > 0 {
			key := k.(string)
			ctx.appendSeries(key, "stddev", flushTimestamp, math.Sqrt(state.q/state.count))
		}
		return true
	})
}
