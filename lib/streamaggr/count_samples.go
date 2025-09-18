package streamaggr

type countSamplesAggrValue struct {
	count uint64
}

func (av *countSamplesAggrValue) pushSample(_ aggrConfig, _ *pushSample, _ string, _ int64) {
	av.count++
}

func (av *countSamplesAggrValue) flush(_ aggrConfig, ctx *flushCtx, key string, _ bool) {
	if av.count > 0 {
		ctx.appendSeries(key, "count_samples", float64(av.count))
		av.count = 0
	}
}

func (*countSamplesAggrValue) state() any {
	return nil
}

func newCountSamplesAggrConfig() aggrConfig {
	return &countSamplesAggrConfig{}
}

type countSamplesAggrConfig struct{}

func (*countSamplesAggrConfig) getValue(_ any) aggrValue {
	return &countSamplesAggrValue{}
}
