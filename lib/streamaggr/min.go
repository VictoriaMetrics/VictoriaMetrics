package streamaggr

import (
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
)

// minAggrState calculates output=min, e.g. the minimum value over input samples.
type minAggrState struct {
	m sync.Map
}

type minStateValue struct {
	mu      sync.Mutex
	min     float64
	deleted bool
}

func newMinAggrState() *minAggrState {
	return &minAggrState{}
}

func (as *minAggrState) pushSample(_, outputKey string, value float64) {
again:
	v, ok := as.m.Load(outputKey)
	if !ok {
		// The entry is missing in the map. Try creating it.
		v = &minStateValue{
			min: value,
		}
		vNew, loaded := as.m.LoadOrStore(outputKey, v)
		if !loaded {
			// The new entry has been successfully created.
			return
		}
		// Use the entry created by a concurrent goroutine.
		v = vNew
	}
	sv := v.(*minStateValue)
	sv.mu.Lock()
	deleted := sv.deleted
	if !deleted {
		if value < sv.min {
			sv.min = value
		}
	}
	sv.mu.Unlock()
	if deleted {
		// The entry has been deleted by the concurrent call to appendSeriesForFlush
		// Try obtaining and updating the entry again.
		goto again
	}
}

func (as *minAggrState) appendSeriesForFlush(ctx *flushCtx) {
	currentTimeMsec := int64(fasttime.UnixTimestamp()) * 1000
	m := &as.m
	m.Range(func(k, v interface{}) bool {
		// Atomically delete the entry from the map, so new entry is created for the next flush.
		m.Delete(k)

		sv := v.(*minStateValue)
		sv.mu.Lock()
		min := sv.min
		// Mark the entry as deleted, so it won't be updated anymore by concurrent pushSample() calls.
		sv.deleted = true
		sv.mu.Unlock()
		key := k.(string)
		ctx.appendSeries(key, "min", currentTimeMsec, min)
		return true
	})
}
