package streamaggr

func countSamplesInitFn(v *aggrValues, enableWindows bool) {
	v.blue = append(v.blue, new(countSamplesAggrValue))
	if enableWindows {
		v.green = append(v.green, new(countSamplesAggrValue))
	}
}

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
