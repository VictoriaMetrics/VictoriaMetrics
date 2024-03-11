package streamaggr

import (
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/logger"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutils"
	"sync"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/prometheus/prometheus/model/histogram"
)

const (
	HistogramMerge = "histogram_merge"
)

// histogramMergeAggrState calculates output=merged histograms
type histogramMergeAggrState struct {
	m      sync.Map
	suffix string
	// Whether to take into account the first sample in new time series when calculating the output value.
	keepFirstSample bool
	// Time series state is dropped if no new samples are received during stalenessSecs.
	//
	// Aslo, the first sample per each new series is ignored during stalenessSecs even if keepFirstSample is set.
	// see ignoreFirstSampleDeadline for more details.
	stalenessSecs uint64

	// The first sample per each new series is ignored until this unix timestamp deadline in seconds even if keepFirstSample is set.
	// This allows avoiding an initial spike of the output values at startup when new time series
	// cannot be distinguished from already existing series. This is tracked with ignoreFirstSampleDeadline.
	ignoreFirstSampleDeadline uint64

	// Ignore samples which were older than the last seen sample. This is preferable to treating it as a
	// counter reset.
	ignoreOutOfOrderSamples bool

	lc *promutils.LabelsCompressor
}

type histogramMergeStateValue struct {
	mu             sync.Mutex
	lastValues     map[string]lastHistogramState
	merged         histogram.FloatHistogram
	deleteDeadline uint64
	deleted        bool
}

type lastHistogramState struct {
	value          *histogram.FloatHistogram
	timestamp      int64
	deleteDeadline uint64
}

func newHistogramMergeAggrState(stalenessInterval time.Duration, keepFirstSample bool, ignoreOutOfOrderSamples bool) *histogramMergeAggrState {
	stalenessSecs := roundDurationToSecs(stalenessInterval)
	ignoreFirstSampleDeadline := fasttime.UnixTimestamp() + stalenessSecs
	return &histogramMergeAggrState{
		ignoreFirstSampleDeadline: ignoreFirstSampleDeadline,
		ignoreOutOfOrderSamples:   ignoreOutOfOrderSamples,
		keepFirstSample:           keepFirstSample,
		stalenessSecs:             stalenessSecs,
		suffix:                    HistogramMerge,
	}
}

func (as *histogramMergeAggrState) setLc(labelsCompressor *promutils.LabelsCompressor) {
	as.lc = labelsCompressor
}

func (as *histogramMergeAggrState) pushHistograms(histograms []pushHistogram) {
	currentTime := fasttime.UnixTimestamp()
	deleteDeadline := currentTime + as.stalenessSecs
	for i := range histograms {
		h := &histograms[i]
		fh := h.value.ToFloatHistogram()
		if err := fh.Validate(); err != nil {
			logger.Errorf("Skipping invalid input histogram (%s) %v", h.key, err)
			continue
		}
		inputKey, outputKey := getInputOutputKey(h.key)

	again:
		v, ok := as.m.Load(outputKey)
		if !ok {
			// The entry is missing in the map. Try creating it.
			zeroHisto := histogram.FloatHistogram{
				CounterResetHint: histogram.CounterResetHint(h.value.ResetHint),
				Schema:           h.value.Schema,
				ZeroThreshold:    h.value.ZeroThreshold,
			}
			v = &histogramMergeStateValue{
				lastValues: make(map[string]lastHistogramState),
				merged:     zeroHisto,
			}
			vNew, loaded := as.m.LoadOrStore(outputKey, v)
			if loaded {
				// Use the entry created by a concurrent goroutine.
				v = vNew
			}
		}
		sv := v.(*histogramMergeStateValue)
		sv.mu.Lock()
		deleted := sv.deleted
		if !deleted {
			lv, ok := sv.lastValues[inputKey]
			outOfOrder := ok && lv.timestamp > h.value.Timestamp
			if !as.ignoreOutOfOrderSamples || !outOfOrder {
				if ok {
					if !fh.DetectReset(lv.value) {
						sv.merged.Add(fh).Sub(lv.value)
					} else {
						// reset detected
						var labels []prompbmarshal.Label
						labels = decompressLabels(labels, as.lc, outputKey)
						logger.Infof("Input histogram reset detected! labels: %v, curr: %s, prev: %s", labels, fh.TestExpression(), lv.value.TestExpression())
						sv.merged.Add(fh)
					}
					if err := sv.merged.Validate(); err != nil {
						var labels []prompbmarshal.Label
						labels = decompressLabels(labels, as.lc, outputKey)
						logger.Errorf("Merged histogram is invalid! labels: %v, value: %s, err: %v", labels, sv.merged.TestExpression(), err)
						sv.merged = histogram.FloatHistogram{
							CounterResetHint: histogram.CounterReset,
							Schema:           h.value.Schema,
							ZeroThreshold:    h.value.ZeroThreshold,
						}
					}
				}
				lv.value = fh
				lv.timestamp = h.value.Timestamp
				lv.deleteDeadline = deleteDeadline
				sv.lastValues[inputKey] = lv
				sv.deleteDeadline = deleteDeadline
			}
		}
		sv.mu.Unlock()
		if deleted {
			// The entry has been deleted by the concurrent call to flushState
			// Try obtaining and updating the entry again.
			goto again
		}
	}
}

func (as *histogramMergeAggrState) removeOldEntries(currentTime uint64) {
	m := &as.m
	m.Range(func(k, v interface{}) bool {
		sv := v.(*histogramMergeStateValue)

		sv.mu.Lock()
		deleted := currentTime > sv.deleteDeadline
		if deleted {
			// Mark the merged entry as deleted
			sv.deleted = deleted
		} else {
			// Delete outdated entries in sv.lastValues
			m := sv.lastValues
			for k1, v1 := range m {
				if currentTime > v1.deleteDeadline {
					delete(m, k1)
				}
			}
		}
		sv.mu.Unlock()

		if deleted {
			m.Delete(k)
		}
		return true
	})
}

func (as *histogramMergeAggrState) flushState(ctx *flushCtx, resetState bool) {
	_ = resetState // it isn't used here
	currentTime := fasttime.UnixTimestamp()
	currentTimeMsec := int64(currentTime) * 1000

	as.removeOldEntries(currentTime)

	m := &as.m
	m.Range(func(k, v interface{}) bool {
		sv := v.(*histogramMergeStateValue)
		sv.mu.Lock()
		if !sv.deleted {
			sv.merged.Compact(0) // reduce buckets before sending to storage
			key := k.(string)
			value := prompbmarshal.FromFloatHistogram(&sv.merged)
			value.Timestamp = currentTimeMsec
			ctx.appendHistograms(key, as.suffix, value)
		}
		sv.mu.Unlock()
		return true
	})
}
