package streamaggr

import (
	"github.com/VictoriaMetrics/metrics"
	"math"
	"slices"
	"sync"
	"time"
)

// totalAggrState calculates output=total, e.g. the summary counter over input counters.
type totalAggrState struct {
	m sync.Map

	suffix string

	// Whether to reset the output value on every flushState call.
	resetTotalOnFlush bool

	// Whether to take into account the first sample in new time series when calculating the output value.
	keepFirstSample bool

	intervalSecs uint64

	// Time series state is dropped if no new samples are received during stalenessSecs.
	//
	// Aslo, the first sample per each new series is ignored during stalenessSecs even if keepFirstSample is set.
	// see ignoreFirstSampleDeadline for more details.
	stalenessSecs uint64

	// The first sample per each new series is ignored until this unix timestamp deadline in seconds even if keepFirstSample is set.
	// This allows avoiding an initial spike of the output values at startup when new time series
	// cannot be distinguished from already existing series. This is tracked with ignoreFirstSampleDeadline.
	ignoreFirstSampleDeadline uint64

	// If greater than zero, the number of seconds (inclusive) a sample can be delayed.
	// In this mode, sample timestamps are taken into account and aggregated within intervals (defined by intervalSecs).
	// Aggregated samples have timestamps aligned with the interval.
	delaySecs uint64

	// Get the current timestamp in Unix seconds. Overridden for testing.
	nowFunc func() uint64

	// Count of samples dropped due to lateness.
	lateSamplesDropped *metrics.Counter

	// Ingestion latency between sample's timestamp and current time.
	ingestionLatency *metrics.Histogram
}

type totalStateValue struct {
	mu             sync.Mutex
	lastValues     map[string]lastValueState
	deltas         map[uint64]float64
	total          float64
	deleteDeadline uint64
	deleted        bool
}

type lastValueState struct {
	value          float64
	timestamp      int64
	deleteDeadline uint64
}

func newTotalAggrState(interval, stalenessInterval, delay time.Duration, resetTotalOnFlush, keepFirstSample bool, nowFunc func() uint64, ms *metrics.Set) *totalAggrState {
	intervalSecs := roundDurationToSecs(interval)
	stalenessSecs := roundDurationToSecs(stalenessInterval)
	delaySecs := roundDurationToSecs(delay)
	ignoreFirstSampleDeadline := nowFunc() + stalenessSecs
	suffix := "total"
	if resetTotalOnFlush {
		suffix = "increase"
	}
	lateSamplesDropped := ms.GetOrCreateCounter(`vm_streamaggr_late_samples_dropped_total`)
	ingestionLatency := ms.GetOrCreateHistogram(`vm_streamaggr_ingestion_latency_seconds`)
	return &totalAggrState{
		suffix:                    suffix,
		resetTotalOnFlush:         resetTotalOnFlush,
		keepFirstSample:           keepFirstSample,
		intervalSecs:              intervalSecs,
		stalenessSecs:             stalenessSecs,
		delaySecs:                 delaySecs,
		ignoreFirstSampleDeadline: ignoreFirstSampleDeadline,
		nowFunc:                   nowFunc,
		lateSamplesDropped:        lateSamplesDropped,
		ingestionLatency:          ingestionLatency,
	}
}

// alignToInterval rounds up timestamp to the nearest interval
func alignToInterval(timestamp, interval uint64) uint64 {
	if timestamp%interval == 0 {
		return timestamp
	}
	return interval - (timestamp % interval) + timestamp
}

func (as *totalAggrState) addDelta(sv *totalStateValue, delta float64, timestampSecs uint64) {
	if as.delaySecs == 0 {
		sv.total += delta
	} else {
		intervalEnd := alignToInterval(timestampSecs, as.intervalSecs)
		sv.deltas[intervalEnd] += delta
	}
}

func (as *totalAggrState) pushSamples(samples []pushSample) {
	currentTime := as.nowFunc()
	deleteDeadline := currentTime + as.stalenessSecs
	keepFirstSample := as.keepFirstSample && currentTime > as.ignoreFirstSampleDeadline
	for i := range samples {
		s := &samples[i]
		timestampSecs := uint64(s.timestamp / 1000)
		as.ingestionLatency.Update(float64(currentTime) - float64(timestampSecs))

		if as.delaySecs > 0 {
			if timestampSecs < currentTime-as.delaySecs {
				as.lateSamplesDropped.Inc()
				continue
			}
		}

		inputKey, outputKey := getInputOutputKey(s.key)

	again:
		v, ok := as.m.Load(outputKey)
		if !ok {
			// The entry is missing in the map. Try creating it.
			v = &totalStateValue{
				lastValues: make(map[string]lastValueState),
				deltas:     make(map[uint64]float64),
			}
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
					as.addDelta(sv, s.value-lv.value, timestampSecs)
				} else {
					// counter reset
					as.addDelta(sv, s.value, timestampSecs)
				}
			}
			lv.value = s.value
			lv.timestamp = s.timestamp
			lv.deleteDeadline = deleteDeadline
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

func (as *totalAggrState) removeOldEntries(currentTime uint64) {
	m := &as.m
	m.Range(func(k, v interface{}) bool {
		sv := v.(*totalStateValue)

		sv.mu.Lock()
		deleted := currentTime > sv.deleteDeadline
		if deleted {
			// Mark the current entry as deleted
			sv.deleted = deleted
		} else {
			// Delete outdated entries in sv.lastValues
			m := sv.lastValues
			for k1, v1 := range m {
				if currentTime > v1.deleteDeadline {
					delete(m, k1)
				}
			}
		}
		sv.mu.Unlock()

		if deleted {
			m.Delete(k)
		}
		return true
	})
}

func cmpUint64(a, b uint64) int {
	if a < b {
		return -1
	} else if a > b {
		return 1
	}
	return 0
}

func (as *totalAggrState) flushState(ctx *flushCtx, resetState bool) {
	currentTime := as.nowFunc()
	currentTimeMsec := int64(currentTime) * 1000

	as.removeOldEntries(currentTime)

	m := &as.m
	m.Range(func(k, v interface{}) bool {
		sv := v.(*totalStateValue)
		key := k.(string)
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

		if as.delaySecs == 0 {
			deleted := sv.deleted
			sv.mu.Unlock()
			if !deleted {
				ctx.appendSeries(key, as.suffix, currentTimeMsec, total)
			}
		} else {
			flushableIntervals := make([]uint64, 0, len(sv.deltas))
			for intervalEnd := range sv.deltas {
				if intervalEnd < currentTime-as.delaySecs {
					flushableIntervals = append(flushableIntervals, intervalEnd)
				}
			}
			slices.SortFunc(flushableIntervals, cmpUint64)

			totals := make([]float64, len(flushableIntervals))
			for i, intervalEnd := range flushableIntervals {
				total += sv.deltas[intervalEnd]
				totals[i] = total
				delete(sv.deltas, intervalEnd)
			}
			sv.total = total

			deleted := sv.deleted
			sv.mu.Unlock()
			if !deleted {
				for i, intervalEnd := range flushableIntervals {
					ctx.appendSeries(key, as.suffix, int64(intervalEnd*1000), totals[i])
				}
			}
		}
		return true
	})
}
