package streamaggr

import (
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	"strconv"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/valyala/histogram"
)

// quantilesAggrState calculates output=quantiles, e.g. the the given quantiles over the input samples.
type quantilesAggrState struct {
	m sync.Map

	phis []float64
}

type quantilesStateValue struct {
	mu      sync.Mutex
	h       *histogram.Fast
	deleted bool
}

func newQuantilesAggrState(phis []float64) *quantilesAggrState {
	return &quantilesAggrState{
		phis: phis,
	}
}

func (as *quantilesAggrState) pushSample(_, outputKey string, value float64) {
again:
	v, ok := as.m.Load(outputKey)
	if !ok {
		// The entry is missing in the map. Try creating it.
		h := histogram.GetFast()
		v = &quantilesStateValue{
			h: h,
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
		sv.h.Update(value)
	}
	sv.mu.Unlock()
	if deleted {
		// The entry has been deleted by the concurrent call to appendSeriesForFlush
		// Try obtaining and updating the entry again.
		goto again
	}
}

func (as *quantilesAggrState) appendSeriesForFlush(ctx *flushCtx) {
	currentTimeMsec := int64(fasttime.UnixTimestamp()) * 1000
	m := &as.m
	phis := as.phis
	var quantiles []float64
	var b []byte
	m.Range(func(k, v interface{}) bool {
		// Atomically delete the entry from the map, so new entry is created for the next flush.
		m.Delete(k)

		sv := v.(*quantilesStateValue)
		sv.mu.Lock()
		quantiles = sv.h.Quantiles(quantiles[:0], phis)
		histogram.PutFast(sv.h)
		// Mark the entry as deleted, so it won't be updated anymore by concurrent pushSample() calls.
		sv.deleted = true
		sv.mu.Unlock()

		key := k.(string)
		for i, quantile := range quantiles {
			b = strconv.AppendFloat(b[:0], phis[i], 'g', -1, 64)
			phiStr := bytesutil.InternBytes(b)
			ctx.appendSeriesWithExtraLabel(key, as.getOutputName(), currentTimeMsec, quantile, "quantile", phiStr)
		}
		return true
	})
}

func (as *quantilesAggrState) getOutputName() string {
	return "quantiles"
}

func (as *quantilesAggrState) getStateRepresentation(suffix string) []aggrStateRepresentation {
	result := make([]aggrStateRepresentation, 0)
	var b []byte
	as.m.Range(func(k, v any) bool {
		value := v.(*quantilesStateValue)
		value.mu.Lock()
		defer value.mu.Unlock()
		if value.deleted {
			return true
		}
		for i, quantile := range value.h.Quantiles(make([]float64, 0), as.phis) {
			b = strconv.AppendFloat(b[:0], as.phis[i], 'g', -1, 64)
			result = append(result, aggrStateRepresentation{
				metric: getLabelsStringFromKey(k.(string), suffix, as.getOutputName(), prompbmarshal.Label{
					Name:  "quantile",
					Value: bytesutil.InternBytes(b),
				}),
				value: quantile,
			})
		}
		return true
	})
	return result
}
