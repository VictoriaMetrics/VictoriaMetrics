package prompbmarshal

import (
	"fmt"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/prometheus"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/slicesutil"
)

// MarshalProtobuf marshals wr to dst and returns the result.
func (wr *WriteRequest) MarshalProtobuf(dst []byte) []byte {
	size := wr.Size()
	dstLen := len(dst)
	dst = slicesutil.SetLength(dst, dstLen+size)
	n, err := wr.MarshalToSizedBuffer(dst[dstLen:])
	if err != nil {
		panic(fmt.Errorf("BUG: unexpected error when marshaling WriteRequest: %w", err))
	}
	return dst[:dstLen+n]
}

// Reset resets wr.
func (wr *WriteRequest) Reset() {
	wr.Timeseries = ResetTimeSeries(wr.Timeseries)
}

// ResetTimeSeries clears all the GC references from tss and returns an empty tss ready for further use.
func ResetTimeSeries(tss []TimeSeries) []TimeSeries {
	clear(tss)
	return tss[:0]
}

// MustParsePromMetrics parses metrics in Prometheus text exposition format from s and returns them.
//
// Metrics must be delimited with newlines.
//
// offsetMsecs is added to every timestamp in parsed metrics.
//
// This function is for testing purposes only. Do not use it in non-test code.
func MustParsePromMetrics(s string, offsetMsecs int64) []TimeSeries {
	var rows prometheus.Rows
	errLogger := func(s string) {
		panic(fmt.Errorf("unexpected error when parsing Prometheus metrics: %s", s))
	}
	rows.UnmarshalWithErrLogger(s, errLogger)
	tss := make([]TimeSeries, 0, len(rows.Rows))
	samples := make([]Sample, 0, len(rows.Rows))
	for _, row := range rows.Rows {
		labels := make([]Label, 0, len(row.Tags)+1)
		labels = append(labels, Label{
			Name:  "__name__",
			Value: row.Metric,
		})
		for _, tag := range row.Tags {
			labels = append(labels, Label{
				Name:  tag.Key,
				Value: tag.Value,
			})
		}
		samples = append(samples, Sample{
			Value:     row.Value,
			Timestamp: row.Timestamp + offsetMsecs,
		})
		ts := TimeSeries{
			Labels:  labels,
			Samples: samples[len(samples)-1:],
		}
		tss = append(tss, ts)
	}
	return tss
}
