package streamaggr

import (
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
)

// countSeriesAggrState calculates output=count_series, e.g. the number of unique series.
type countSeriesAggrState struct {
	m sync.Map
}

type countSeriesStateValue struct {
	mu            sync.Mutex
	countedSeries map[string]struct{}
	n             uint64
	deleted       bool
}

func newCountSeriesAggrState() *countSeriesAggrState {
	return &countSeriesAggrState{}
}

func (as *countSeriesAggrState) pushSample(inputKey, outputKey string, _ float64) {
again:
	v, ok := as.m.Load(outputKey)
	if !ok {
		// The entry is missing in the map. Try creating it.
		v = &countSeriesStateValue{
			countedSeries: map[string]struct{}{
				inputKey: {},
			},
			n: 1,
		}
		vNew, loaded := as.m.LoadOrStore(outputKey, v)
		if !loaded {
			// The entry has been added to the map.
			return
		}
		// Update the entry created by a concurrent goroutine.
		v = vNew
	}
	sv := v.(*countSeriesStateValue)
	sv.mu.Lock()
	deleted := sv.deleted
	if !deleted {
		if _, ok := sv.countedSeries[inputKey]; !ok {
			sv.countedSeries[inputKey] = struct{}{}
			sv.n++
		}
	}
	sv.mu.Unlock()
	if deleted {
		// The entry has been deleted by the concurrent call to appendSeriesForFlush
		// Try obtaining and updating the entry again.
		goto again
	}
}

func (as *countSeriesAggrState) appendSeriesForFlush(ctx *flushCtx) {
	currentTimeMsec := int64(fasttime.UnixTimestamp()) * 1000
	m := &as.m
	m.Range(func(k, v interface{}) bool {
		// Atomically delete the entry from the map, so new entry is created for the next flush.
		m.Delete(k)

		sv := v.(*countSeriesStateValue)
		sv.mu.Lock()
		n := sv.n
		// Mark the entry as deleted, so it won't be updated anymore by concurrent pushSample() calls.
		sv.deleted = true
		sv.mu.Unlock()
		key := k.(string)
		ctx.appendSeries(key, "count_series", currentTimeMsec, float64(n))
		return true
	})
}
