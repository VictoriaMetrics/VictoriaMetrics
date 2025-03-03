package prometheus

import (
	"fmt"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
)

// MustParsePromMetrics parses metrics in Prometheus text exposition format from s and returns them.
//
// Metrics must be delimited with newlines.
//
// offsetMsecs is added to every timestamp in parsed metrics.
//
// This function is for testing purposes only. Do not use it in non-test code.
func MustParsePromMetrics(s string, offsetMsecs int64) []prompbmarshal.TimeSeries {
	var rows Rows
	errLogger := func(s string) {
		panic(fmt.Errorf("unexpected error when parsing Prometheus metrics: %s", s))
	}
	rows.UnmarshalWithErrLogger(s, errLogger)
	tss := make([]prompbmarshal.TimeSeries, 0, len(rows.Rows))
	samples := make([]prompbmarshal.Sample, 0, len(rows.Rows))
	for _, row := range rows.Rows {
		labels := make([]prompbmarshal.Label, 0, len(row.Tags)+1)
		labels = append(labels, prompbmarshal.Label{
			Name:  "__name__",
			Value: row.Metric,
		})
		for _, tag := range row.Tags {
			labels = append(labels, prompbmarshal.Label{
				Name:  tag.Key,
				Value: tag.Value,
			})
		}
		samples = append(samples, prompbmarshal.Sample{
			Value:     row.Value,
			Timestamp: row.Timestamp + offsetMsecs,
		})
		ts := prompbmarshal.TimeSeries{
			Labels:  labels,
			Samples: samples[len(samples)-1:],
		}
		tss = append(tss, ts)
	}
	return tss
}
