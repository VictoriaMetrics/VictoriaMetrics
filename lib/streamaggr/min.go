package streamaggr

func minInitFn(v *aggrValues, enableWindows bool) {
	v.blue = append(v.blue, new(minAggrValue))
	if enableWindows {
		v.green = append(v.green, new(minAggrValue))

	}
}

type minAggrValue struct {
	min     float64
	defined bool
}

func (av *minAggrValue) pushSample(_ string, sample *pushSample, _ int64) {
	if sample.value < av.min || !av.defined {
		av.min = sample.value
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
