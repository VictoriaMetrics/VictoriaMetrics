package streamaggr

import (
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/metrics"
	"sync"
)

type aggrValuesFn func(*aggrValues, bool)

type aggrOutputs struct {
	m             sync.Map
	enableWindows bool
	initFns       []aggrValuesFn
	outputSamples *metrics.Counter
}

func (ao *aggrOutputs) pushSamples(samples []pushSample, deleteDeadline int64, isGreen bool) {
	var inputKey, outputKey string
	var sample *pushSample
	for i := range samples {
		sample = &samples[i]
		inputKey, outputKey = getInputOutputKey(sample.key)

	again:
		v, ok := ao.m.Load(outputKey)
		if !ok {
			// The entry is missing in the map. Try creating it.
			nv := &aggrValues{
				blue: make([]aggrValue, 0, len(ao.initFns)),
			}
			if ao.enableWindows {
				nv.green = make([]aggrValue, 0, len(ao.initFns))
			}
			for _, initFn := range ao.initFns {
				initFn(nv, ao.enableWindows)
			}
			v = nv
			outputKey = bytesutil.InternString(outputKey)
			vNew, loaded := ao.m.LoadOrStore(outputKey, v)
			if loaded {
				// Use the entry created by a concurrent goroutine.
				v = vNew
			}
		}
		av := v.(*aggrValues)
		av.mu.Lock()
		deleted := av.deleted
		if !deleted {
			if isGreen {
				for _, sv := range av.green {
					sv.pushSample(inputKey, sample, deleteDeadline)
				}
			} else {
				for _, sv := range av.blue {
					sv.pushSample(inputKey, sample, deleteDeadline)
				}
			}
			av.deleteDeadline = deleteDeadline
		}
		av.mu.Unlock()
		if deleted {
			// The entry has been deleted by the concurrent call to flush
			// Try obtaining and updating the entry again.
			goto again
		}
	}
}

func (ao *aggrOutputs) flushState(ctx *flushCtx) {
	m := &ao.m
	var outputs []aggrValue
	m.Range(func(k, v any) bool {
		// Atomically delete the entry from the map, so new entry is created for the next flush.
		av := v.(*aggrValues)
		av.mu.Lock()

		// check for stale entries
		deleted := ctx.flushTimestamp > av.deleteDeadline
		if deleted {
			// Mark the current entry as deleted
			av.deleted = deleted
			av.mu.Unlock()
			m.Delete(k)
			return true
		}
		key := k.(string)
		if ctx.isGreen {
			outputs = av.green
		} else {
			outputs = av.blue
		}
		for _, state := range outputs {
			state.flush(ctx, key)
		}
		av.mu.Unlock()
		return true
	})
}

type aggrValues struct {
	mu             sync.Mutex
	blue           []aggrValue
	green          []aggrValue
	deleteDeadline int64
	deleted        bool
}

type aggrValue interface {
	pushSample(string, *pushSample, int64)
	flush(*flushCtx, string)
}
