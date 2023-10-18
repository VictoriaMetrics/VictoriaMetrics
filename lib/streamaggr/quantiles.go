package streamaggr

import (
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	"strconv"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/valyala/histogram"
)

// quantilesAggrState calculates output=quantiles, e.g. the the given quantiles over the input samples.
type quantilesAggrState struct {
	m                 sync.Map
	phis              []float64
	intervalSecs      uint64
	stalenessSecs     uint64
	lastPushTimestamp uint64
}

type quantilesStateValue struct {
	mu             sync.Mutex
	h              *histogram.Fast
	samplesCount   uint64
	deleted        bool
	deleteDeadline uint64
}

func newQuantilesAggrState(interval time.Duration, stalenessInterval time.Duration, phis []float64) *quantilesAggrState {
	return &quantilesAggrState{
		intervalSecs:  roundDurationToSecs(interval),
		stalenessSecs: roundDurationToSecs(stalenessInterval),
		phis:          phis,
	}
}

func (as *quantilesAggrState) pushSample(_, outputKey string, value float64) {
	currentTime := fasttime.UnixTimestamp()
	deleteDeadline := currentTime + as.stalenessSecs

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
		sv.samplesCount++
		sv.deleteDeadline = deleteDeadline
	}
	sv.mu.Unlock()
	if deleted {
		// The entry has been deleted by the concurrent call to appendSeriesForFlush
		// Try obtaining and updating the entry again.
		goto again
	}
}

func (as *quantilesAggrState) removeOldEntries(currentTime uint64) {
	m := &as.m
	m.Range(func(k, v interface{}) bool {
		sv := v.(*quantilesStateValue)

		sv.mu.Lock()
		deleted := currentTime > sv.deleteDeadline
		if deleted {
			// Mark the current entry as deleted
			sv.deleted = deleted
			histogram.PutFast(sv.h)
		}
		sv.mu.Unlock()

		if deleted {
			m.Delete(k)
		}
		return true
	})
}

func (as *quantilesAggrState) appendSeriesForFlush(ctx *flushCtx) {
	currentTime := fasttime.UnixTimestamp()
	currentTimeMsec := int64(currentTime) * 1000

	as.removeOldEntries(currentTime)

	m := &as.m
	phis := as.phis
	var quantiles []float64
	var b []byte
	m.Range(func(k, v interface{}) bool {
		sv := v.(*quantilesStateValue)
		sv.mu.Lock()
		quantiles = sv.h.Quantiles(quantiles[:0], phis)
		sv.h.Reset()
		sv.mu.Unlock()

		key := k.(string)
		for i, quantile := range quantiles {
			b = strconv.AppendFloat(b[:0], phis[i], 'g', -1, 64)
			phiStr := bytesutil.InternBytes(b)
			ctx.appendSeriesWithExtraLabel(key, as.getOutputName(), currentTimeMsec, quantile, "quantile", phiStr)
		}
		return true
	})
	as.lastPushTimestamp = currentTime
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
				currentValue:      quantile,
				lastPushTimestamp: as.lastPushTimestamp,
				nextPushTimestamp: as.lastPushTimestamp + as.intervalSecs,
				samplesCount:      value.samplesCount,
			})
		}
		return true
	})
	return result
}
