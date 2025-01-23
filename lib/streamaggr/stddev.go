package streamaggr

import (
	"math"
)

func stddevInitFn(v *aggrValues, enableWindows bool) {
	v.blue = append(v.blue, new(stddevAggrValue))
	if enableWindows {
		v.green = append(v.green, new(stddevAggrValue))
	}
}

// stddevAggrValue calculates output=stddev, e.g. the average value over input samples.
type stddevAggrValue struct {
	count float64
	avg   float64
	q     float64
}

func (av *stddevAggrValue) pushSample(_ string, sample *pushSample, _ int64) {
	av.count++
	avg := av.avg + (sample.value-av.avg)/av.count
	av.q += (sample.value - av.avg) * (sample.value - avg)
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
