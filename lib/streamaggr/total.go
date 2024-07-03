package streamaggr

import (
	"math"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
)

// totalAggrState calculates output=total, total_prometheus, increase and increase_prometheus.
type totalAggrState struct {
	m sync.Map

	// Whether to reset the output value on every flushState call.
	resetTotalOnFlush bool

	// Whether to take into account the first sample in new time series when calculating the output value.
	keepFirstSample bool

	// The first sample per each new series is ignored until this unix timestamp deadline in seconds even if keepFirstSample is set.
	// This allows avoiding an initial spike of the output values at startup when new time series
	// cannot be distinguished from already existing series. This is tracked with ignoreFirstSampleDeadline.
	ignoreFirstSampleDeadline uint64
}

type totalStateValue struct {
	mu             sync.Mutex
	shared         totalState
	state          [aggrStateSize]float64
	deleteDeadline int64
	deleted        bool
}

type totalState struct {
	total      float64
	lastValues map[string]totalLastValueState
}

type totalLastValueState struct {
	value          float64
	timestamp      int64
	deleteDeadline int64
}

func newTotalAggrState(ignoreFirstSampleInterval time.Duration, resetTotalOnFlush, keepFirstSample bool) *totalAggrState {
	ignoreFirstSampleDeadline := time.Now().Add(ignoreFirstSampleInterval)

	return &totalAggrState{
		resetTotalOnFlush:         resetTotalOnFlush,
		keepFirstSample:           keepFirstSample,
		ignoreFirstSampleDeadline: uint64(ignoreFirstSampleDeadline.Unix()),
	}
}

func (as *totalAggrState) pushSamples(samples []pushSample, deleteDeadline int64, idx int) {
	var deleted bool
	currentTime := fasttime.UnixTimestamp()
	keepFirstSample := as.keepFirstSample && currentTime >= as.ignoreFirstSampleDeadline
	for i := range samples {
		s := &samples[i]
		inputKey, outputKey := getInputOutputKey(s.key)

	again:
		v, ok := as.m.Load(outputKey)
		if !ok {
			// The entry is missing in the map. Try creating it.
			v = &totalStateValue{
				shared: totalState{
					lastValues: make(map[string]totalLastValueState),
				},
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
		deleted = sv.deleted
		if !deleted {
			lv, ok := sv.shared.lastValues[inputKey]
			if ok || keepFirstSample {
				if s.timestamp < lv.timestamp {
					// Skip out of order sample
					sv.mu.Unlock()
					continue
				}

				if s.value >= lv.value {
					sv.state[idx] += s.value - lv.value
				} else {
					// counter reset
					sv.state[idx] += s.value
				}
			}
			lv.value = s.value
			lv.timestamp = s.timestamp
			lv.deleteDeadline = deleteDeadline

			inputKey = bytesutil.InternString(inputKey)
			sv.shared.lastValues[inputKey] = lv
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

func (as *totalAggrState) flushState(ctx *flushCtx) {
	var total float64
	m := &as.m
	suffix := as.getSuffix()
	m.Range(func(k, v interface{}) bool {
		sv := v.(*totalStateValue)

		sv.mu.Lock()
		// check for stale entries
		deleted := ctx.flushTimestamp > sv.deleteDeadline
		if deleted {
			// Mark the current entry as deleted
			sv.deleted = deleted
			sv.mu.Unlock()
			m.Delete(k)
			return true
		}
		total = sv.shared.total + sv.state[ctx.idx]
		for k1, v1 := range sv.shared.lastValues {
			if ctx.flushTimestamp > v1.deleteDeadline {
				delete(sv.shared.lastValues, k1)
			}
		}
		sv.state[ctx.idx] = 0
		if !as.resetTotalOnFlush {
			if math.Abs(total) >= (1 << 53) {
				// It is time to reset the entry, since it starts losing float64 precision
				sv.shared.total = 0
			} else {
				sv.shared.total = total
			}
		}
		sv.mu.Unlock()
		key := k.(string)
		ctx.appendSeries(key, suffix, total)
		return true
	})
}
