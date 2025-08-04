package prometheus

import (
	"fmt"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
)

// MustParsePromMetrics parses metrics in Prometheus text exposition format from s and returns them.
//
// Metrics must be delimited with newlines.
//
// offsetMsecs is added to every timestamp in parsed metrics.
//
// This function is for testing purposes only. Do not use it in non-test code.
func MustParsePromMetrics(s string, offsetMsecs int64) []prompb.TimeSeries {
	var rows Rows
	errLogger := func(s string) {
		panic(fmt.Errorf("unexpected error when parsing Prometheus metrics: %s", s))
	}
	rows.UnmarshalWithErrLogger(s, errLogger)
	tss := make([]prompb.TimeSeries, 0, len(rows.Rows))
	samples := make([]prompb.Sample, 0, len(rows.Rows))
	for _, row := range rows.Rows {
		labels := make([]prompb.Label, 0, len(row.Tags)+1)
		labels = append(labels, prompb.Label{
			Name:  "__name__",
			Value: row.Metric,
		})
		for _, tag := range row.Tags {
			labels = append(labels, prompb.Label{
				Name:  tag.Key,
				Value: tag.Value,
			})
		}
		samples = append(samples, prompb.Sample{
			Value:     row.Value,
			Timestamp: row.Timestamp + offsetMsecs,
		})
		ts := prompb.TimeSeries{
			Labels:  labels,
			Samples: samples[len(samples)-1:],
		}
		tss = append(tss, ts)
	}
	return tss
}
