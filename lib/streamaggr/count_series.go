package streamaggr

import (
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/cespare/xxhash/v2"
)

// countSeriesAggrState calculates output=count_series, e.g. the number of unique series.
type countSeriesAggrState struct {
	m sync.Map
}

type countSeriesStateValue struct {
	mu             sync.Mutex
	state          [aggrStateSize]map[uint64]struct{}
	deleted        bool
	deleteDeadline int64
}

func newCountSeriesAggrState() *countSeriesAggrState {
	return &countSeriesAggrState{}
}

func (as *countSeriesAggrState) pushSamples(samples []pushSample, deleteDeadline int64, idx int) {
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
			csv := &countSeriesStateValue{}
			for ic := range csv.state {
				csv.state[ic] = make(map[uint64]struct{})
			}
			v = csv
			outputKey = bytesutil.InternString(outputKey)
			vNew, loaded := as.m.LoadOrStore(outputKey, v)
			if loaded {
				// Update the entry created by a concurrent goroutine.
				v = vNew
			}
		}
		sv := v.(*countSeriesStateValue)
		sv.mu.Lock()
		deleted := sv.deleted
		if !deleted {
			if _, ok := sv.state[idx][h]; !ok {
				sv.state[idx][h] = struct{}{}
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

func (as *countSeriesAggrState) flushState(ctx *flushCtx, flushTimestamp int64, idx int) {
	m := &as.m
	m.Range(func(k, v interface{}) bool {
		sv := v.(*countSeriesStateValue)
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
		sv.state[idx] = make(map[uint64]struct{})
		sv.mu.Unlock()
		if state > 0 {
			key := k.(string)
			ctx.appendSeries(key, "count_series", flushTimestamp, float64(state))
		}
		return true
	})
}
