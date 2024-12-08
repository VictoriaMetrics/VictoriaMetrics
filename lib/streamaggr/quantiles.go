package streamaggr

import (
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/valyala/histogram"
	"strconv"
)

func quantilesInitFn(stateSize int, phis []float64) aggrValuesInitFn {
	states := make([]*quantilesAggrState, stateSize)
	return func(values []aggrValue) []aggrValue {
		for i := range values {
			state := states[i]
			if state == nil {
				state = &quantilesAggrState{
					phis: phis,
				}
				states[i] = state
			}
			values[i] = &quantilesAggrValue{
				state: state,
			}
		}
		return values
	}
}

type quantilesAggrState struct {
	phis      []float64
	quantiles []float64
	b         []byte
}

// quantilesAggrValue calculates output=quantiles, e.g. the given quantiles over the input samples.
type quantilesAggrValue struct {
	h     *histogram.Fast
	state *quantilesAggrState
}

func (av *quantilesAggrValue) pushSample(ctx *pushSampleCtx) {
	if av.h == nil {
		av.h = histogram.GetFast()
	}
	av.h.Update(ctx.sample.value)
}

func (av *quantilesAggrValue) flush(ctx *flushCtx, key string) {
	if av.h != nil {
		av.state.quantiles = av.h.Quantiles(av.state.quantiles[:0], av.state.phis)
	}
	histogram.PutFast(av.h)
	if len(av.state.quantiles) > 0 {
		for i, quantile := range av.state.quantiles {
			av.state.b = strconv.AppendFloat(av.state.b[:0], av.state.phis[i], 'g', -1, 64)
			phiStr := bytesutil.InternBytes(av.state.b)
			ctx.appendSeriesWithExtraLabel(key, "quantiles", quantile, "quantile", phiStr)
		}
	}
}
