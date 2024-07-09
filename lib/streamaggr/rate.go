package streamaggr

import (
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
)

// rateAggrState calculates output=rate_avg and rate_sum, e.g. the average per-second increase rate for counter metrics.
type rateAggrState struct {
	m sync.Map

	// isAvg is set to true if rate_avg() must be calculated instead of rate_sum().
	isAvg bool
}

type rateStateValue struct {
	mu             sync.Mutex
	state          map[string]rateState
	deleted        bool
	deleteDeadline int64
}

type rateState struct {
	lastValues [aggrStateSize]rateLastValueState
	// prevTimestamp stores timestamp of the last registered value
	// in the previous aggregation interval
	prevTimestamp int64

	// prevValue stores last registered value
	// in the previous aggregation interval
	prevValue      float64
	deleteDeadline int64
}

type rateLastValueState struct {
	firstValue float64
	value      float64
	timestamp  int64

	// total stores cumulative difference between registered values
	// in the aggregation interval
	total float64
}

func newRateAggrState(isAvg bool) *rateAggrState {
	return &rateAggrState{
		isAvg: isAvg,
	}
}

func (as *rateAggrState) pushSamples(samples []pushSample, deleteDeadline int64, idx int) {
	for i := range samples {
		s := &samples[i]
		inputKey, outputKey := getInputOutputKey(s.key)

	again:
		v, ok := as.m.Load(outputKey)
		if !ok {
			// The entry is missing in the map. Try creating it.
			rsv := &rateStateValue{
				state: make(map[string]rateState),
			}
			v = rsv
			outputKey = bytesutil.InternString(outputKey)
			vNew, loaded := as.m.LoadOrStore(outputKey, v)
			if loaded {
				// Use the entry created by a concurrent goroutine.
				v = vNew
			}
		}
		sv := v.(*rateStateValue)
		sv.mu.Lock()
		deleted := sv.deleted
		if !deleted {
			state, ok := sv.state[inputKey]
			lv := state.lastValues[idx]
			if ok && lv.timestamp > 0 {
				if s.timestamp < lv.timestamp {
					// Skip out of order sample
					sv.mu.Unlock()
					continue
				}
				if state.prevTimestamp == 0 {
					state.prevTimestamp = lv.timestamp
					state.prevValue = lv.value
				}
				if s.value >= lv.value {
					lv.total += s.value - lv.value
				} else {
					// counter reset
					lv.total += s.value
				}
			} else if state.prevTimestamp > 0 {
				lv.firstValue = s.value
			}
			lv.value = s.value
			lv.timestamp = s.timestamp
			state.lastValues[idx] = lv
			state.deleteDeadline = deleteDeadline
			inputKey = bytesutil.InternString(inputKey)
			sv.state[inputKey] = state
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

func (as *rateAggrState) getSuffix() string {
	if as.isAvg {
		return "rate_avg"
	}
	return "rate_sum"
}

func (as *rateAggrState) flushState(ctx *flushCtx) {
	m := &as.m
	suffix := as.getSuffix()
	m.Range(func(k, v any) bool {
		sv := v.(*rateStateValue)
		sv.mu.Lock()

		// check for stale entries
		deleted := ctx.flushTimestamp > sv.deleteDeadline
		if deleted {
			// Mark the current entry as deleted
			sv.deleted = true
			sv.mu.Unlock()
			m.Delete(k)
			return true
		}

		// Delete outdated entries in state
		rate := 0.0
		countSeries := 0
		for k1, state := range sv.state {
			if ctx.flushTimestamp > state.deleteDeadline {
				delete(sv.state, k1)
				continue
			}
			v1 := state.lastValues[ctx.idx]
			rateInterval := v1.timestamp - state.prevTimestamp
			if rateInterval > 0 && state.prevTimestamp > 0 {
				if v1.firstValue >= state.prevValue {
					v1.total += v1.firstValue - state.prevValue
				} else {
					v1.total += v1.firstValue
				}

				// calculate rate only if value was seen at least twice with different timestamps
				rate += (v1.total) * 1000 / float64(rateInterval)
				state.prevTimestamp = v1.timestamp
				state.prevValue = v1.value
				countSeries++
			}
			state.lastValues[ctx.idx] = rateLastValueState{}
			sv.state[k1] = state
		}

		sv.mu.Unlock()

		if countSeries > 0 {
			if as.isAvg {
				rate /= float64(countSeries)
			}
			key := k.(string)
			ctx.appendSeries(key, suffix, rate)
		}
		return true
	})
}
