package opentelemetry

import (
	"bytes"
	"fmt"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/decimal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/opentelemetry/pb"
)

func TestParseStream(t *testing.T) {
	prettifyLabel := func(label prompb.Label) string {
		return fmt.Sprintf("name=%q value=%q", label.Name, label.Value)
	}
	prettifySample := func(sample prompb.Sample) string {
		return fmt.Sprintf("sample=%f timestamp: %d", sample.Value, sample.Timestamp)
	}
	f := func(name string, samples []*pb.Metric, expectedTss []prompb.TimeSeries) {
		t.Run(name, func(t *testing.T) {
			pbRequest := pb.ExportMetricsServiceRequest{
				ResourceMetrics: []*pb.ResourceMetrics{generateOTLPSamples(samples...)},
			}
			data, err := pbRequest.MarshalVT()
			if err != nil {
				t.Fatalf("cannot marshal data: %s", err)
			}
			err = ParseStream(bytes.NewBuffer(data), false, false, func(tss []prompb.TimeSeries) error {
				if len(tss) != len(expectedTss) {
					t.Fatalf("not expected tss count, got: %d, want: %d", len(tss), len(expectedTss))
				}
				sort.Slice(expectedTss, func(i, j int) bool {
					var n1, n2 string
					for _, l := range expectedTss[i].Labels {
						if string(l.Name) == "__name__" {
							n1 = string(l.Value)
						}
					}
					for _, l := range expectedTss[j].Labels {
						if string(l.Name) == "__name__" {
							n2 = string(l.Value)
						}
					}
					return n1 < n2
				})
				sort.Slice(tss, func(i, j int) bool {
					var n1, n2 string
					for _, l := range tss[i].Labels {
						if string(l.Name) == "__name__" {
							n1 = string(l.Value)
						}
					}
					for _, l := range tss[j].Labels {
						if string(l.Name) == "__name__" {
							n2 = string(l.Value)
						}
					}
					return n1 < n2
				})
				for i := 0; i < len(tss); i++ {
					ts := tss[i]
					tsWant := expectedTss[i]
					if len(ts.Labels) != len(tsWant.Labels) {
						t.Fatalf("idx: %d, not expected labels count, got: %d, want: %d", i, len(ts.Labels), len(tsWant.Labels))
					}
					sort.Slice(ts.Labels, func(i, j int) bool {
						return string(ts.Labels[i].Name) < string(ts.Labels[j].Name)
					})
					sort.Slice(tsWant.Labels, func(i, j int) bool {
						return string(tsWant.Labels[i].Name) < string(tsWant.Labels[j].Name)
					})
					for j, label := range ts.Labels {
						wantLabel := tsWant.Labels[j]
						if !reflect.DeepEqual(label, wantLabel) {
							t.Fatalf("idx: %d, label idx: %d, not equal label pairs, \ngot: \n%s, \nwant: \n%s", i, j, prettifyLabel(label), prettifyLabel(wantLabel))
						}
					}
					if len(ts.Samples) != len(tsWant.Samples) {
						t.Fatalf("idx: %d, not expected samples count, got: %d, want: %d", i, len(ts.Samples), len(tsWant.Samples))
					}
					for j, sample := range ts.Samples {
						wantSample := tsWant.Samples[j]
						if !reflect.DeepEqual(sample, wantSample) {
							t.Fatalf("idx: %d, label idx: %d, not equal sample pairs, \ngot: \n%s,\nwant: \n%s", i, j, prettifySample(sample), prettifySample(wantSample))
						}
					}
				}
				return nil
			})
			if err != nil {
				t.Fatalf("unexpected error: %s", err)
			}
		})
	}
	jobLabelValue := prompb.Label{Name: []byte(`job`), Value: []byte(`vm`)}
	leLabel := func(value string) prompb.Label {
		return prompb.Label{Name: boundLabel, Value: []byte(value)}
	}
	kvLabel := func(k, v string) prompb.Label {
		return prompb.Label{Name: []byte(k), Value: []byte(v)}
	}
	f("test all metric types",
		[]*pb.Metric{generateGauge("my-gauge"), generateHistogram("my-histogram"), generateSum("my-sum"), generateSummary("my-summary")},
		[]prompb.TimeSeries{
			newPromPBTs("my-gauge", 15000, 15.0, false, jobLabelValue, kvLabel("label1", "value1")),
			newPromPBTs("my-histogram_count", 30000, 15.0, false, jobLabelValue, kvLabel("label2", "value2")),
			newPromPBTs("my-histogram_sum", 30000, 30.0, false, jobLabelValue, kvLabel("label2", "value2")),
			newPromPBTs("my-histogram_bucket", 30000, 0.0, false, jobLabelValue, kvLabel("label2", "value2"), leLabel("0.1")),
			newPromPBTs("my-histogram_bucket", 30000, 5.0, false, jobLabelValue, kvLabel("label2", "value2"), leLabel("0.5")),
			newPromPBTs("my-histogram_bucket", 30000, 15.0, false, jobLabelValue, kvLabel("label2", "value2"), leLabel("1")),
			newPromPBTs("my-histogram_bucket", 30000, 15.0, false, jobLabelValue, kvLabel("label2", "value2"), leLabel("5")),
			newPromPBTs("my-histogram_bucket", 30000, 15.0, false, jobLabelValue, kvLabel("label2", "value2"), leLabel("+Inf")),
			newPromPBTs("my-sum", 150000, 15.5, false, jobLabelValue, kvLabel("label5", "value5")),
			newPromPBTs("my-summary_sum", 35000, 32.5, false, jobLabelValue, kvLabel("label6", "value6")),
			newPromPBTs("my-summary_count", 35000, 5.0, false, jobLabelValue, kvLabel("label6", "value6")),
			newPromPBTs("my-summary", 35000, 7.5, false, jobLabelValue, kvLabel("label6", "value6"), kvLabel("quantile", "0.1")),
			newPromPBTs("my-summary", 35000, 10.0, false, jobLabelValue, kvLabel("label6", "value6"), kvLabel("quantile", "0.5")),
			newPromPBTs("my-summary", 35000, 15.0, false, jobLabelValue, kvLabel("label6", "value6"), kvLabel("quantile", "1")),
		})
	f("test gauge",
		[]*pb.Metric{generateGauge("my-gauge")},
		[]prompb.TimeSeries{newPromPBTs("my-gauge", 15000, 15.0, false, jobLabelValue, kvLabel("label1", "value1"))})
}

func attributesFromKV(kvs ...[2]string) []*pb.KeyValue {
	var r []*pb.KeyValue
	for _, kv := range kvs {
		r = append(r, &pb.KeyValue{
			Key:   kv[0],
			Value: &pb.AnyValue{Value: &pb.AnyValue_StringValue{StringValue: kv[1]}},
		})
	}
	return r
}

func generateGauge(name string, points ...*pb.NumberDataPoint) *pb.Metric {
	defaultPoints := []*pb.NumberDataPoint{
		{

			Attributes:   attributesFromKV([2]string{"label1", "value1"}),
			Value:        &pb.NumberDataPoint_AsInt{AsInt: 15},
			TimeUnixNano: uint64(15 * time.Second),
		},
	}
	if len(points) == 0 {
		points = defaultPoints
	}
	return &pb.Metric{
		Name: name,
		Data: &pb.Metric_Gauge{
			Gauge: &pb.Gauge{
				DataPoints: points,
			},
		},
	}
}

func generateHistogram(name string, points ...*pb.HistogramDataPoint) *pb.Metric {
	defaultPoints := []*pb.HistogramDataPoint{
		{

			Attributes:     attributesFromKV([2]string{"label2", "value2"}),
			Count:          15,
			Sum:            func() *float64 { v := 30.0; return &v }(),
			ExplicitBounds: []float64{0.1, 0.5, 1.0, 5.0},
			BucketCounts:   []uint64{0, 5, 10, 0, 0},
			TimeUnixNano:   uint64(30 * time.Second),
		},
	}
	if len(points) == 0 {
		points = defaultPoints
	}
	return &pb.Metric{
		Name: name,
		Data: &pb.Metric_Histogram{
			Histogram: &pb.Histogram{
				AggregationTemporality: pb.AggregationTemporality_AGGREGATION_TEMPORALITY_CUMULATIVE,
				DataPoints:             points,
			},
		},
	}
}

func generateSum(name string, points ...*pb.NumberDataPoint) *pb.Metric {
	defaultPoints := []*pb.NumberDataPoint{
		{
			Attributes:   attributesFromKV([2]string{"label5", "value5"}),
			Value:        &pb.NumberDataPoint_AsDouble{AsDouble: 15.5},
			TimeUnixNano: uint64(150 * time.Second),
		},
	}
	if len(points) == 0 {
		points = defaultPoints
	}
	return &pb.Metric{
		Name: name,
		Data: &pb.Metric_Sum{
			Sum: &pb.Sum{
				AggregationTemporality: pb.AggregationTemporality_AGGREGATION_TEMPORALITY_CUMULATIVE,
				DataPoints:             points,
			},
		},
	}
}

func generateSummary(name string, points ...*pb.SummaryDataPoint) *pb.Metric {
	defaultPoints := []*pb.SummaryDataPoint{
		{
			Attributes:   attributesFromKV([2]string{"label6", "value6"}),
			TimeUnixNano: uint64(35 * time.Second),
			Sum:          32.5,
			Count:        5,
			QuantileValues: []*pb.SummaryDataPoint_ValueAtQuantile{
				{
					Quantile: 0.1,
					Value:    7.5,
				},
				{
					Quantile: 0.5,
					Value:    10.0,
				},
				{
					Quantile: 1.0,
					Value:    15.0,
				},
			},
		},
	}
	if len(points) == 0 {
		points = defaultPoints
	}
	return &pb.Metric{
		Name: name,
		Data: &pb.Metric_Summary{
			Summary: &pb.Summary{
				DataPoints: points,
			},
		},
	}
}

func generateOTLPSamples(srcs ...*pb.Metric) *pb.ResourceMetrics {
	otlpMetrics := &pb.ResourceMetrics{
		Resource: &pb.Resource{Attributes: attributesFromKV([2]string{"job", "vm"})},
	}
	otlpMetrics.ScopeMetrics = []*pb.ScopeMetrics{
		{
			Metrics: append([]*pb.Metric{}, srcs...),
		},
	}
	return otlpMetrics
}

func newPromPBTs(metricName string, t int64, value float64, isStale bool, extraLabels ...prompb.Label) prompb.TimeSeries {
	if isStale {
		value = decimal.StaleNaN
	}
	if t <= 0 {
		// Set the current timestamp if t isn't set.
		t = int64(fasttime.UnixTimestamp()) * 1000
	}
	ts := prompb.TimeSeries{
		Labels: []prompb.Label{
			{
				Name:  metricNameLabel,
				Value: []byte(metricName),
			},
		},
		Samples: []prompb.Sample{
			{
				Value:     value,
				Timestamp: t,
			},
		},
	}
	ts.Labels = append(ts.Labels, extraLabels...)
	return ts
}
