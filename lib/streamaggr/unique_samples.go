package streamaggr

func uniqueSamplesInitFn(values []aggrValue) []aggrValue {
	for i := range values {
		values[i] = &uniqueSamplesAggrValue{
			samples: make(map[float64]struct{}),
		}
	}
	return values
}

type uniqueSamplesAggrValue struct {
	samples map[float64]struct{}
}

func (av *uniqueSamplesAggrValue) pushSample(ctx *pushSampleCtx) {
	if _, ok := av.samples[ctx.sample.value]; !ok {
		av.samples[ctx.sample.value] = struct{}{}
	}
}

func (av *uniqueSamplesAggrValue) flush(ctx *flushCtx, key string) {
	ctx.appendSeries(key, "unique_samples", float64(len(av.samples)))
	clear(av.samples)
}
