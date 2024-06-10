package streamaggr

import (
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
)

// stdvarAggrState calculates output=stdvar, e.g. the average value over input samples.
type stdvarAggrState struct {
	m sync.Map
}

type stdvarStateValue struct {
	mu      sync.Mutex
	count   float64
	avg     float64
	q       float64
	deleted bool
}

func newStdvarAggrState() *stdvarAggrState {
	return &stdvarAggrState{}
}

func (as *stdvarAggrState) pushSamples(samples []pushSample) {
	for i := range samples {
		s := &samples[i]
		outputKey := getOutputKey(s.key)

	again:
		v, ok := as.m.Load(outputKey)
		if !ok {
			// The entry is missing in the map. Try creating it.
			v = &stdvarStateValue{}
			outputKey = bytesutil.InternString(outputKey)
			vNew, loaded := as.m.LoadOrStore(outputKey, v)
			if loaded {
				// Use the entry created by a concurrent goroutine.
				v = vNew
			}
		}
		sv := v.(*stdvarStateValue)
		sv.mu.Lock()
		deleted := sv.deleted
		if !deleted {
			// See `Rapid calculation methods` at https://en.wikipedia.org/wiki/Standard_deviation
			sv.count++
			avg := sv.avg + (s.value-sv.avg)/sv.count
			sv.q += (s.value - sv.avg) * (s.value - avg)
			sv.avg = avg
		}
		sv.mu.Unlock()
		if deleted {
			// The entry has been deleted by the concurrent call to flushState
			// Try obtaining and updating the entry again.
			goto again
		}
	}
}

func (as *stdvarAggrState) flushState(ctx *flushCtx, resetState bool) {
	currentTimeMsec := int64(fasttime.UnixTimestamp()) * 1000
	m := &as.m
	m.Range(func(k, v interface{}) bool {
		if resetState {
			// Atomically delete the entry from the map, so new entry is created for the next flush.
			m.Delete(k)
		}

		sv := v.(*stdvarStateValue)
		sv.mu.Lock()
		stdvar := sv.q / sv.count
		if resetState {
			// Mark the entry as deleted, so it won't be updated anymore by concurrent pushSample() calls.
			sv.deleted = true
		}
		sv.mu.Unlock()

		key := k.(string)
		ctx.appendSeries(key, "stdvar", currentTimeMsec, stdvar)
		return true
	})
}
