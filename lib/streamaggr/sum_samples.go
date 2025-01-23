package streamaggr

func sumSamplesInitFn(v *aggrValues, enableWindows bool) {
	v.blue = append(v.blue, new(sumSamplesAggrValue))
	if enableWindows {
		v.green = append(v.green, new(sumSamplesAggrValue))
	}
}

type sumSamplesAggrValue struct {
	sum float64
}

func (av *sumSamplesAggrValue) pushSample(_ string, sample *pushSample, _ int64) {
	av.sum += sample.value
}

func (av *sumSamplesAggrValue) flush(ctx *flushCtx, key string) {
	ctx.appendSeries(key, "sum_samples", av.sum)
	av.sum = 0
}
