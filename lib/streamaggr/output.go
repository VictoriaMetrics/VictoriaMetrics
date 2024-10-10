package streamaggr

import (
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/metrics"
	"sync"
)

type pushSampleCtx struct {
	stateSize      int
	deleteDeadline int64
	sample         *pushSample
	idx            int
	inputKey       string
}

type aggrValuesFn func(*pushSampleCtx) []aggrValue

type aggrValuesInitFn func([]aggrValue) []aggrValue

func newAggrValues[V any, VP aggrValuePtr[V]](initFn aggrValuesInitFn) aggrValuesFn {
	return func(ctx *pushSampleCtx) []aggrValue {
		output := make([]aggrValue, ctx.stateSize)
		if initFn != nil {
			return initFn(output)
		}
		for i := range output {
			var v VP = new(V)
			output[i] = v
		}
		return output
	}
}

type aggrOutputs struct {
	m             sync.Map
	stateSize     int
	initFns       []aggrValuesFn
	outputSamples *metrics.Counter
}

func (ao *aggrOutputs) pushSamples(data *pushCtxData) {
	ctx := &pushSampleCtx{
		stateSize:      ao.stateSize,
		deleteDeadline: data.deleteDeadline,
		idx:            data.idx,
	}
	var outputKey string
	for i := range data.samples {
		ctx.sample = &data.samples[i]
		ctx.inputKey, outputKey = getInputOutputKey(ctx.sample.key)

	again:
		v, ok := ao.m.Load(outputKey)
		if !ok {
			// The entry is missing in the map. Try creating it.
			nv := &aggrValues{
				values: make([][]aggrValue, len(ao.initFns)),
			}
			for i, initFn := range ao.initFns {
				nv.values[i] = initFn(ctx)
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
			for i := range av.values {
				av.values[i][data.idx].pushSample(ctx)
			}
			av.deleteDeadline = data.deleteDeadline
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
		for _, ov := range av.values {
			ov[ctx.idx].flush(ctx, key)
		}
		av.mu.Unlock()
		return true
	})
}

type aggrValues struct {
	mu             sync.Mutex
	values         [][]aggrValue
	deleteDeadline int64
	deleted        bool
}

type aggrValue interface {
	pushSample(*pushSampleCtx)
	flush(*flushCtx, string)
}

type aggrValuePtr[V any] interface {
	*V
	aggrValue
}
