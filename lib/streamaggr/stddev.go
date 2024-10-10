package streamaggr

import (
	"math"
)

// stddevAggrValue calculates output=stddev, e.g. the average value over input samples.
type stddevAggrValue struct {
	count float64
	avg   float64
	q     float64
}

func (av *stddevAggrValue) pushSample(ctx *pushSampleCtx) {
	av.count++
	avg := av.avg + (ctx.sample.value-av.avg)/av.count
	av.q += (ctx.sample.value - av.avg) * (ctx.sample.value - avg)
	av.avg = avg
}

func (av *stddevAggrValue) flush(ctx *flushCtx, key string) {
	if av.count > 0 {
		ctx.appendSeries(key, "stddev", math.Sqrt(av.q/av.count))
		av.count = 0
		av.avg = 0
		av.q = 0
	}
}
