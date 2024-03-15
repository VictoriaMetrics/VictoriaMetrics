package streamaggr

import (
	"math"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
)

const (
	intervalJitter = 0.05
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
	stalenessSecs uint64

	// Aggregation interval jitter
	intervalJitter int64
}

type totalStateValue struct {
	mu             sync.Mutex
	lastValues     map[string]lastValueState
	total          float64
	deleteDeadline uint64
	deleted        bool
	intervalStart  int64
}

type lastValueState struct {
	value          float64
	timestamp      int64
	deleteDeadline uint64
}

func newTotalAggrState(interval, stalenessInterval time.Duration, resetTotalOnFlush, keepFirstSample bool) *totalAggrState {
	stalenessSecs := roundDurationToSecs(stalenessInterval)
	suffix := "total"
	if resetTotalOnFlush {
		suffix = "increase"
	}
	return &totalAggrState{
		suffix:            suffix,
		resetTotalOnFlush: resetTotalOnFlush,
		stalenessSecs:     stalenessSecs,
		keepFirstSample:   keepFirstSample,
		intervalJitter:    int64(float64(interval.Milliseconds()) * intervalJitter),
	}
}

func (as *totalAggrState) pushSamples(samples []pushSample) {
	currentTime := fasttime.UnixTimestamp()
	currentTimeMsec := int64(currentTime) * 1000
	deleteDeadline := currentTime + as.stalenessSecs
	for i := range samples {
		s := &samples[i]
		inputKey, outputKey := getInputOutputKey(s.key)

	again:
		v, ok := as.m.Load(outputKey)
		if !ok {
			// The entry is missing in the map. Try creating it.
			v = &totalStateValue{
				lastValues:    make(map[string]lastValueState),
				intervalStart: currentTimeMsec,
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
			if s.timestamp >= sv.intervalStart {
				if as.keepFirstSample || ok {
					if s.value >= lv.value {
						sv.total += s.value - lv.value
					} else {
						// counter reset
						sv.total += s.value
					}
				}
				lv.timestamp = s.timestamp
				lv.value = s.value
				lv.deleteDeadline = deleteDeadline
				sv.lastValues[inputKey] = lv
				sv.deleteDeadline = deleteDeadline
			}
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

func (as *totalAggrState) flushState(ctx *flushCtx, resetState bool) {
	currentTime := fasttime.UnixTimestamp()
	currentTimeMsec := int64(currentTime) * 1000

	as.removeOldEntries(currentTime)

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
		sv.intervalStart = currentTimeMsec - as.intervalJitter
		deleted := sv.deleted
		sv.mu.Unlock()
		if !deleted {
			key := k.(string)
			ctx.appendSeries(key, as.suffix, currentTimeMsec, total)
		}
		return true
	})
}
