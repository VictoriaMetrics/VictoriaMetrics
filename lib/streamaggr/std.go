package streamaggr

import (
	"math"
)

// stdAggrValue calculates output=stdvar or stddev, e.g. the average value over input samples.
type stdAggrValue struct {
	count float64
	avg   float64
	q     float64
}

func (av *stdAggrValue) pushSample(_ aggrConfig, sample *pushSample, _ string, _ int64) {
	av.count++
	avg := av.avg + (sample.value-av.avg)/av.count
	av.q += (sample.value - av.avg) * (sample.value - avg)
	av.avg = avg
}

func (av *stdAggrValue) flush(c aggrConfig, ctx *flushCtx, key string, _ bool) {
	ac := c.(*stdAggrConfig)
	if av.count > 0 {
		suffix := ac.getSuffix()
		output := av.q / av.count
		if ac.isDeviation {
			output = math.Sqrt(output)
		}
		ctx.appendSeries(key, suffix, output)
		av.count = 0
		av.avg = 0
		av.q = 0
	}
}

func (ac *stdAggrConfig) getSuffix() string {
	if ac.isDeviation {
		return "stddev"
	}
	return "stdvar"
}

func (*stdAggrValue) state() any {
	return nil
}

func newStddevAggrConfig() aggrConfig {
	return &stdAggrConfig{
		isDeviation: true,
	}
}

func newStdvarAggrConfig() aggrConfig {
	return &stdAggrConfig{}
}

type stdAggrConfig struct {
	isDeviation bool
}

func (*stdAggrConfig) getValue(_ any) aggrValue {
	return &stdAggrValue{}
}
