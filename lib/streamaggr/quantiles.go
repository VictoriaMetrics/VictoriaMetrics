package streamaggr

import (
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/valyala/histogram"
	"strconv"
)

// quantilesAggrValue calculates output=quantiles, e.g. the given quantiles over the input samples.
type quantilesAggrValue struct {
	h *histogram.Fast
}

func (av *quantilesAggrValue) pushSample(_ aggrConfig, sample *pushSample, _ string, _ int64) {
	if av.h == nil {
		av.h = histogram.GetFast()
	}
	av.h.Update(sample.value)
}

func (av *quantilesAggrValue) flush(c aggrConfig, ctx *flushCtx, key string, _ bool) {
	ac := c.(*quantilesAggrConfig)
	if av.h != nil {
		ac.quantiles = av.h.Quantiles(ac.quantiles[:0], ac.phis)
	}
	histogram.PutFast(av.h)
	if len(ac.quantiles) > 0 {
		for i, quantile := range ac.quantiles {
			ac.b = strconv.AppendFloat(ac.b[:0], ac.phis[i], 'g', -1, 64)
			phiStr := bytesutil.InternBytes(ac.b)
			ctx.appendSeriesWithExtraLabel(key, "quantiles", quantile, "quantile", phiStr)
		}
	}
}

func (*quantilesAggrValue) state() any {
	return nil
}

func newQuantilesAggrConfig(phis []float64) aggrConfig {
	return &quantilesAggrConfig{
		phis: phis,
	}
}

type quantilesAggrConfig struct {
	phis      []float64
	quantiles []float64
	b         []byte
}

func (*quantilesAggrConfig) getValue(_ any) aggrValue {
	return &quantilesAggrValue{}
}
