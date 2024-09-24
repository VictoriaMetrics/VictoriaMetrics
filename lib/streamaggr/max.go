package streamaggr

type maxAggrValue struct {
	max     float64
	defined bool
}

func (av *maxAggrValue) pushSample(ctx *pushSampleCtx) {
	if ctx.sample.value > av.max || !av.defined {
		av.max = ctx.sample.value
	}
	if !av.defined {
		av.defined = true
	}
}

func (av *maxAggrValue) flush(ctx *flushCtx, key string) {
	if av.defined {
		ctx.appendSeries(key, "max", av.max)
		av.max = 0
		av.defined = false
	}
}
