package streamaggr

import (
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
)

// countSamplesAggrState calculates output=countSamples, e.g. the count of input samples.
type countSamplesAggrState struct {
	m sync.Map
}

type countSamplesStateValue struct {
	mu      sync.Mutex
	n       uint64
	deleted bool
}

func newCountSamplesAggrState() *countSamplesAggrState {
	return &countSamplesAggrState{}
}

func (as *countSamplesAggrState) pushSample(_, outputKey string, _ float64) {
again:
	v, ok := as.m.Load(outputKey)
	if !ok {
		// The entry is missing in the map. Try creating it.
		v = &countSamplesStateValue{
			n: 1,
		}
		vNew, loaded := as.m.LoadOrStore(outputKey, v)
		if !loaded {
			// The new entry has been successfully created.
			return
		}
		// Use the entry created by a concurrent goroutine.
		v = vNew
	}
	sv := v.(*countSamplesStateValue)
	sv.mu.Lock()
	deleted := sv.deleted
	if !deleted {
		sv.n++
	}
	sv.mu.Unlock()
	if deleted {
		// The entry has been deleted by the concurrent call to appendSeriesForFlush
		// Try obtaining and updating the entry again.
		goto again
	}
}

func (as *countSamplesAggrState) appendSeriesForFlush(ctx *flushCtx) {
	currentTimeMsec := int64(fasttime.UnixTimestamp()) * 1000
	m := &as.m
	m.Range(func(k, v interface{}) bool {
		// Atomically delete the entry from the map, so new entry is created for the next flush.
		m.Delete(k)

		sv := v.(*countSamplesStateValue)
		sv.mu.Lock()
		n := sv.n
		// Mark the entry as deleted, so it won't be updated anymore by concurrent pushSample() calls.
		sv.deleted = true
		sv.mu.Unlock()
		key := k.(string)
		ctx.appendSeries(key, "count_samples", currentTimeMsec, float64(n))
		return true
	})
}
