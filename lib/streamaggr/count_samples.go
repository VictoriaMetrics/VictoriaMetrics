package streamaggr

type countSamplesAggrValue struct {
	count uint64
}

func (av *countSamplesAggrValue) pushSample(_ *pushSampleCtx) {
	av.count++
}

func (av *countSamplesAggrValue) flush(ctx *flushCtx, key string) {
	ctx.appendSeries(key, "count_samples", float64(av.count))
	av.count = 0
}
