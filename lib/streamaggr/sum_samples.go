package streamaggr

type sumSamplesAggrValue struct {
	sum float64
}

func (av *sumSamplesAggrValue) pushSample(_ aggrConfig, sample *pushSample, _ string, _ int64) {
	if sample.stateOnly {
		return
	}
	av.sum += sample.value
}

func (av *sumSamplesAggrValue) flush(_ aggrConfig, ctx *flushCtx, key string, _ bool) {
	ctx.appendSeries(key, "sum_samples", av.sum)
	av.sum = 0
}

func (*sumSamplesAggrValue) state() any {
	return nil
}

func newSumSamplesAggrConfig() aggrConfig {
	return &sumSamplesAggrConfig{}
}

type sumSamplesAggrConfig struct{}

func (*sumSamplesAggrConfig) getValue(_ any) aggrValue {
	return &sumSamplesAggrValue{}
}
