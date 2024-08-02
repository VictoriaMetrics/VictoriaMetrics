package streamaggr

import (
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
)

// rateAggrState calculates output=rate_avg and rate_sum, e.g. the average per-second increase rate for counter metrics.
type rateAggrState struct {
	m sync.Map

	// isAvg is set to true if rate_avg() must be calculated instead of rate_sum().
	isAvg bool

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

	// increase stores cumulative increase for the current time series on the current aggregation interval
	increase float64

	// prevTimestamp is the timestamp of the last registered sample in the previous aggregation interval
	prevTimestamp int64
}

func newRateAggrState(stalenessInterval time.Duration, isAvg bool) *rateAggrState {
	stalenessSecs := roundDurationToSecs(stalenessInterval)
	return &rateAggrState{
		isAvg:         isAvg,
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

				if s.value >= lv.value {
					lv.increase += s.value - lv.value
				} else {
					// counter reset
					lv.increase += s.value
				}
			} else {
				lv.prevTimestamp = s.timestamp
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

func (as *rateAggrState) flushState(ctx *flushCtx, resetState bool) {
	currentTime := fasttime.UnixTimestamp()
	currentTimeMsec := int64(currentTime) * 1000

	suffix := as.getSuffix()

	as.removeOldEntries(currentTime)

	m := &as.m
	m.Range(func(k, v any) bool {
		sv := v.(*rateStateValue)

		sv.mu.Lock()
		lvs := sv.lastValues
		sumRate := 0.0
		countSeries := 0
		for k1, lv := range lvs {
			d := float64(lv.timestamp-lv.prevTimestamp) / 1000
			if d > 0 {
				sumRate += lv.increase / d
				countSeries++
			}
			if resetState {
				lv.prevTimestamp = lv.timestamp
				lv.increase = 0
				lvs[k1] = lv
			}
		}
		deleted := sv.deleted
		sv.mu.Unlock()

		if countSeries == 0 || deleted {
			// Nothing to update
			return true
		}

		result := sumRate
		if as.isAvg {
			result /= float64(countSeries)
		}

		key := k.(string)
		ctx.appendSeries(key, suffix, currentTimeMsec, result)
		return true
	})
}

func (as *rateAggrState) getSuffix() string {
	if as.isAvg {
		return "rate_avg"
	}
	return "rate_sum"
}

func (as *rateAggrState) removeOldEntries(currentTime uint64) {
	m := &as.m
	m.Range(func(k, v any) bool {
		sv := v.(*rateStateValue)

		sv.mu.Lock()
		if currentTime > sv.deleteDeadline {
			// Mark the current entry as deleted
			sv.deleted = true
			sv.mu.Unlock()
			m.Delete(k)
			return true
		}

		// Delete outdated entries in sv.lastValues
		lvs := sv.lastValues
		for k1, lv := range lvs {
			if currentTime > lv.deleteDeadline {
				delete(lvs, k1)
			}
		}
		sv.mu.Unlock()
		return true
	})
}
