package streamaggr

type lastAggrValue struct {
	last      float64
	timestamp int64
}

func (av *lastAggrValue) pushSample(ctx *pushSampleCtx) {
	if ctx.sample.timestamp >= av.timestamp {
		av.last = ctx.sample.value
		av.timestamp = ctx.sample.timestamp
	}
}

func (av *lastAggrValue) flush(ctx *flushCtx, key string) {
	if av.timestamp > 0 {
		ctx.appendSeries(key, "last", av.last)
		av.timestamp = 0
	}
}
