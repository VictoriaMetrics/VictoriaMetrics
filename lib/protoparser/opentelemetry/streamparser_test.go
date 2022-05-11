package opentelemetry

import (
	"bytes"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
	colmetricpb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	common "go.opentelemetry.io/proto/otlp/common/v1"
	metricpb "go.opentelemetry.io/proto/otlp/metrics/v1"
	metricsv1 "go.opentelemetry.io/proto/otlp/metrics/v1"
	resource "go.opentelemetry.io/proto/otlp/resource/v1"
	"google.golang.org/protobuf/proto"
)

func TestParseStream(t *testing.T) {
	prettifyLabel := func(label prompb.Label) string {
		return fmt.Sprintf("name=%q value=%q", label.Name, label.Value)
	}
	prettifySample := func(sample prompb.Sample) string {
		return fmt.Sprintf("sample=%f timestamp: %d", sample.Value, sample.Timestamp)
	}
	f := func(name string, samples []*metricsv1.Metric, expectedTss []prompb.TimeSeries) {
		t.Run(name, func(t *testing.T) {
			pbRequest := colmetricpb.ExportMetricsServiceRequest{
				ResourceMetrics: []*metricpb.ResourceMetrics{generateOTLPSamples(samples...)},
			}
			data, err := proto.Marshal(&pbRequest)
			if err != nil {
				t.Fatalf("cannot marshal data: %s", err)
			}
			err = ParseStream(bytes.NewBuffer(data), false, false, func(tss []prompb.TimeSeries) error {
				if len(tss) != len(expectedTss) {
					t.Fatalf("not expected tss count, got: %d, want: %d", len(tss), len(expectedTss))
				}
				for i := 0; i < len(tss); i++ {
					ts := tss[i]
					tsWant := expectedTss[i]
					if len(ts.Labels) != len(tsWant.Labels) {
						t.Fatalf("idx: %d, not expected labels count, got: %d, want: %d", i, len(ts.Labels), len(tsWant.Labels))
					}
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
		[]*metricsv1.Metric{generateGauge("my-gauge"), generateHistogram("my-histogram"), generateSum("my-sum"), generateSummary("my-summary")},
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
		[]*metricpb.Metric{generateGauge("my-gauge")},
		[]prompb.TimeSeries{newPromPBTs("my-gauge", 15000, 15.0, false, jobLabelValue, kvLabel("label1", "value1"))})
}

func attributesFromKV(kvs ...[2]string) []*common.KeyValue {
	var r []*common.KeyValue
	for _, kv := range kvs {
		r = append(r, &common.KeyValue{
			Key:   kv[0],
			Value: &common.AnyValue{Value: &common.AnyValue_StringValue{StringValue: kv[1]}},
		})
	}
	return r
}

func generateGauge(name string, points ...*metricpb.NumberDataPoint) *metricsv1.Metric {
	defaultPoints := []*metricpb.NumberDataPoint{
		{

			Attributes:   attributesFromKV([2]string{"label1", "value1"}),
			Value:        &metricpb.NumberDataPoint_AsInt{AsInt: 15},
			TimeUnixNano: uint64(15 * time.Second),
		},
	}
	if len(points) == 0 {
		points = defaultPoints
	}
	return &metricsv1.Metric{
		Name: name,
		Data: &metricsv1.Metric_Gauge{
			Gauge: &metricsv1.Gauge{
				DataPoints: points,
			},
		},
	}
}

func generateHistogram(name string, points ...*metricpb.HistogramDataPoint) *metricsv1.Metric {
	defaultPoints := []*metricpb.HistogramDataPoint{
		{

			Attributes:     attributesFromKV([2]string{"label2", "value2"}),
			Count:          15,
			Sum:            func() *float64 { f := 30.0; return &f }(),
			ExplicitBounds: []float64{0.1, 0.5, 1.0, 5.0},
			BucketCounts:   []uint64{0, 5, 10, 0, 0},
			TimeUnixNano:   uint64(30 * time.Second),
		},
	}
	if len(points) == 0 {
		points = defaultPoints
	}
	return &metricsv1.Metric{
		Name: name,
		Data: &metricsv1.Metric_Histogram{
			Histogram: &metricpb.Histogram{
				AggregationTemporality: metricpb.AggregationTemporality_AGGREGATION_TEMPORALITY_CUMULATIVE,
				DataPoints:             points,
			},
		},
	}
}

func generateSum(name string, points ...*metricpb.NumberDataPoint) *metricsv1.Metric {
	defaultPoints := []*metricpb.NumberDataPoint{
		{
			Attributes:   attributesFromKV([2]string{"label5", "value5"}),
			Value:        &metricpb.NumberDataPoint_AsDouble{AsDouble: 15.5},
			TimeUnixNano: uint64(150 * time.Second),
		},
	}
	if len(points) == 0 {
		points = defaultPoints
	}
	return &metricsv1.Metric{
		Name: name,
		Data: &metricsv1.Metric_Sum{
			Sum: &metricpb.Sum{
				AggregationTemporality: metricpb.AggregationTemporality_AGGREGATION_TEMPORALITY_CUMULATIVE,
				DataPoints:             points,
			},
		},
	}
}

func generateSummary(name string, points ...*metricpb.SummaryDataPoint) *metricsv1.Metric {
	defaultPoints := []*metricpb.SummaryDataPoint{
		{
			Attributes:   attributesFromKV([2]string{"label6", "value6"}),
			TimeUnixNano: uint64(35 * time.Second),
			Sum:          32.5,
			Count:        5,
			QuantileValues: []*metricpb.SummaryDataPoint_ValueAtQuantile{
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
	return &metricsv1.Metric{
		Name: name,
		Data: &metricsv1.Metric_Summary{
			Summary: &metricpb.Summary{
				DataPoints: points,
			},
		},
	}
}

func generateOTLPSamples(srcs ...*metricsv1.Metric) *metricpb.ResourceMetrics {
	otlpMetrics := &metricpb.ResourceMetrics{
		Resource: &resource.Resource{Attributes: attributesFromKV([2]string{"job", "vm"})},
	}
	otlpMetrics.ScopeMetrics = []*metricsv1.ScopeMetrics{
		{
			Metrics: append([]*metricsv1.Metric{}, srcs...),
		},
	}
	return otlpMetrics
}
