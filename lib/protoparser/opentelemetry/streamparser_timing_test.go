package opentelemetry

import (
	"bytes"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/opentelemetry/pb"
)

func BenchmarkParseStream(b *testing.B) {
	samples := []*pb.Metric{generateGauge("my-gauge"), generateHistogram("my-histogram"), generateSum("my-sum"), generateSummary("my-summary")}
	b.ReportAllocs()
	b.RunParallel(func(p *testing.PB) {
		for p.Next() {
			pbRequest := pb.ExportMetricsServiceRequest{
				ResourceMetrics: []*pb.ResourceMetrics{generateOTLPSamples(samples...)},
			}
			data, err := pbRequest.MarshalVT()
			if err != nil {
				b.Fatalf("cannot marshal data: %s", err)
			}
			err = ParseStream(bytes.NewBuffer(data), false, false, func(tss []prompb.TimeSeries) error {
				return nil
			})
			if err != nil {
				b.Fatalf("cannot parse stream: %s", err)
			}
		}
	})

}

func BenchmarkScopedMetricToTimeSeries(b *testing.B) {

	scm := &pb.ScopeMetrics{
		Metrics: []*pb.Metric{
			{
				Name: "base_gauge",
				Data: &pb.Metric_Gauge{
					Gauge: &pb.Gauge{
						DataPoints: []*pb.NumberDataPoint{
							{
								TimeUnixNano: 12312124,
								Value:        &pb.NumberDataPoint_AsInt{AsInt: 100},
							},
						},
					},
				},
			},
			{
				Name: "base_summ",
				Data: &pb.Metric_Summary{
					Summary: &pb.Summary{
						DataPoints: []*pb.SummaryDataPoint{
							{
								TimeUnixNano: 12312124,
								Count:        15,
								Sum:          32,
								QuantileValues: []*pb.SummaryDataPoint_ValueAtQuantile{
									{
										Quantile: 0.5,
										Value:    15},
								},
							},
						},
					},
				},
			},
			{
				Name: "base_hsm",
				Data: &pb.Metric_Histogram{
					Histogram: &pb.Histogram{
						AggregationTemporality: pb.AggregationTemporality_AGGREGATION_TEMPORALITY_CUMULATIVE,
						DataPoints: []*pb.HistogramDataPoint{
							{
								TimeUnixNano:   12312124,
								Sum:            func() *float64 { v := 16.1; return &v }(),
								Count:          5,
								BucketCounts:   []uint64{1, 2, 3},
								ExplicitBounds: []float64{5, 6},
							},
						},
					},
				},
			},
		},
	}
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			tss := make([]prompb.TimeSeries, 0, 100)
			lbl := make([]prompb.Label, 0, 200)
			sample := make([]prompb.Sample, 0, 200)
			wr := &writeContext{tss: tss, labelsPool: lbl, samplesPool: sample}
			scopedMetricToTimeSeries(wr, scm)
		}
	})
}
