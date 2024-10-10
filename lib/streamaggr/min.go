package streamaggr

type minAggrValue struct {
	min     float64
	defined bool
}

func (av *minAggrValue) pushSample(ctx *pushSampleCtx) {
	if ctx.sample.value < av.min || !av.defined {
		av.min = ctx.sample.value
	}
	if !av.defined {
		av.defined = true
	}
}

func (av *minAggrValue) flush(ctx *flushCtx, key string) {
	if av.defined {
		ctx.appendSeries(key, "min", av.min)
		av.defined = false
		av.min = 0
	}
}
