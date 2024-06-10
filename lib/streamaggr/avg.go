package streamaggr

import (
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
)

// avgAggrState calculates output=avg, e.g. the average value over input samples.
type avgAggrState struct {
	m sync.Map
}

type avgStateValue struct {
	mu      sync.Mutex
	sum     float64
	count   int64
	deleted bool
}

func newAvgAggrState() *avgAggrState {
	return &avgAggrState{}
}

func (as *avgAggrState) pushSamples(samples []pushSample) {
	for i := range samples {
		s := &samples[i]
		outputKey := getOutputKey(s.key)

	again:
		v, ok := as.m.Load(outputKey)
		if !ok {
			// The entry is missing in the map. Try creating it.
			v = &avgStateValue{
				sum:   s.value,
				count: 1,
			}
			outputKey = bytesutil.InternString(outputKey)
			vNew, loaded := as.m.LoadOrStore(outputKey, v)
			if !loaded {
				// The entry has been successfully stored
				continue
			}
			// Update the entry created by a concurrent goroutine.
			v = vNew
		}
		sv := v.(*avgStateValue)
		sv.mu.Lock()
		deleted := sv.deleted
		if !deleted {
			sv.sum += s.value
			sv.count++
		}
		sv.mu.Unlock()
		if deleted {
			// The entry has been deleted by the concurrent call to flushState
			// Try obtaining and updating the entry again.
			goto again
		}
	}
}

func (as *avgAggrState) flushState(ctx *flushCtx, resetState bool) {
	currentTimeMsec := int64(fasttime.UnixTimestamp()) * 1000
	m := &as.m
	m.Range(func(k, v interface{}) bool {
		if resetState {
			// Atomically delete the entry from the map, so new entry is created for the next flush.
			m.Delete(k)
		}

		sv := v.(*avgStateValue)
		sv.mu.Lock()
		avg := sv.sum / float64(sv.count)
		if resetState {
			// Mark the entry as deleted, so it won't be updated anymore by concurrent pushSample() calls.
			sv.deleted = true
		}
		sv.mu.Unlock()

		key := k.(string)
		ctx.appendSeries(key, "avg", currentTimeMsec, avg)
		return true
	})
}
