package streamaggr

import (
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
)

// maxAggrState calculates output=max, e.g. the maximum value over input samples.
type maxAggrState struct {
	m sync.Map
}

type maxStateValue struct {
	mu      sync.Mutex
	max     float64
	deleted bool
}

func newMaxAggrState() *maxAggrState {
	return &maxAggrState{}
}

func (as *maxAggrState) pushSamples(samples []pushSample) {
	for i := range samples {
		s := &samples[i]
		outputKey := getOutputKey(s.key)

	again:
		v, ok := as.m.Load(outputKey)
		if !ok {
			// The entry is missing in the map. Try creating it.
			v = &maxStateValue{
				max: s.value,
			}
			outputKey = bytesutil.InternString(outputKey)
			vNew, loaded := as.m.LoadOrStore(outputKey, v)
			if !loaded {
				// The new entry has been successfully created.
				continue
			}
			// Use the entry created by a concurrent goroutine.
			v = vNew
		}
		sv := v.(*maxStateValue)
		sv.mu.Lock()
		deleted := sv.deleted
		if !deleted {
			if s.value > sv.max {
				sv.max = s.value
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

func (as *maxAggrState) flushState(ctx *flushCtx) {
	m := &as.m
	m.Range(func(k, v any) bool {
		// Atomically delete the entry from the map, so new entry is created for the next flush.
		m.Delete(k)

		sv := v.(*maxStateValue)
		sv.mu.Lock()
		maxV := sv.max
		// Mark the entry as deleted, so it won't be updated anymore by concurrent pushSample() calls.
		sv.deleted = true
		sv.mu.Unlock()

		key := k.(string)
		ctx.appendSeries(key, "max", maxV)
		return true
	})
}
