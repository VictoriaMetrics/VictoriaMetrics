package streamaggr

import (
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
)

// lastAggrState calculates output=last, e.g. the last value over input samples.
type lastAggrState struct {
	m sync.Map
}

type lastStateValue struct {
	mu             sync.Mutex
	last           float64
	timestamp      int64
	deleted        bool
	defined        bool
	deleteDeadline int64
}

func newLastAggrState() *lastAggrState {
	return &lastAggrState{}
}

func (as *lastAggrState) pushSamples(samples []pushSample, deleteDeadline int64, includeInputKey bool) {
	for i := range samples {
		s := &samples[i]
		outputKey := getOutputKey(s.key, includeInputKey)

	again:
		v, ok := as.m.Load(outputKey)
		if !ok {
			// The entry is missing in the map. Try creating it.
			v = &lastStateValue{}
			outputKey = bytesutil.InternString(outputKey)
			vNew, loaded := as.m.LoadOrStore(outputKey, v)
			if loaded {
				// Use the entry created by a concurrent goroutine.
				v = vNew
			}
		}
		sv := v.(*lastStateValue)
		sv.mu.Lock()
		deleted := sv.deleted
		if !deleted {
			if !sv.defined || s.timestamp >= sv.timestamp {
				sv.last = s.value
				sv.timestamp = s.timestamp
				sv.deleteDeadline = deleteDeadline
			}
			sv.defined = true
		}
		sv.mu.Unlock()
		if deleted {
			// The entry has been deleted by the concurrent call to flushState
			// Try obtaining and updating the entry again.
			goto again
		}
	}
}

func (as *lastAggrState) flushState(ctx *flushCtx) {
	m := &as.m
	m.Range(func(k, v any) bool {
		// Atomically delete the entry from the map, so new entry is created for the next flush.
		m.Delete(k)

		sv := v.(*lastStateValue)
		sv.mu.Lock()
		if ctx.flushTimestamp > sv.deleteDeadline {
			sv.deleted = true
			sv.mu.Unlock()
			key := k.(string)
			ctx.a.lc.Delete(bytesutil.ToUnsafeBytes(key), ctx.flushTimestamp)
			m.Delete(k)
			return true
		}
		last := sv.last
		// Mark the entry as deleted, so it won't be updated anymore by concurrent pushSample() calls.
		sv.deleted = true
		sv.mu.Unlock()

		key := k.(string)
		ctx.appendSeries(key, "last", last)
		return true
	})
}
