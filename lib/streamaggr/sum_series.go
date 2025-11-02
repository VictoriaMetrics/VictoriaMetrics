package streamaggr

import (
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/cespare/xxhash/v2"
)

type sumSeriesAggrValue struct {
	samples map[uint64]float64
}

func (av *sumSeriesAggrValue) pushSample(_ aggrConfig, sample *pushSample, key string, _ int64) {
	// Store the last value for unique hashes over the keys instead of unique key values.
	// This reduces memory usage at the cost of possible hash collisions for distinct key values.
	h := xxhash.Sum64(bytesutil.ToUnsafeBytes(key))
	av.samples[h] = sample.value
}

func (av *sumSeriesAggrValue) flush(_ aggrConfig, ctx *flushCtx, key string, _ bool) {
	if len(av.samples) > 0 {
		sum := float64(0)
		for _, v := range av.samples {
			sum += v
		}
		ctx.appendSeries(key, "sum_series", sum)
		clear(av.samples)
	}
}

func (*sumSeriesAggrValue) state() any {
	return nil
}

func newSumSeriesAggrConfig() aggrConfig {
	return &sumSeriesAggrConfig{}
}

type sumSeriesAggrConfig struct{}

func (*sumSeriesAggrConfig) getValue(_ any) aggrValue {
	return &sumSeriesAggrValue{
		samples: make(map[uint64]float64),
	}
}
