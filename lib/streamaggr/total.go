package streamaggr

import (
	"math"
	"sync"
	"sync/atomic"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
)

// totalAggrState calculates output=total, total_prometheus, increase and increase_prometheus.
type totalAggrState struct {
	m sync.Map

	// Whether to reset the output value on every flushState call.
	resetTotalOnFlush bool

	// Whether to take into account the first sample in new time series when calculating the output value.
	keepFirstSample bool

	// The first sample per each new series is ignored first two intervals
	// This allows avoiding an initial spike of the output values at startup when new time series
	// cannot be distinguished from already existing series. This is tracked with ignoreFirstSamples.
	ignoreFirstSamples atomic.Int32
}

type totalStateValue struct {
	mu             sync.Mutex
	lastValues     map[string]totalLastValueState
	total          float64
	deleteDeadline int64
	deleted        bool
}

type totalLastValueState struct {
	value          float64
	timestamp      int64
	deleteDeadline int64
}

func newTotalAggrState(resetTotalOnFlush, keepFirstSample bool) *totalAggrState {
	as := &totalAggrState{
		resetTotalOnFlush: resetTotalOnFlush,
		keepFirstSample:   keepFirstSample,
	}
	as.ignoreFirstSamples.Store(2)
	return as
}

func (as *totalAggrState) pushSamples(samples []pushSample, deleteDeadline int64, _ bool) {
	keepFirstSample := as.keepFirstSample && as.ignoreFirstSamples.Load() <= 0
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

func (as *totalAggrState) flushState(ctx *flushCtx) {
	suffix := as.getSuffix()

	as.removeOldEntries(ctx)

	m := &as.m
	m.Range(func(k, v any) bool {
		sv := v.(*totalStateValue)

		sv.mu.Lock()
		total := sv.total
		if as.resetTotalOnFlush {
			sv.total = 0
		} else if math.Abs(sv.total) >= (1 << 53) {
			// It is time to reset the entry, since it starts losing float64 precision
			sv.total = 0
		}
		deleted := sv.deleted
		sv.mu.Unlock()

		if !deleted {
			key := k.(string)
			ctx.appendSeries(key, suffix, total)
		}
		return true
	})
	ignoreFirstSamples := as.ignoreFirstSamples.Load()
	if ignoreFirstSamples > 0 {
		as.ignoreFirstSamples.Add(-1)
	}
}

func (as *totalAggrState) getSuffix() string {
	// Note: this function is at hot path, so it shouldn't allocate.
	if as.resetTotalOnFlush {
		if as.keepFirstSample {
			return "increase"
		}
		return "increase_prometheus"
	}
	if as.keepFirstSample {
		return "total"
	}
	return "total_prometheus"
}

func (as *totalAggrState) removeOldEntries(ctx *flushCtx) {
	m := &as.m
	m.Range(func(k, v any) bool {
		sv := v.(*totalStateValue)

		sv.mu.Lock()
		if ctx.flushTimestamp > sv.deleteDeadline {
			// Mark the current entry as deleted
			sv.deleted = true
			sv.mu.Unlock()
			key := k.(string)
			ctx.a.lc.Delete(bytesutil.ToUnsafeBytes(key), ctx.flushTimestamp)
			m.Delete(k)
			return true
		}

		// Delete outdated entries in sv.lastValues
		lvs := sv.lastValues
		for k1, lv := range lvs {
			if ctx.flushTimestamp > lv.deleteDeadline {
				delete(lvs, k1)
			}
		}
		sv.mu.Unlock()
		return true
	})
}
