package streamaggr

import (
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bloomfilter"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/bytesutil"
	"github.com/cespare/xxhash/v2"
	"sync"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
)

// countSeriesBloomfilterAggrState calculates output=count_series, e.g. the number of unique series.
type countSeriesBloomfilterAggrState struct {
	m         sync.Map
	maxSeries int
}

type countSeriesBloomfilterStateValue struct {
	mu          sync.Mutex
	bloomfilter *bloomfilter.Filter
	n           uint64
	deleted     bool
}

func newcountSeriesBloomfilterAggrState(maxSeries int) *countSeriesBloomfilterAggrState {
	return &countSeriesBloomfilterAggrState{maxSeries: maxSeries}
}

func (as *countSeriesBloomfilterAggrState) pushSample(inputKey, outputKey string, _ float64) {
again:
	v, ok := as.m.Load(outputKey)
	if !ok {
		// The entry is missing in the map. Try creating it.
		filter := bloomfilter.NewFilter(as.maxSeries)
		filter.Add(xxhash.Sum64(bytesutil.ToUnsafeBytes(inputKey)))
		v = &countSeriesBloomfilterStateValue{
			bloomfilter: filter,
			n:           1,
		}
		vNew, loaded := as.m.LoadOrStore(outputKey, v)
		if !loaded {
			// The entry has been added to the map.
			return
		}
		// Update the entry created by a concurrent goroutine.
		v = vNew
	}
	sv := v.(*countSeriesBloomfilterStateValue)
	sv.mu.Lock()
	deleted := sv.deleted
	if !deleted {
		h := xxhash.Sum64(bytesutil.ToUnsafeBytes(inputKey))
		if sv.bloomfilter.Add(h) {
			sv.n++
		}
	}
	sv.mu.Unlock()
	if deleted {
		// The entry has been deleted by the concurrent call to appendSeriesForFlush
		// Try obtaining and updating the entry again.
		goto again
	}
}

func (as *countSeriesBloomfilterAggrState) appendSeriesForFlush(ctx *flushCtx) {
	currentTimeMsec := int64(fasttime.UnixTimestamp()) * 1000
	m := &as.m
	m.Range(func(k, v interface{}) bool {
		// Atomically delete the entry from the map, so new entry is created for the next flush.
		m.Delete(k)

		sv := v.(*countSeriesBloomfilterStateValue)
		sv.mu.Lock()
		n := sv.n
		// Mark the entry as deleted, so it won't be updated anymore by concurrent pushSample() calls.
		sv.deleted = true
		sv.mu.Unlock()
		key := k.(string)
		ctx.appendSeries(key, "count_series_bloomfilter", currentTimeMsec, float64(n))
		return true
	})
}
