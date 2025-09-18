package streamaggr

type minAggrValue struct {
	min     float64
	defined bool
}

func (av *minAggrValue) pushSample(_ aggrConfig, sample *pushSample, _ string, _ int64) {
	if sample.value < av.min || !av.defined {
		av.min = sample.value
	}
	if !av.defined {
		av.defined = true
	}
}

func (av *minAggrValue) flush(_ aggrConfig, ctx *flushCtx, key string, _ bool) {
	if av.defined {
		ctx.appendSeries(key, "min", av.min)
		av.min = 0
		av.defined = false
	}
}

func (*minAggrValue) state() any {
	return nil
}

func newMinAggrConfig() aggrConfig {
	return &minAggrConfig{}
}

type minAggrConfig struct{}

func (*minAggrConfig) getValue(_ any) aggrValue {
	return &minAggrValue{}
}
