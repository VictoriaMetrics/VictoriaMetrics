package streamaggr

func lastInitFn(v *aggrValues, enableWindows bool) {
	v.blue = append(v.blue, new(lastAggrValue))
	if enableWindows {
		v.green = append(v.green, new(lastAggrValue))
	}
}

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
