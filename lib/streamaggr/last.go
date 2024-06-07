package streamaggr

import (
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
)

// lastAggrState calculates output=last, e.g. the last value over input samples.
type lastAggrState struct {
	m sync.Map
}

type lastStateValue struct {
	mu        sync.Mutex
	last      float64
	timestamp int64
	deleted   bool
}

func newLastAggrState() *lastAggrState {
	return &lastAggrState{}
}

func (as *lastAggrState) pushSamples(samples []pushSample) {
	for i := range samples {
		s := &samples[i]
		outputKey := getOutputKey(s.key)

	again:
		v, ok := as.m.Load(outputKey)
		if !ok {
			// The entry is missing in the map. Try creating it.
			v = &lastStateValue{
				last:      s.value,
				timestamp: s.timestamp,
			}
			outputKey = bytesutil.InternString(outputKey)
			vNew, loaded := as.m.LoadOrStore(outputKey, v)
			if !loaded {
				// The new entry has been successfully created.
				continue
			}
			// Use the entry created by a concurrent goroutine.
			v = vNew
		}
		sv := v.(*lastStateValue)
		sv.mu.Lock()
		deleted := sv.deleted
		if !deleted {
			if s.timestamp >= sv.timestamp {
				sv.last = s.value
				sv.timestamp = s.timestamp
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

func (as *lastAggrState) flushState(ctx *flushCtx, resetState bool) {
	currentTimeMsec := int64(fasttime.UnixTimestamp()) * 1000
	m := &as.m
	m.Range(func(k, v interface{}) bool {
		if resetState {
			// Atomically delete the entry from the map, so new entry is created for the next flush.
			m.Delete(k)
		}

		sv := v.(*lastStateValue)
		sv.mu.Lock()
		last := sv.last
		if resetState {
			// Mark the entry as deleted, so it won't be updated anymore by concurrent pushSample() calls.
			sv.deleted = true
		}
		sv.mu.Unlock()

		key := k.(string)
		ctx.appendSeries(key, "last", currentTimeMsec, last)
		return true
	})
}
