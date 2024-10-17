package streamaggr

type avgAggrValue struct {
	sum   float64
	count float64
}

func (sv *avgAggrValue) pushSample(ctx *pushSampleCtx) {
	sv.sum += ctx.sample.value
	sv.count++
}

func (sv *avgAggrValue) flush(ctx *flushCtx, key string) {
	if sv.count > 0 {
		avg := sv.sum / sv.count
		ctx.appendSeries(key, "avg", avg)
		sv.sum = 0
		sv.count = 0
	}
}
