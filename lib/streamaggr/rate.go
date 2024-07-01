package streamaggr

import (
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
)

// rateAggrState calculates output=rate, e.g. the counter per-second change.
type rateAggrState struct {
	m sync.Map

	suffix string

	// Time series state is dropped if no new samples are received during stalenessSecs.
	stalenessSecs uint64
}

type rateStateValue struct {
	mu             sync.Mutex
	lastValues     map[string]rateLastValueState
	deleteDeadline uint64
	deleted        bool
}

type rateLastValueState struct {
	value          float64
	timestamp      int64
	deleteDeadline uint64

	// total stores cumulative difference between registered values
	// in the aggregation interval
	total float64
	// prevTimestamp stores timestamp of the last registered value
	// in the previous aggregation interval
	prevTimestamp int64
}

func newRateAggrState(stalenessInterval time.Duration, suffix string) *rateAggrState {
	stalenessSecs := roundDurationToSecs(stalenessInterval)
	return &rateAggrState{
		suffix:        suffix,
		stalenessSecs: stalenessSecs,
	}
}

func (as *rateAggrState) pushSamples(samples []pushSample) {
	currentTime := fasttime.UnixTimestamp()
	deleteDeadline := currentTime + as.stalenessSecs
	for i := range samples {
		s := &samples[i]
		inputKey, outputKey := getInputOutputKey(s.key)

	again:
		v, ok := as.m.Load(outputKey)
		if !ok {
			// The entry is missing in the map. Try creating it.
			v = &rateStateValue{
				lastValues: make(map[string]rateLastValueState),
			}
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
			lv, ok := sv.lastValues[inputKey]
			if ok {
				if s.timestamp < lv.timestamp {
					// Skip out of order sample
					sv.mu.Unlock()
					continue
				}
				if lv.prevTimestamp == 0 {
					lv.prevTimestamp = lv.timestamp
				}
				if s.value >= lv.value {
					lv.total += s.value - lv.value
				} else {
					// counter reset
					lv.total += s.value
				}
			}
			lv.value = s.value
			lv.timestamp = s.timestamp
			lv.deleteDeadline = deleteDeadline

			inputKey = bytesutil.InternString(inputKey)
			sv.lastValues[inputKey] = lv
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

func (as *rateAggrState) flushState(ctx *flushCtx, _ bool) {
	currentTime := fasttime.UnixTimestamp()
	currentTimeMsec := int64(currentTime) * 1000
	var staleOutputSamples, staleInputSamples int

	m := &as.m
	m.Range(func(k, v interface{}) bool {
		sv := v.(*rateStateValue)
		sv.mu.Lock()

		// check for stale entries
		deleted := currentTime > sv.deleteDeadline
		if deleted {
			// Mark the current entry as deleted
			sv.deleted = deleted
			sv.mu.Unlock()
			staleOutputSamples++
			m.Delete(k)
			return true
		}

		// Delete outdated entries in sv.lastValues
		var rate float64
		lvs := sv.lastValues
		for k1, v1 := range lvs {
			if currentTime > v1.deleteDeadline {
				delete(lvs, k1)
				staleInputSamples++
				continue
			}
			rateInterval := v1.timestamp - v1.prevTimestamp
			if v1.prevTimestamp > 0 && rateInterval > 0 {
				// calculate rate only if value was seen at least twice with different timestamps
				rate += v1.total * 1000 / float64(rateInterval)
				v1.prevTimestamp = v1.timestamp
				v1.total = 0
				lvs[k1] = v1
			}
		}
		// capture m length after deleted items were removed
		totalItems := len(lvs)
		sv.mu.Unlock()

		if as.suffix == "rate_avg" && totalItems > 0 {
			rate /= float64(totalItems)
		}

		key := k.(string)
		ctx.appendSeries(key, as.suffix, currentTimeMsec, rate)
		return true
	})
	ctx.a.staleOutputSamples[as.suffix].Add(staleOutputSamples)
	ctx.a.staleInputSamples[as.suffix].Add(staleInputSamples)
}
