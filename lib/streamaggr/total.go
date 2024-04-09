package streamaggr

import (
	"math"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"go.uber.org/atomic"
)

// totalAggrState calculates output=total, e.g. the summary counter over input counters.
type totalAggrState struct {
	m sync.Map

	suffix string

	// Whether to reset the output value on every flushState call.
	resetTotalOnFlush bool

	// Whether to take into account the first sample in new time series when calculating the output value.
	keepFirstSample bool

	// Time series state is dropped if no new samples are received during stalenessMsecs.
	//
	// Aslo, the first sample per each new series is ignored during stalenessMsecs even if keepFirstSample is set.
	stalenessMsecs int64
}

type totalStateValue struct {
	mu             sync.Mutex
	lastValues     map[string]lastValueState
	total          map[int64]float64
	totalState     atomic.Float64
	deleteDeadline int64
	deleted        bool
}

type lastValueState struct {
	value          float64
	timestamp      int64
	deleteDeadline int64
}

func newTotalAggrState(stalenessInterval time.Duration, resetTotalOnFlush, keepFirstSample bool) *totalAggrState {
	stalenessMsecs := durationToMsecs(stalenessInterval)
	suffix := "total"
	if resetTotalOnFlush {
		suffix = "increase"
	}
	return &totalAggrState{
		suffix:            suffix,
		resetTotalOnFlush: resetTotalOnFlush,
		keepFirstSample:   keepFirstSample,
		stalenessMsecs:    stalenessMsecs,
	}
}

func (as *totalAggrState) pushSamples(windows map[int64][]pushSample) {
	currentTime := fasttime.UnixMilli()
	deleteDeadline := currentTime + as.stalenessMsecs
	for ts, samples := range windows {
		for i := range samples {
			s := &samples[i]
			inputKey, outputKey := getInputOutputKey(s.key)

		again:
			v, ok := as.m.Load(outputKey)
			if !ok {
				// The entry is missing in the map. Try creating it.
				v = &totalStateValue{
					lastValues: make(map[string]lastValueState),
					total:      make(map[int64]float64),
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
				if ok || as.keepFirstSample {
					if s.timestamp < lv.timestamp {
						// Skip out of order sample
						sv.mu.Unlock()
						continue
					}
					if s.value >= lv.value {
						sv.total[ts] += s.value - lv.value
					} else {
						// counter reset
						sv.total[ts] += s.value
					}
				} else if _, ok := sv.total[ts]; !ok {
					sv.total[ts] = 0
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
}

func (as *totalAggrState) removeOldEntries(currentTime int64) {
	m := &as.m
	m.Range(func(k, v interface{}) bool {
		sv := v.(*totalStateValue)
		sv.mu.Lock()
		deleted := currentTime > sv.deleteDeadline || len(sv.total) == 0
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

func (as *totalAggrState) flushState(ctx *flushCtx, flushTimestamp int64) {
	as.removeOldEntries(flushTimestamp)
	m := &as.m
	fn := func(states map[int64]float64) []float64 {
		output := make([]float64, 0)
		if flushTimestamp == -1 {
			for ts := range states {
				delete(states, ts)
			}
		} else if state, ok := states[flushTimestamp]; ok {
			output = append(output, state)
			delete(states, flushTimestamp)
		}
		return output
	}
	m.Range(func(k, v interface{}) bool {
		sv := v.(*totalStateValue)
		sv.mu.Lock()
		states := fn(sv.total)
		deleted := sv.deleted
		sv.mu.Unlock()
		for _, state := range states {
			if as.resetTotalOnFlush {
				sv.totalState.Store(0)
			} else if math.Abs(state) >= (1 << 53) {
				// It is time to reset the entry, since it starts losing float64 precision
				sv.totalState.Store(0)
			} else {
				state += sv.totalState.Load()
				sv.totalState.Store(state)
			}
			if !deleted {
				key := k.(string)
				ctx.appendSeries(key, as.suffix, flushTimestamp, state)
			}
		}
		return true
	})
}
