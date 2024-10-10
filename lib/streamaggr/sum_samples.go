package streamaggr

type sumSamplesAggrValue struct {
	sum float64
}

func (av *sumSamplesAggrValue) pushSample(ctx *pushSampleCtx) {
	av.sum += ctx.sample.value
}

func (av *sumSamplesAggrValue) flush(ctx *flushCtx, key string) {
	ctx.appendSeries(key, "sum_samples", av.sum)
	av.sum = 0
}
