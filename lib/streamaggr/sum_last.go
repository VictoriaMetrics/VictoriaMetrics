package streamaggr

import (
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
)

// sumLastAggrState calculates output=sum_last, e.g. the sum over last input samples.
type sumLastAggrState struct {
	m sync.Map

	// Time series state is dropped if no new samples are received during stalenessSecs.
	stalenessSecs uint64
}

type sumLastStateValue struct {
	mu             sync.Mutex
	lastValues     map[string]sumLastValue
	deleteDeadline uint64
	deleted        bool
}

type sumLastValue struct {
	value          float64
	timestamp      int64
	deleteDeadline uint64
}

func newSumLastAggrState(stalenessInterval time.Duration) *sumLastAggrState {
	stalenessSecs := roundDurationToSecs(stalenessInterval)
	return &sumLastAggrState{
		stalenessSecs: stalenessSecs,
	}
}

func (as *sumLastAggrState) pushSamples(samples []pushSample) {
	currentTime := fasttime.UnixTimestamp()
	deleteDeadline := currentTime + as.stalenessSecs
	for i := range samples {
		s := &samples[i]
		inputKey, outputKey := getInputOutputKey(s.key)

	again:
		v, ok := as.m.Load(outputKey)
		if !ok {
			// The entry is missing in the map. Try creating it.
			v = &sumLastStateValue{
				lastValues: make(map[string]sumLastValue),
			}
			outputKey = bytesutil.InternString(outputKey)
			vNew, loaded := as.m.LoadOrStore(outputKey, v)
			if loaded {
				// Use the entry created by a concurrent goroutine.
				v = vNew
			}
		}
		sv := v.(*sumLastStateValue)
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

func (as *sumLastAggrState) flushState(ctx *flushCtx) {
	currentTime := fasttime.UnixTimestamp()

	as.removeOldEntries(currentTime)

	m := &as.m
	m.Range(func(k, v any) bool {
		sv := v.(*sumLastStateValue)

		sv.mu.Lock()
		sum := 0.0
		count := len(sv.lastValues)
		for _, lv := range sv.lastValues {
			sum += lv.value
		}
		deleted := sv.deleted
		sv.mu.Unlock()

		if count == 0 || deleted {
			// Nothing to update
			return true
		}

		key := k.(string)
		ctx.appendSeries(key, "sum_last", sum)
		return true
	})
}

func (as *sumLastAggrState) removeOldEntries(currentTime uint64) {
	m := &as.m
	m.Range(func(k, v any) bool {
		sv := v.(*sumLastStateValue)

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
