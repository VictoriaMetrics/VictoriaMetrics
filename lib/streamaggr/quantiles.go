package streamaggr

import (
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/valyala/histogram"
	"strconv"
)

func quantilesInitFn(phis []float64) aggrValuesFn {
	blue := &quantilesAggrState{
		phis: phis,
	}
	green := &quantilesAggrState{
		phis: phis,
	}
	return func(v *aggrValues, enableWindows bool) {
		v.blue = append(v.blue, &quantilesAggrValue{
			state: blue,
		})
		if enableWindows {
			v.green = append(v.green, &quantilesAggrValue{
				state: green,
			})
		}
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

func (av *quantilesAggrValue) pushSample(_ string, sample *pushSample, _ int64) {
	if av.h == nil {
		av.h = histogram.GetFast()
	}
	av.h.Update(sample.value)
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
