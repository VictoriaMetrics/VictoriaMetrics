package streamaggr

import (
	"math"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
)

// stddevAggrState calculates output=stddev, e.g. the average value over input samples.
type stddevAggrState struct {
	m sync.Map
}

type stddevStateValue struct {
	mu             sync.Mutex
	count          float64
	avg            float64
	q              float64
	deleted        bool
	deleteDeadline int64
}

func newStddevAggrState() *stddevAggrState {
	return &stddevAggrState{}
}

func (as *stddevAggrState) pushSamples(samples []pushSample, deleteDeadline int64, includeInputKey bool) {
	for i := range samples {
		s := &samples[i]
		outputKey := getOutputKey(s.key, includeInputKey)

	again:
		v, ok := as.m.Load(outputKey)
		if !ok {
			// The entry is missing in the map. Try creating it.
			v = &stddevStateValue{}
			outputKey = bytesutil.InternString(outputKey)
			vNew, loaded := as.m.LoadOrStore(outputKey, v)
			if loaded {
				// Use the entry created by a concurrent goroutine.
				v = vNew
			}
		}
		sv := v.(*stddevStateValue)
		sv.mu.Lock()
		deleted := sv.deleted
		if !deleted {
			// See `Rapid calculation methods` at https://en.wikipedia.org/wiki/Standard_deviation
			sv.count++
			avg := sv.avg + (s.value-sv.avg)/sv.count
			sv.q += (s.value - sv.avg) * (s.value - avg)
			sv.avg = avg
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

func (as *stddevAggrState) flushState(ctx *flushCtx) {
	m := &as.m
	m.Range(func(k, v any) bool {
		// Atomically delete the entry from the map, so new entry is created for the next flush.
		m.Delete(k)

		sv := v.(*stddevStateValue)
		sv.mu.Lock()
		if ctx.flushTimestamp > sv.deleteDeadline {
			sv.deleted = true
			sv.mu.Unlock()
			key := k.(string)
			ctx.a.lc.Delete(bytesutil.ToUnsafeBytes(key), ctx.flushTimestamp)
			m.Delete(k)
			return true
		}
		if sv.count == 0 {
			sv.mu.Unlock()
			return true
		}

		stddev := math.Sqrt(sv.q / sv.count)
		sv.q = 0
		sv.count = 0
		sv.avg = 0
		sv.mu.Unlock()

		key := k.(string)
		ctx.appendSeries(key, "stddev", stddev)
		return true
	})
}
