package streamaggr

import (
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
)

// uniqueSamplesAggrState calculates output=unique_samples, e.g. the number of unique sample values.
type uniqueSamplesAggrState struct {
	m sync.Map
}

type uniqueSamplesStateValue struct {
	mu             sync.Mutex
	state          [aggrStateSize]map[float64]struct{}
	deleted        bool
	deleteDeadline int64
}

func newUniqueSamplesAggrState() *uniqueSamplesAggrState {
	return &uniqueSamplesAggrState{}
}

func (as *uniqueSamplesAggrState) pushSamples(samples []pushSample, deleteDeadline int64, idx int) {
	for i := range samples {
		s := &samples[i]
		outputKey := getOutputKey(s.key)

	again:
		v, ok := as.m.Load(outputKey)
		if !ok {
			// The entry is missing in the map. Try creating it.
			usv := &uniqueSamplesStateValue{}
			for iu := range usv.state {
				usv.state[iu] = make(map[float64]struct{})
			}
			v = usv
			outputKey = bytesutil.InternString(outputKey)
			vNew, loaded := as.m.LoadOrStore(outputKey, v)
			if loaded {
				// Update the entry created by a concurrent goroutine.
				v = vNew
			}
		}
		sv := v.(*uniqueSamplesStateValue)
		sv.mu.Lock()
		deleted := sv.deleted
		if !deleted {
			if _, ok := sv.state[idx][s.value]; !ok {
				sv.state[idx][s.value] = struct{}{}
			}
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

func (as *uniqueSamplesAggrState) flushState(ctx *flushCtx, flushTimestamp int64, idx int) {
	m := &as.m
	m.Range(func(k, v interface{}) bool {
		sv := v.(*uniqueSamplesStateValue)
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
		state := len(sv.state[idx])
		sv.state[idx] = make(map[float64]struct{})
		sv.mu.Unlock()
		if state > 0 {
			key := k.(string)
			ctx.appendSeries(key, "unique_series", flushTimestamp, float64(state))
		}
		return true
	})
}
