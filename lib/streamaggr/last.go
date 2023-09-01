package streamaggr

import (
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
)

// lastAggrState calculates output=last, e.g. the last value over input samples.
type lastAggrState struct {
	m sync.Map
}

type lastStateValue struct {
	mu      sync.Mutex
	last    float64
	deleted bool
}

func newLastAggrState() *lastAggrState {
	return &lastAggrState{}
}

func (as *lastAggrState) pushSample(_, outputKey string, value float64) {
again:
	v, ok := as.m.Load(outputKey)
	if !ok {
		// The entry is missing in the map. Try creating it.
		v = &lastStateValue{
			last: value,
		}
		vNew, loaded := as.m.LoadOrStore(outputKey, v)
		if !loaded {
			// The new entry has been successfully created.
			return
		}
		// Use the entry created by a concurrent goroutine.
		v = vNew
	}
	sv := v.(*lastStateValue)
	sv.mu.Lock()
	deleted := sv.deleted
	if !deleted {
		sv.last = value
	}
	sv.mu.Unlock()
	if deleted {
		// The entry has been deleted by the concurrent call to appendSeriesForFlush
		// Try obtaining and updating the entry again.
		goto again
	}
}

func (as *lastAggrState) appendSeriesForFlush(ctx *flushCtx) {
	currentTimeMsec := int64(fasttime.UnixTimestamp()) * 1000
	m := &as.m
	m.Range(func(k, v interface{}) bool {
		// Atomically delete the entry from the map, so new entry is created for the next flush.
		m.Delete(k)

		sv := v.(*lastStateValue)
		sv.mu.Lock()
		last := sv.last
		// Mark the entry as deleted, so it won't be updated anymore by concurrent pushSample() calls.
		sv.deleted = true
		sv.mu.Unlock()
		key := k.(string)
		ctx.appendSeries(key, "last", currentTimeMsec, last)
		return true
	})
}
