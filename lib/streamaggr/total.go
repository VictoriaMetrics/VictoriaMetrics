package streamaggr

import (
	"math"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
)

// totalAggrState calculates output=total, e.g. the summary counter over input counters.
type totalAggrState struct {
	m sync.Map

	suffix string

	// Whether to reset the output value on every flushState call.
	resetTotalOnFlush bool

	// Whether to take into account the first sample in new time series when calculating the output value.
	keepFirstSample bool

	// Time series state is dropped if no new samples are received during stalenessSecs.
	//
	// Aslo, the first sample per each new series is ignored during stalenessSecs even if keepFirstSample is set.
	// see ignoreFirstSampleDeadline for more details.
	stalenessSecs uint64

	// The first sample per each new series is ignored until this unix timestamp deadline in seconds even if keepFirstSample is set.
	// This allows avoiding an initial spike of the output values at startup when new time series
	// cannot be distinguished from already existing series. This is tracked with ignoreFirstSampleDeadline.
	ignoreFirstSampleDeadline uint64
}

type totalStateValue struct {
	mu             sync.Mutex
	lastValues     map[string]totalLastValueState
	total          float64
	deleteDeadline uint64
	deleted        bool
}

type totalLastValueState struct {
	value          float64
	timestamp      int64
	deleteDeadline uint64
}

func newTotalAggrState(stalenessInterval time.Duration, resetTotalOnFlush, keepFirstSample bool) *totalAggrState {
	stalenessSecs := roundDurationToSecs(stalenessInterval)
	ignoreFirstSampleDeadline := fasttime.UnixTimestamp() + stalenessSecs
	suffix := "total"
	if resetTotalOnFlush {
		suffix = "increase"
	}
	if !keepFirstSample {
		suffix += "_prometheus"
	}
	return &totalAggrState{
		suffix:                    suffix,
		resetTotalOnFlush:         resetTotalOnFlush,
		keepFirstSample:           keepFirstSample,
		stalenessSecs:             stalenessSecs,
		ignoreFirstSampleDeadline: ignoreFirstSampleDeadline,
	}
}

func (as *totalAggrState) pushSamples(samples []pushSample) {
	currentTime := fasttime.UnixTimestamp()
	deleteDeadline := currentTime + as.stalenessSecs
	keepFirstSample := as.keepFirstSample && currentTime > as.ignoreFirstSampleDeadline
	for i := range samples {
		s := &samples[i]
		inputKey, outputKey := getInputOutputKey(s.key)

	again:
		v, ok := as.m.Load(outputKey)
		if !ok {
			// The entry is missing in the map. Try creating it.
			v = &totalStateValue{
				lastValues: make(map[string]totalLastValueState),
			}
			outputKey = bytesutil.InternString(outputKey)
			vNew, loaded := as.m.LoadOrStore(outputKey, v)
			if loaded {
				// Use the entry created by a concurrent goroutine.
				v = vNew
			}
		}
		sv := v.(*totalStateValue)
		sv.mu.Lock()
		deleted := sv.deleted
		if !deleted {
			lv, ok := sv.lastValues[inputKey]
			if ok || keepFirstSample {
				if s.timestamp < lv.timestamp {
					// Skip out of order sample
					sv.mu.Unlock()
					continue
				}

				if s.value >= lv.value {
					sv.total += s.value - lv.value
				} else {
					// counter reset
					sv.total += s.value
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

func (as *totalAggrState) removeOldEntries(ctx *flushCtx, currentTime uint64) {
	m := &as.m
	var staleInputSamples, staleOutputSamples int
	m.Range(func(k, v interface{}) bool {
		sv := v.(*totalStateValue)

		sv.mu.Lock()
		deleted := currentTime > sv.deleteDeadline
		if deleted {
			// Mark the current entry as deleted
			sv.deleted = deleted
			staleOutputSamples++
		} else {
			// Delete outdated entries in sv.lastValues
			m := sv.lastValues
			for k1, v1 := range m {
				if currentTime > v1.deleteDeadline {
					delete(m, k1)
					staleInputSamples++
				}
			}
		}
		sv.mu.Unlock()

		if deleted {
			m.Delete(k)
		}
		return true
	})
	ctx.a.staleInputSamples[as.suffix].Add(staleInputSamples)
	ctx.a.staleOutputSamples[as.suffix].Add(staleOutputSamples)
}

func (as *totalAggrState) flushState(ctx *flushCtx, resetState bool) {
	currentTime := fasttime.UnixTimestamp()
	currentTimeMsec := int64(currentTime) * 1000

	as.removeOldEntries(ctx, currentTime)

	m := &as.m
	m.Range(func(k, v interface{}) bool {
		sv := v.(*totalStateValue)
		sv.mu.Lock()
		total := sv.total
		if resetState {
			if as.resetTotalOnFlush {
				sv.total = 0
			} else if math.Abs(sv.total) >= (1 << 53) {
				// It is time to reset the entry, since it starts losing float64 precision
				sv.total = 0
			}
		}
		deleted := sv.deleted
		sv.mu.Unlock()
		if !deleted {
			key := k.(string)
			ctx.appendSeries(key, as.suffix, currentTimeMsec, total)
		}
		return true
	})
}
