package streamaggr

import (
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/encoding"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/metrics"
)

type aggrOutputs struct {
	m              sync.Map
	useSharedState bool
	useInputKey    bool
	configs        []aggrConfig
	outputSamples  *metrics.Counter
}

func (ao *aggrOutputs) getInputOutputKey(key string) (string, string) {
	src := bytesutil.ToUnsafeBytes(key)
	outputKeyLen, nSize := encoding.UnmarshalVarUint64(src)
	if nSize <= 0 {
		logger.Panicf("BUG: cannot unmarshal outputKeyLen from uvarint")
	}
	src = src[nSize:]
	outputKey := src[:outputKeyLen]
	if !ao.useInputKey {
		return key, bytesutil.ToUnsafeString(outputKey)
	}
	inputKey := src[outputKeyLen:]
	return bytesutil.ToUnsafeString(inputKey), bytesutil.ToUnsafeString(outputKey)
}

func (ao *aggrOutputs) pushSamples(samples []pushSample, deleteDeadline int64, isGreen bool) {
	var inputKey, outputKey string
	var sample *pushSample
	var outputs []aggrValue
	var nv *aggrValues
	for i := range samples {
		sample = &samples[i]
		inputKey, outputKey = ao.getInputOutputKey(sample.key)

	again:
		v, ok := ao.m.Load(outputKey)
		if !ok {
			// The entry is missing in the map. Try creating it.
			nv = &aggrValues{
				blue: make([]aggrValue, len(ao.configs)),
			}
			if ao.useSharedState {
				nv.green = make([]aggrValue, len(ao.configs))
			}
			for idx, ac := range ao.configs {
				nv.blue[idx] = ac.getValue(nil)
				if ao.useSharedState {
					nv.green[idx] = ac.getValue(nv.blue[idx].state())
				}
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
		deleted := av.deleteDeadline < 0
		if !deleted {
			if isGreen {
				outputs = av.green
			} else {
				outputs = av.blue
			}
			for idx, o := range outputs {
				if o == nil {
					o = av.blue[idx]
				}
				o.pushSample(ao.configs[idx], sample, inputKey, deleteDeadline)
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

// stateSizeBytes returns the approximate total size in bytes of the state kept by the output config at configIdx
// across all the currently tracked series.
func (ao *aggrOutputs) stateSizeBytes(configIdx int) uint64 {
	var n uint64
	ao.m.Range(func(_, v any) bool {
		av := v.(*aggrValues)
		av.mu.Lock()
		if av.deleteDeadline >= 0 {
			if o := av.blue[configIdx]; o != nil {
				n += o.sizeBytes()
			}
			if ao.useSharedState {
				if o := av.green[configIdx]; o != nil {
					n += o.sizeBytes()
				}
			}
		}
		av.mu.Unlock()
		return true
	})
	return n
}

// stateItemsCount returns the number of series currently tracked in the output state.
//
// This number is the same for every output config, since all the configs of a single aggregation
// share the same set of series keys.
func (ao *aggrOutputs) stateItemsCount() uint64 {
	var n uint64
	ao.m.Range(func(_, v any) bool {
		av := v.(*aggrValues)
		av.mu.Lock()
		if av.deleteDeadline >= 0 {
			n++
		}
		av.mu.Unlock()
		return true
	})
	return n
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
			av.deleteDeadline = -1
			av.mu.Unlock()
			m.Delete(k)
			return true
		}
		outputKey := k.(string)
		if ctx.isGreen {
			outputs = av.green
		} else {
			outputs = av.blue
		}
		for i, o := range outputs {
			if o == nil {
				o = av.blue[i]
			}
			o.flush(ao.configs[i], ctx, outputKey, ctx.isLast)
		}
		av.mu.Unlock()
		if ctx.isLast {
			m.Delete(k)
		}
		return true
	})
}

type aggrValues struct {
	mu             sync.Mutex
	blue           []aggrValue
	green          []aggrValue
	deleteDeadline int64
}

type aggrConfig interface {
	getValue(any) aggrValue
}

type aggrValue interface {
	pushSample(aggrConfig, *pushSample, string, int64)
	flush(aggrConfig, *flushCtx, string, bool)
	state() any

	// sizeBytes returns the approximate memory size occupied by the value's state.
	sizeBytes() uint64
}

// mapEntryOverheadBytes is the approximate per-entry overhead of a Go map, used for estimating
// sizeBytes() of aggrValue implementations, which keep their state in a map.
const mapEntryOverheadBytes = 24
