package streamaggr

// stdvarAggrValue calculates output=stdvar, e.g. the average value over input samples.
type stdvarAggrValue struct {
	count float64
	avg   float64
	q     float64
}

func (av *stdvarAggrValue) pushSample(ctx *pushSampleCtx) {
	av.count++
	avg := av.avg + (ctx.sample.value-av.avg)/av.count
	av.q += (ctx.sample.value - av.avg) * (ctx.sample.value - avg)
	av.avg = avg
}

func (av *stdvarAggrValue) flush(ctx *flushCtx, key string) {
	if av.count > 0 {
		ctx.appendSeries(key, "stdvar", av.q/av.count)
		av.count = 0
		av.avg = 0
		av.q = 0
	}
}
