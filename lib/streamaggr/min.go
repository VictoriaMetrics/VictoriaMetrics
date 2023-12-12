package streamaggr

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
)

// minAggrState calculates output=min, e.g. the minimum value over input samples.
type minAggrState struct {
	m                 sync.Map
	intervalSecs      uint64
	lastPushTimestamp atomic.Uint64
}

type minStateValue struct {
	mu           sync.Mutex
	min          float64
	samplesCount uint64
	deleted      bool
}

func newMinAggrState(interval time.Duration) *minAggrState {
	return &minAggrState{
		intervalSecs: roundDurationToSecs(interval),
	}
}

func (as *minAggrState) pushSample(_, outputKey string, value float64) {
again:
	v, ok := as.m.Load(outputKey)
	if !ok {
		// The entry is missing in the map. Try creating it.
		v = &minStateValue{
			min: value,
		}
		vNew, loaded := as.m.LoadOrStore(outputKey, v)
		if !loaded {
			// The new entry has been successfully created.
			return
		}
		// Use the entry created by a concurrent goroutine.
		v = vNew
	}
	sv := v.(*minStateValue)
	sv.mu.Lock()
	deleted := sv.deleted
	if !deleted {
		if value < sv.min {
			sv.min = value
			sv.samplesCount++
		}
	}
	sv.mu.Unlock()
	if deleted {
		// The entry has been deleted by the concurrent call to appendSeriesForFlush
		// Try obtaining and updating the entry again.
		goto again
	}
}

func (as *minAggrState) appendSeriesForFlush(ctx *flushCtx) {
	currentTime := fasttime.UnixTimestamp()
	currentTimeMsec := int64(currentTime) * 1000

	m := &as.m
	m.Range(func(k, v interface{}) bool {
		// Atomically delete the entry from the map, so new entry is created for the next flush.
		m.Delete(k)

		sv := v.(*minStateValue)
		sv.mu.Lock()
		value := sv.min
		// Mark the entry as deleted, so it won't be updated anymore by concurrent pushSample() calls.
		sv.deleted = true
		sv.mu.Unlock()
		key := k.(string)
		ctx.appendSeries(key, as.getOutputName(), currentTimeMsec, value)
		return true
	})
	as.lastPushTimestamp.Store(currentTime)
}

func (as *minAggrState) getOutputName() string {
	return "min"
}

func (as *minAggrState) getStateRepresentation(suffix string) aggrStateRepresentation {
	metrics := make([]aggrStateRepresentationMetric, 0)
	as.m.Range(func(k, v any) bool {
		value := v.(*minStateValue)
		value.mu.Lock()
		defer value.mu.Unlock()
		if value.deleted {
			return true
		}
		metrics = append(metrics, aggrStateRepresentationMetric{
			metric:       getLabelsStringFromKey(k.(string), suffix, as.getOutputName()),
			currentValue: value.min,
			samplesCount: value.samplesCount,
		})
		return true
	})
	return aggrStateRepresentation{
		intervalSecs:      as.intervalSecs,
		lastPushTimestamp: as.lastPushTimestamp.Load(),
		metrics:           metrics,
	}
}
