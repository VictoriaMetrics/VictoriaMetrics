package streamaggr

import (
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/cespare/xxhash/v2"
)

func countSeriesInitFn(values []aggrValue) []aggrValue {
	for i := range values {
		values[i] = &countSeriesAggrValue{
			samples: make(map[uint64]struct{}),
		}
	}
	return values
}

type countSeriesAggrValue struct {
	samples map[uint64]struct{}
}

func (av *countSeriesAggrValue) pushSample(ctx *pushSampleCtx) {
	// Count unique hashes over the inputKeys instead of unique inputKey values.
	// This reduces memory usage at the cost of possible hash collisions for distinct inputKey values.
	h := xxhash.Sum64(bytesutil.ToUnsafeBytes(ctx.inputKey))
	if _, ok := av.samples[h]; !ok {
		av.samples[h] = struct{}{}
	}
}

func (av *countSeriesAggrValue) flush(ctx *flushCtx, key string) {
	ctx.appendSeries(key, "count_series", float64(len(av.samples)))
	clear(av.samples)
}
