package streamaggr

import (
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
)

// minAggrState calculates output=min, e.g. the minimum value over input samples.
type minAggrState struct {
	m sync.Map
}

type minStateValue struct {
	mu             sync.Mutex
	min            float64
	deleted        bool
	defined        bool
	deleteDeadline int64
}

func newMinAggrState() *minAggrState {
	return &minAggrState{}
}

func (as *minAggrState) pushSamples(samples []pushSample, deleteDeadline int64, includeInputKey bool) {
	for i := range samples {
		s := &samples[i]
		outputKey := getOutputKey(s.key, includeInputKey)

	again:
		v, ok := as.m.Load(outputKey)
		if !ok {
			// The entry is missing in the map. Try creating it.
			v = &minStateValue{}
			outputKey = bytesutil.InternString(outputKey)
			vNew, loaded := as.m.LoadOrStore(outputKey, v)
			if loaded {
				// Use the entry created by a concurrent goroutine.
				v = vNew
			}
		}
		sv := v.(*minStateValue)
		sv.mu.Lock()
		deleted := sv.deleted
		if !deleted {
			if !sv.defined || s.value < sv.min {
				sv.min = s.value
			}
			sv.deleteDeadline = deleteDeadline
			if !sv.defined {
				sv.defined = true
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

func (as *minAggrState) flushState(ctx *flushCtx) {
	m := &as.m
	m.Range(func(k, v any) bool {
		// Atomically delete the entry from the map, so new entry is created for the next flush.
		m.Delete(k)

		sv := v.(*minStateValue)
		sv.mu.Lock()
		if ctx.flushTimestamp > sv.deleteDeadline {
			sv.deleted = true
			sv.mu.Unlock()
			key := k.(string)
			ctx.a.lc.Delete(bytesutil.ToUnsafeBytes(key), ctx.flushTimestamp)
			m.Delete(k)
			return true
		}
		if !sv.defined {
			sv.mu.Unlock()
			return true
		}
		min := sv.min
		sv.min = 0
		sv.defined = false
		sv.mu.Unlock()

		key := k.(string)
		ctx.appendSeries(key, "min", min)
		return true
	})
}
