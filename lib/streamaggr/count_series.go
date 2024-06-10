package streamaggr

import (
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/cespare/xxhash/v2"
)

// countSeriesAggrState calculates output=count_series, e.g. the number of unique series.
type countSeriesAggrState struct {
	m sync.Map
}

type countSeriesStateValue struct {
	mu      sync.Mutex
	m       map[uint64]struct{}
	deleted bool
}

func newCountSeriesAggrState() *countSeriesAggrState {
	return &countSeriesAggrState{}
}

func (as *countSeriesAggrState) pushSamples(samples []pushSample) {
	for i := range samples {
		s := &samples[i]
		inputKey, outputKey := getInputOutputKey(s.key)

		// Count unique hashes over the inputKeys instead of unique inputKey values.
		// This reduces memory usage at the cost of possible hash collisions for distinct inputKey values.
		h := xxhash.Sum64(bytesutil.ToUnsafeBytes(inputKey))

	again:
		v, ok := as.m.Load(outputKey)
		if !ok {
			// The entry is missing in the map. Try creating it.
			v = &countSeriesStateValue{
				m: map[uint64]struct{}{
					h: {},
				},
			}
			outputKey = bytesutil.InternString(outputKey)
			vNew, loaded := as.m.LoadOrStore(outputKey, v)
			if !loaded {
				// The entry has been added to the map.
				continue
			}
			// Update the entry created by a concurrent goroutine.
			v = vNew
		}
		sv := v.(*countSeriesStateValue)
		sv.mu.Lock()
		deleted := sv.deleted
		if !deleted {
			if _, ok := sv.m[h]; !ok {
				sv.m[h] = struct{}{}
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

func (as *countSeriesAggrState) flushState(ctx *flushCtx, resetState bool) {
	currentTimeMsec := int64(fasttime.UnixTimestamp()) * 1000
	m := &as.m
	m.Range(func(k, v interface{}) bool {
		if resetState {
			// Atomically delete the entry from the map, so new entry is created for the next flush.
			m.Delete(k)
		}

		sv := v.(*countSeriesStateValue)
		sv.mu.Lock()
		n := len(sv.m)
		if resetState {
			// Mark the entry as deleted, so it won't be updated anymore by concurrent pushSample() calls.
			sv.deleted = true
		}
		sv.mu.Unlock()

		key := k.(string)
		ctx.appendSeries(key, "count_series", currentTimeMsec, float64(n))
		return true
	})
}
