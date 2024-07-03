package streamaggr

import (
	"strconv"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/valyala/histogram"
)

// quantilesAggrState calculates output=quantiles, e.g. the the given quantiles over the input samples.
type quantilesAggrState struct {
	m    sync.Map
	phis []float64
}

type quantilesStateValue struct {
	mu             sync.Mutex
	state          [aggrStateSize]*histogram.Fast
	deleted        bool
	deleteDeadline int64
}

func newQuantilesAggrState(phis []float64) *quantilesAggrState {
	return &quantilesAggrState{
		phis: phis,
	}
}

func (as *quantilesAggrState) pushSamples(samples []pushSample, deleteDeadline int64, idx int) {
	for i := range samples {
		s := &samples[i]
		outputKey := getOutputKey(s.key)

	again:
		v, ok := as.m.Load(outputKey)
		if !ok {
			// The entry is missing in the map. Try creating it.
			v = &quantilesStateValue{}
			outputKey = bytesutil.InternString(outputKey)
			vNew, loaded := as.m.LoadOrStore(outputKey, v)
			if loaded {
				// Use the entry created by a concurrent goroutine.
				v = vNew
			}
		}
		sv := v.(*quantilesStateValue)
		sv.mu.Lock()
		deleted := sv.deleted
		if !deleted {
			if sv.state[idx] == nil {
				sv.state[idx] = histogram.GetFast()
			}
			sv.state[idx].Update(s.value)
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

func (as *quantilesAggrState) flushState(ctx *flushCtx, flushTimestamp int64, idx int) {
	m := &as.m
	phis := as.phis
	var quantiles []float64
	var b []byte
	m.Range(func(k, v interface{}) bool {
		sv := v.(*quantilesStateValue)
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
		state := sv.state[idx]
		quantiles = quantiles[:0]
		if state != nil {
			quantiles = state.Quantiles(quantiles[:0], phis)
			histogram.PutFast(state)
			state.Reset()
		}
		sv.mu.Unlock()
		if len(quantiles) > 0 {
			key := k.(string)
			for i, quantile := range quantiles {
				b = strconv.AppendFloat(b[:0], phis[i], 'g', -1, 64)
				phiStr := bytesutil.InternBytes(b)
				ctx.appendSeriesWithExtraLabel(key, "quantiles", flushTimestamp, quantile, "quantile", phiStr)
			}
		}
		return true
	})
}
