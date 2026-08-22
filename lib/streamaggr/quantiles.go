package streamaggr

import (
	"strconv"
	"unsafe"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/valyala/histogram"
)

// maxFastHistogramSamples is the maximum number of samples histogram.Fast keeps internally
// (see maxSamples in github.com/valyala/histogram). It isn't exported, so sizeBytes() below
// uses it only as an upper-bound approximation of the histogram.Fast memory usage, since the
// actual number of currently buffered samples isn't observable from outside that package.
const maxFastHistogramSamples = 1000

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
	if av.h == nil {
		return
	}
	ac := c.(*quantilesAggrConfig)
	ac.quantiles = av.h.Quantiles(ac.quantiles[:0], ac.phis)
	histogram.PutFast(av.h)
	// reset h to avoid producing stale results on the next flush if av didn't get new pushSample() calls
	av.h = nil

	for i, quantile := range ac.quantiles {
		ac.b = strconv.AppendFloat(ac.b[:0], ac.phis[i], 'g', -1, 64)
		phiStr := bytesutil.InternBytes(ac.b)
		ctx.appendSeriesWithExtraLabel(key, "quantiles", quantile, "quantile", phiStr)
	}
}

func (av *quantilesAggrValue) state() any {
	return nil
}

func (av *quantilesAggrValue) sizeBytes() uint64 {
	n := uint64(unsafe.Sizeof(*av))
	if av.h != nil {
		n += uint64(unsafe.Sizeof(*av.h)) + 2*maxFastHistogramSamples*uint64(unsafe.Sizeof(float64(0)))
	}
	return n
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
