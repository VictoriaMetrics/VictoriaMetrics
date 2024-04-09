package streamaggr

import (
	"strconv"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/valyala/histogram"
)

// quantilesAggrState calculates output=quantiles, e.g. the the given quantiles over the input samples.
type quantilesAggrState struct {
	m sync.Map

	phis []float64
}

type quantilesStateValue struct {
	mu      sync.Mutex
	h       map[int64]*histogram.Fast
	deleted bool
}

func newQuantilesAggrState(phis []float64) *quantilesAggrState {
	return &quantilesAggrState{
		phis: phis,
	}
}

func (as *quantilesAggrState) pushSamples(windows map[int64][]pushSample) {
	for ts, samples := range windows {
		for i := range samples {
			s := &samples[i]
			outputKey := getOutputKey(s.key)

		again:
			v, ok := as.m.Load(outputKey)
			if !ok {
				// The entry is missing in the map. Try creating it.
				h := histogram.GetFast()
				v = &quantilesStateValue{
					h: map[int64]*histogram.Fast{
						ts: h,
					},
				}
				vNew, loaded := as.m.LoadOrStore(outputKey, v)
				if loaded {
					// Use the entry created by a concurrent goroutine.
					histogram.PutFast(h)
					v = vNew
				}
			}
			sv := v.(*quantilesStateValue)
			sv.mu.Lock()
			deleted := sv.deleted
			if !deleted {
				sv.h[ts].Update(s.value)
			}
			sv.mu.Unlock()
			if deleted {
				// The entry has been deleted by the concurrent call to flushState
				// Try obtaining and updating the entry again.
				goto again
			}
		}
	}
}

func (as *quantilesAggrState) flushState(ctx *flushCtx, flushTimestamp int64) {
	m := &as.m
	fn := func(states map[int64]*histogram.Fast) map[int64]*histogram.Fast {
		output := make(map[int64]*histogram.Fast)
		if flushTimestamp == -1 {
			for ts, state := range states {
				output[ts] = state
				delete(states, ts)
			}
		} else if state, ok := states[flushTimestamp]; ok {
			output[flushTimestamp] = state
			delete(states, flushTimestamp)
		}
		return output
	}
	phis := as.phis
	var quantiles []float64
	var b []byte
	m.Range(func(k, v interface{}) bool {
		sv := v.(*quantilesStateValue)
		sv.mu.Lock()
		states := fn(sv.h)
		sv.mu.Unlock()
		for ts, state := range states {
			quantiles = state.Quantiles(quantiles[:0], phis)
			histogram.PutFast(state)
			key := k.(string)
			for i, quantile := range quantiles {
				b = strconv.AppendFloat(b[:0], phis[i], 'g', -1, 64)
				phiStr := bytesutil.InternBytes(b)
				ctx.appendSeriesWithExtraLabel(key, "quantiles", ts, quantile, "quantile", phiStr)
			}
		}
		sv.mu.Lock()
		if len(sv.h) == 0 {
			// Mark the entry as deleted, so it won't be updated anymore by concurrent pushSample() calls.
			sv.deleted = true
			sv.mu.Unlock()
			// Atomically delete the entry from the map, so new entry is created for the next flush.
			m.Delete(k)
		} else {
			sv.mu.Unlock()
		}
		return true
	})
}
