package streamaggr

import (
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
)

// sumSamplesAggrState calculates output=sum_samples, e.g. the sum over input samples.
type sumSamplesAggrState struct {
	m sync.Map
}

type sumSamplesStateValue struct {
	mu      sync.Mutex
	sum     float64
	deleted bool
}

func newSumSamplesAggrState() *sumSamplesAggrState {
	return &sumSamplesAggrState{}
}

func (as *sumSamplesAggrState) pushSamples(samples []pushSample) {
	for i := range samples {
		s := &samples[i]
		outputKey := getOutputKey(s.key)

	again:
		v, ok := as.m.Load(outputKey)
		if !ok {
			// The entry is missing in the map. Try creating it.
			v = &sumSamplesStateValue{
				sum: s.value,
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
		sv := v.(*sumSamplesStateValue)
		sv.mu.Lock()
		deleted := sv.deleted
		if !deleted {
			sv.sum += s.value
		}
		sv.mu.Unlock()
		if deleted {
			// The entry has been deleted by the concurrent call to flushState
			// Try obtaining and updating the entry again.
			goto again
		}
	}
}

func (as *sumSamplesAggrState) flushState(ctx *flushCtx, resetState bool) {
	currentTimeMsec := int64(fasttime.UnixTimestamp()) * 1000
	m := &as.m
	m.Range(func(k, v interface{}) bool {
		if resetState {
			// Atomically delete the entry from the map, so new entry is created for the next flush.
			m.Delete(k)
		}

		sv := v.(*sumSamplesStateValue)
		sv.mu.Lock()
		sum := sv.sum
		if resetState {
			// Mark the entry as deleted, so it won't be updated anymore by concurrent pushSample() calls.
			sv.deleted = true
		}
		sv.mu.Unlock()

		key := k.(string)
		ctx.appendSeries(key, "sum_samples", currentTimeMsec, sum)
		return true
	})
}
