package streamaggr

type sumLastValue struct {
	value          float64
	timestamp      int64
	deleteDeadline int64
}

type sumLastAggrValue struct {
	lastValues map[string]sumLastValue
}

func (s *sumLastAggrValue) pushSample(_ aggrConfig, sample *pushSample, key string, deleteDeadline int64) {
	lv := s.lastValues[key]
	if sample.timestamp < lv.timestamp {
		return
	}
	lv.value = sample.value
	lv.timestamp = sample.timestamp
	lv.deleteDeadline = deleteDeadline
	s.lastValues[key] = lv
}

func (s *sumLastAggrValue) flush(_ aggrConfig, ctx *flushCtx, key string, _ bool) {
	if len(s.lastValues) == 0 {
		return
	}

	var (
		sum   float64
		isAny bool
	)
	for k, lv := range s.lastValues {
		if ctx.flushTimestamp > lv.deleteDeadline {
			delete(s.lastValues, k)
			continue
		}
		if lv.timestamp <= 0 {
			continue
		}
		sum += lv.value
		isAny = true
		lv.timestamp = 0
		s.lastValues[k] = lv
	}
	if isAny {
		ctx.appendSeries(key, "sum_last", sum)
	}
}

func (s *sumLastAggrValue) state() any {
	return nil
}

type sumLastConfig struct{}

func newSumLastAggrConfig() aggrConfig {
	return &sumLastConfig{}
}

func (*sumLastConfig) getValue(s any) aggrValue {
	return &sumLastAggrValue{
		lastValues: make(map[string]sumLastValue),
	}
}
