package streamaggr

import (
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
)

// rateAggrState calculates output=rate, e.g. the summary counter over input counters.
type rateAggrState struct {
	m sync.Map

	suffix string

	// Time series state is dropped if no new samples are received during stalenessSecs.
	stalenessSecs uint64
}

type rateStateValue struct {
	mu             sync.Mutex
	lastValues     map[string]*rateLastValueState
	deleteDeadline uint64
	deleted        bool
}

type rateLastValueState struct {
	value          float64
	timestamp      int64
	deleteDeadline uint64
	total          float64
	startTimestamp int64
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
				lastValues: make(map[string]*rateLastValueState),
			}
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
				if lv.startTimestamp == 0 {
					lv.startTimestamp = lv.timestamp
				}
				if s.value >= lv.value {
					lv.total += s.value - lv.value
				} else {
					// counter reset
					lv.total += s.value
				}
			} else {
				lv = &rateLastValueState{}
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

func (as *rateAggrState) flushState(ctx *flushCtx, resetState bool) {
	currentTime := fasttime.UnixTimestamp()
	currentTimeMsec := int64(currentTime) * 1000

	m := &as.m
	m.Range(func(k, v interface{}) bool {
		sv := v.(*rateStateValue)
		sv.mu.Lock()
		deleted := currentTime > sv.deleteDeadline
		if deleted {
			// Mark the current entry as deleted
			sv.deleted = deleted
			sv.mu.Unlock()
			m.Delete(k)
		} else {
			// Delete outdated entries in sv.lastValues
			var rate float64
			m := sv.lastValues
			count := float64(len(m))
			for k1, v1 := range m {
				if currentTime > v1.deleteDeadline {
					delete(m, k1)
				} else if v1.startTimestamp > 0 {
					rate += v1.total * 1000 / float64(v1.timestamp-v1.startTimestamp)
					v1.startTimestamp = v1.timestamp
					v1.total = 0
				}
			}
			sv.mu.Unlock()
			key := k.(string)
			if as.suffix == "rate_avg" {
				rate /= count
			}
			ctx.appendSeries(key, as.suffix, currentTimeMsec, rate)
		}
		return true
	})
}
