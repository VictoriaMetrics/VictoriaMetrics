package streamaggr

import (
	"math"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
)

// stddevAggrState calculates output=stddev, e.g. the average value over input samples.
type stddevAggrState struct {
	m sync.Map
}

type stddevStateValue struct {
	mu      sync.Mutex
	count   float64
	avg     float64
	q       float64
	deleted bool
}

func newStddevAggrState() *stddevAggrState {
	return &stddevAggrState{}
}

func (as *stddevAggrState) pushSample(_, outputKey string, value float64) {
again:
	v, ok := as.m.Load(outputKey)
	if !ok {
		// The entry is missing in the map. Try creating it.
		v = &stddevStateValue{}
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
		avg := sv.avg + (value-sv.avg)/sv.count
		sv.q += (value - sv.avg) * (value - avg)
		sv.avg = avg
	}
	sv.mu.Unlock()
	if deleted {
		// The entry has been deleted by the concurrent call to appendSeriesForFlush
		// Try obtaining and updating the entry again.
		goto again
	}
}

func (as *stddevAggrState) appendSeriesForFlush(ctx *flushCtx) {
	currentTimeMsec := int64(fasttime.UnixTimestamp()) * 1000
	m := &as.m
	m.Range(func(k, v interface{}) bool {
		// Atomically delete the entry from the map, so new entry is created for the next flush.
		m.Delete(k)

		sv := v.(*stddevStateValue)
		sv.mu.Lock()
		stddev := math.Sqrt(sv.q / sv.count)
		// Mark the entry as deleted, so it won't be updated anymore by concurrent pushSample() calls.
		sv.deleted = true
		sv.mu.Unlock()
		key := k.(string)
		ctx.appendSeries(key, "stddev", currentTimeMsec, stddev)
		return true
	})
}
