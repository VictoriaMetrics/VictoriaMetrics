package streamaggr

import (
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
)

// rateAggrState calculates output=rate, e.g. the counter per-second change.
type rateAggrState struct {
	m      sync.Map
	suffix string
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

func newRateAggrState(suffix string) *rateAggrState {
	return &rateAggrState{
		suffix: suffix,
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

func (as *rateAggrState) flushState(ctx *flushCtx, flushTimestamp int64, idx int) {
	var staleOutputSamples, staleInputSamples int
	m := &as.m
	m.Range(func(k, v interface{}) bool {
		sv := v.(*rateStateValue)
		sv.mu.Lock()

		// check for stale entries
		deleted := flushTimestamp > sv.deleteDeadline
		if deleted {
			// Mark the current entry as deleted
			sv.deleted = deleted
			sv.mu.Unlock()
			staleOutputSamples++
			m.Delete(k)
			return true
		}

		// Delete outdated entries in state
		var rate float64
		totalItems := len(sv.state)
		for k1, state := range sv.state {
			if flushTimestamp > state.deleteDeadline {
				delete(sv.state, k1)
				staleInputSamples++
				continue
			}
			v1 := state.lastValues[idx]
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
			} else {
				totalItems--
			}
			totalItems -= staleInputSamples
			state.lastValues[idx] = rateLastValueState{}
			sv.state[k1] = state
		}

		sv.mu.Unlock()

		if as.suffix == "rate_avg" && totalItems > 0 {
			rate /= float64(totalItems)
		}

		key := k.(string)
		ctx.appendSeries(key, as.suffix, flushTimestamp, rate)
		return true
	})
	ctx.a.staleOutputSamples[as.suffix].Add(staleOutputSamples)
	ctx.a.staleInputSamples[as.suffix].Add(staleInputSamples)
}
