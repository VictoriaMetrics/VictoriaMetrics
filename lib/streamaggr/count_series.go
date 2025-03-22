package streamaggr

import (
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/cespare/xxhash/v2"
)

type countSeriesAggrValue struct {
	samples map[uint64]struct{}
}

func (av *countSeriesAggrValue) pushSample(_ aggrConfig, _ *pushSample, key string, _ int64) {
	// Count unique hashes over the keys instead of unique key values.
	// This reduces memory usage at the cost of possible hash collisions for distinct key values.
	h := xxhash.Sum64(bytesutil.ToUnsafeBytes(key))
	if _, ok := av.samples[h]; !ok {
		av.samples[h] = struct{}{}
	}
}

func (av *countSeriesAggrValue) flush(_ aggrConfig, ctx *flushCtx, key string, _ bool) {
	if len(av.samples) > 0 {
		ctx.appendSeries(key, "count_series", float64(len(av.samples)))
		clear(av.samples)
	}
}

func (*countSeriesAggrValue) state() any {
	return nil
}

func newCountSeriesAggrConfig() aggrConfig {
	return &countSeriesAggrConfig{}
}

type countSeriesAggrConfig struct{}

func (*countSeriesAggrConfig) getValue(_ any) aggrValue {
	return &countSeriesAggrValue{
		samples: make(map[uint64]struct{}),
	}
}
