package main

import (
	"sort"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
)

func newTimeSeries(value float64, labels map[string]string, timestamp time.Time) prompbmarshal.TimeSeries {
	ts := prompbmarshal.TimeSeries{}
	ts.Samples = append(ts.Samples, prompbmarshal.Sample{
		Value:     value,
		Timestamp: timestamp.UnixNano() / 1e6,
	})
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, key := range keys {
		ts.Labels = append(ts.Labels, prompbmarshal.Label{
			Name:  key,
			Value: labels[key],
		})
	}
	return ts
}
