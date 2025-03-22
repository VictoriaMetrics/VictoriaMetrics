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
}
