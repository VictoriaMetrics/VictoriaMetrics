package main

import (
	"sort"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
)

func newTimeSeries(values []float64, timestamps []int64, labels map[string]string) prompbmarshal.TimeSeries {
	ts := prompbmarshal.TimeSeries{
		Samples: make([]prompbmarshal.Sample, len(values)),
	}
	for i := range values {
		ts.Samples[i] = prompbmarshal.Sample{
			Value:     values[i],
			Timestamp: time.Unix(timestamps[i], 0).UnixNano() / 1e6,
		}
	}
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys) // make order deterministic
	for _, key := range keys {
		ts.Labels = append(ts.Labels, prompbmarshal.Label{
			Name:  key,
			Value: labels[key],
		})
	}
	return ts
}
