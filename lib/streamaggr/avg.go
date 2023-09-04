package streamaggr

import (
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
)

// avgAggrState calculates output=avg, e.g. the average value over input samples.
type avgAggrState struct {
	m sync.Map
}

type avgStateValue struct {
	mu      sync.Mutex
	sum     float64
	count   int64
	deleted bool
}

func newAvgAggrState() *avgAggrState {
	return &avgAggrState{}
}

func (as *avgAggrState) pushSample(_, outputKey string, value float64) {
again:
	v, ok := as.m.Load(outputKey)
	if !ok {
		// The entry is missing in the map. Try creating it.
		v = &avgStateValue{
			sum:   value,
			count: 1,
		}
		vNew, loaded := as.m.LoadOrStore(outputKey, v)
		if !loaded {
			// The entry has been successfully stored
			return
		}
		// Update the entry created by a concurrent goroutine.
		v = vNew
	}
	sv := v.(*avgStateValue)
	sv.mu.Lock()
	deleted := sv.deleted
	if !deleted {
		sv.sum += value
		sv.count++
	}
	sv.mu.Unlock()
	if deleted {
		// The entry has been deleted by the concurrent call to appendSeriesForFlush
		// Try obtaining and updating the entry again.
		goto again
	}
}

func (as *avgAggrState) appendSeriesForFlush(ctx *flushCtx) {
	currentTimeMsec := int64(fasttime.UnixTimestamp()) * 1000
	m := &as.m
	m.Range(func(k, v interface{}) bool {
		// Atomically delete the entry from the map, so new entry is created for the next flush.
		m.Delete(k)

		sv := v.(*avgStateValue)
		sv.mu.Lock()
		avg := sv.sum / float64(sv.count)
		// Mark the entry as deleted, so it won't be updated anymore by concurrent pushSample() calls.
		sv.deleted = true
		sv.mu.Unlock()
		key := k.(string)
		ctx.appendSeries(key, "avg", currentTimeMsec, avg)
		return true
	})
}
