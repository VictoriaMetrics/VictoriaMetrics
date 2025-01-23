package streamaggr

func uniqueSamplesInitFn(v *aggrValues, enableWindows bool) {
	v.blue = append(v.blue, &uniqueSamplesAggrValue{
		samples: make(map[float64]struct{}),
	})
	if enableWindows {
		v.green = append(v.green, &uniqueSamplesAggrValue{
			samples: make(map[float64]struct{}),
		})
	}
}

type uniqueSamplesAggrValue struct {
	samples map[float64]struct{}
}

func (av *uniqueSamplesAggrValue) pushSample(_ string, sample *pushSample, _ int64) {
	if _, ok := av.samples[sample.value]; !ok {
		av.samples[sample.value] = struct{}{}
	}
}

func (av *uniqueSamplesAggrValue) flush(ctx *flushCtx, key string) {
	ctx.appendSeries(key, "unique_samples", float64(len(av.samples)))
	clear(av.samples)
}
