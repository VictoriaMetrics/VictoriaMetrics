package promql

import (
	"math"
	"strings"
	"sync"
)

// callbacks for optimized incremental calculations for aggregate functions
// over rollups over metricExpr.
//
// These calculations save RAM for aggregates over big number of time series.
var incrementalAggrFuncCallbacksMap = map[string]*incrementalAggrFuncCallbacks{
	"sum": {
		updateAggrFunc:   updateAggrSum,
		finalizeAggrFunc: finalizeAggrCommon,
	},
	"min": {
		updateAggrFunc:   updateAggrMin,
		finalizeAggrFunc: finalizeAggrCommon,
	},
	"max": {
		updateAggrFunc:   updateAggrMax,
		finalizeAggrFunc: finalizeAggrCommon,
	},
	"avg": {
		updateAggrFunc:   updateAggrAvg,
		finalizeAggrFunc: finalizeAggrAvg,
	},
	"count": {
		updateAggrFunc:   updateAggrCount,
		finalizeAggrFunc: finalizeAggrCount,
	},
	"sum2": {
		updateAggrFunc:   updateAggrSum2,
		finalizeAggrFunc: finalizeAggrCommon,
	},
	"geomean": {
		updateAggrFunc:   updateAggrGeomean,
		finalizeAggrFunc: finalizeAggrGeomean,
	},
}

type incrementalAggrFuncContext struct {
	ae *aggrFuncExpr

	mu sync.Mutex
	m  map[string]*incrementalAggrContext

	callbacks *incrementalAggrFuncCallbacks
}

func newIncrementalAggrFuncContext(ae *aggrFuncExpr, callbacks *incrementalAggrFuncCallbacks) *incrementalAggrFuncContext {
	return &incrementalAggrFuncContext{
		ae:        ae,
		m:         make(map[string]*incrementalAggrContext, 1),
		callbacks: callbacks,
	}
}

func (iafc *incrementalAggrFuncContext) updateTimeseries(ts *timeseries) {
	removeGroupTags(&ts.MetricName, &iafc.ae.Modifier)
	bb := bbPool.Get()
	bb.B = marshalMetricNameSorted(bb.B[:0], &ts.MetricName)
	iafc.mu.Lock()
	iac := iafc.m[string(bb.B)]
	if iac == nil {
		tsAggr := &timeseries{
			Values:     make([]float64, len(ts.Values)),
			Timestamps: ts.Timestamps,
			denyReuse:  true,
		}
		tsAggr.MetricName.CopyFrom(&ts.MetricName)
		iac = &incrementalAggrContext{
			ts:     tsAggr,
			values: make([]float64, len(ts.Values)),
		}
		iafc.m[string(bb.B)] = iac
	}
	iafc.callbacks.updateAggrFunc(iac, ts.Values)
	iafc.mu.Unlock()
	bbPool.Put(bb)
}

func (iafc *incrementalAggrFuncContext) finalizeTimeseries() []*timeseries {
	// There is no need in iafc.mu.Lock here, since getTimeseries must be called
	// without concurrent goroutines touching iafc.
	tss := make([]*timeseries, 0, len(iafc.m))
	finalizeAggrFunc := iafc.callbacks.finalizeAggrFunc
	for _, iac := range iafc.m {
		finalizeAggrFunc(iac)
		tss = append(tss, iac.ts)
	}
	return tss
}

type incrementalAggrFuncCallbacks struct {
	updateAggrFunc   func(iac *incrementalAggrContext, values []float64)
	finalizeAggrFunc func(iac *incrementalAggrContext)
}

func getIncrementalAggrFuncCallbacks(name string) *incrementalAggrFuncCallbacks {
	name = strings.ToLower(name)
	return incrementalAggrFuncCallbacksMap[name]
}

type incrementalAggrContext struct {
	ts     *timeseries
	values []float64
}

func finalizeAggrCommon(iac *incrementalAggrContext) {
	counts := iac.values
	dstValues := iac.ts.Values
	_ = dstValues[len(counts)-1]
	for i, v := range counts {
		if v == 0 {
			dstValues[i] = nan
		}
	}
}

func updateAggrSum(iac *incrementalAggrContext, values []float64) {
	dstValues := iac.ts.Values
	dstCounts := iac.values
	_ = dstValues[len(values)-1]
	_ = dstCounts[len(values)-1]
	for i, v := range values {
		if math.IsNaN(v) {
			continue
		}
		dstValues[i] += v
		dstCounts[i] = 1
	}
}

func updateAggrMin(iac *incrementalAggrContext, values []float64) {
	dstValues := iac.ts.Values
	dstCounts := iac.values
	_ = dstValues[len(values)-1]
	_ = dstCounts[len(values)-1]
	for i, v := range values {
		if math.IsNaN(v) {
			continue
		}
		if dstCounts[i] == 0 {
			dstValues[i] = v
			dstCounts[i] = 1
			continue
		}
		if v < dstValues[i] {
			dstValues[i] = v
		}
	}
}

func updateAggrMax(iac *incrementalAggrContext, values []float64) {
	dstValues := iac.ts.Values
	dstCounts := iac.values
	_ = dstValues[len(values)-1]
	_ = dstCounts[len(values)-1]
	for i, v := range values {
		if math.IsNaN(v) {
			continue
		}
		if dstCounts[i] == 0 {
			dstValues[i] = v
			dstCounts[i] = 1
			continue
		}
		if v > dstValues[i] {
			dstValues[i] = v
		}
	}
}

func updateAggrAvg(iac *incrementalAggrContext, values []float64) {
	// Do not use `Rapid calculation methods` at https://en.wikipedia.org/wiki/Standard_deviation,
	// since it is slower and has no obvious benefits in increased precision.
	dstValues := iac.ts.Values
	dstCounts := iac.values
	_ = dstValues[len(values)-1]
	_ = dstCounts[len(values)-1]
	for i, v := range values {
		if math.IsNaN(v) {
			continue
		}
		if dstCounts[i] == 0 {
			dstValues[i] = v
			dstCounts[i] = 1
			continue
		}
		dstValues[i] += v
		dstCounts[i]++
	}
}

func finalizeAggrAvg(iac *incrementalAggrContext) {
	dstValues := iac.ts.Values
	counts := iac.values
	_ = dstValues[len(counts)-1]
	for i, v := range counts {
		if v == 0 {
			dstValues[i] = nan
			continue
		}
		dstValues[i] /= v
	}
}

func updateAggrCount(iac *incrementalAggrContext, values []float64) {
	dstValues := iac.ts.Values
	_ = dstValues[len(values)-1]
	for i, v := range values {
		if math.IsNaN(v) {
			continue
		}
		dstValues[i]++
	}
}

func finalizeAggrCount(iac *incrementalAggrContext) {
	// Nothing to do
}

func updateAggrSum2(iac *incrementalAggrContext, values []float64) {
	dstValues := iac.ts.Values
	dstCounts := iac.values
	_ = dstValues[len(values)-1]
	_ = dstCounts[len(values)-1]
	for i, v := range values {
		if math.IsNaN(v) {
			continue
		}
		dstValues[i] += v * v
		dstCounts[i] = 1
	}
}

func updateAggrGeomean(iac *incrementalAggrContext, values []float64) {
	dstValues := iac.ts.Values
	dstCounts := iac.values
	_ = dstValues[len(values)-1]
	_ = dstCounts[len(values)-1]
	for i, v := range values {
		if math.IsNaN(v) {
			continue
		}
		if dstCounts[i] == 0 {
			dstValues[i] = v
			dstCounts[i] = 1
			continue
		}
		dstValues[i] *= v
		dstCounts[i]++
	}
}

func finalizeAggrGeomean(iac *incrementalAggrContext) {
	dstValues := iac.ts.Values
	counts := iac.values
	_ = dstValues[len(counts)-1]
	for i, v := range counts {
		if v == 0 {
			dstValues[i] = nan
			continue
		}
		dstValues[i] = math.Pow(dstValues[i], 1/v)
	}
}
