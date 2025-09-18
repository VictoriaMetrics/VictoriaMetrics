package stream

import (
	"bytes"
	"fmt"
	"io"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/klauspost/compress/gzip"
	"github.com/klauspost/compress/zstd"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/opentelemetry/pb"
)

func TestParseStream(t *testing.T) {
	f := func(samples []*pb.Metric, tssExpected []prompb.TimeSeries, mmsExpected []prompb.MetricMetadata, usePromNaming bool) {
		t.Helper()

		prevPromNaming := *usePrometheusNaming
		*usePrometheusNaming = usePromNaming
		defer func() {
			*usePrometheusNaming = prevPromNaming
		}()

		checkSeries := func(tss []prompb.TimeSeries, mms []prompb.MetricMetadata) error {
			if len(tss) != len(tssExpected) {
				return fmt.Errorf("not expected time series count, got: %d, want: %d", len(tss), len(tssExpected))
			}
			sortByMetricName(tss)
			sortByMetricName(tssExpected)
			for i := 0; i < len(tss); i++ {
				ts := tss[i]
				tsExpected := tssExpected[i]
				if len(ts.Labels) != len(tsExpected.Labels) {
					return fmt.Errorf("idx: %d, not expected labels count, got: %d, want: %d", i, len(ts.Labels), len(tsExpected.Labels))
				}
				sortLabels(ts.Labels)
				sortLabels(tsExpected.Labels)
				for j, label := range ts.Labels {
					labelExpected := tsExpected.Labels[j]
					if !reflect.DeepEqual(label, labelExpected) {
						return fmt.Errorf("idx: %d, label idx: %d, not equal label pairs, \ngot: \n%s, \nwant: \n%s",
							i, j, prettifyLabel(label), prettifyLabel(labelExpected))
					}
				}
				if len(ts.Samples) != len(tsExpected.Samples) {
					return fmt.Errorf("idx: %d, not expected samples count, got: %d, want: %d", i, len(ts.Samples), len(tsExpected.Samples))
				}
				for j, sample := range ts.Samples {
					sampleExpected := tsExpected.Samples[j]
					if !reflect.DeepEqual(sample, sampleExpected) {
						return fmt.Errorf("idx: %d, label idx: %d, not equal sample pairs, \ngot: \n%s,\nwant: \n%s",
							i, j, prettifySample(sample), prettifySample(sampleExpected))
					}
				}
			}

			if len(mms) != len(mmsExpected) {
				return fmt.Errorf("not expected metadata count, got: %d, want: %d", len(mms), len(mmsExpected))
			}
			for i := range mms {
				if mms[i].Type != mmsExpected[i].Type {
					return fmt.Errorf("idx: %d, not expected metadata type, got: %q, want: %q", i, mms[i].Type, mmsExpected[i].Type)
				}
				if mms[i].MetricFamilyName != mmsExpected[i].MetricFamilyName {
					return fmt.Errorf("idx: %d, not expected metadata metric family name, got: %q, want: %q", i, mms[i].MetricFamilyName, mmsExpected[i].MetricFamilyName)
				}
				if mms[i].Help != mmsExpected[i].Help {
					return fmt.Errorf("idx: %d, not expected metadata help, got: %q, want: %q", i, mms[i].Help, mmsExpected[i].Help)
				}
				if mms[i].Unit != mmsExpected[i].Unit {
					return fmt.Errorf("idx: %d, not expected metadata unit, got: %q, want: %q", i, mms[i].Unit, mmsExpected[i].Unit)
				}
			}
			return nil
		}

		req := &pb.ExportMetricsServiceRequest{
			ResourceMetrics: []*pb.ResourceMetrics{
				generateOTLPSamples(samples),
			},
		}

		// Verify protobuf parsing
		pbData := req.MarshalProtobuf(nil)
		if err := checkParseStream(pbData, checkSeries); err != nil {
			t.Fatalf("cannot parse protobuf: %s", err)
		}
	}

	jobLabelValue := prompb.Label{
		Name:  "job",
		Value: "vm",
	}
	leLabel := func(value string) prompb.Label {
		return prompb.Label{
			Name:  "le",
			Value: value,
		}
	}
	kvLabel := func(k, v string) prompb.Label {
		return prompb.Label{
			Name:  k,
			Value: v,
		}
	}

	// Test all metric types
	f(
		[]*pb.Metric{
			generateGauge("my-gauge", ""),
			generateGaugeUnknown("my-gauge-unknown", ""),
			generateHistogram("my-histogram", "", true),
			generateHistogram("my-sumless-histogram", "", false),
			generateSum("my-sum", "", false),
			generateSummary("my-summary", ""),
		},
		[]prompb.TimeSeries{
			newPromPBTs("my-gauge", 15000, 15.0, jobLabelValue, kvLabel("label1", "value1")),
			newPromPBTs("my-gauge-unknown", 15000, 15.0, jobLabelValue, kvLabel("label1", "value1")),
			newPromPBTs("my-histogram_count", 30000, 15.0, jobLabelValue, kvLabel("label2", "value2")),
			newPromPBTs("my-histogram_sum", 30000, 30.0, jobLabelValue, kvLabel("label2", "value2")),
			newPromPBTs("my-histogram_bucket", 30000, 0.0, jobLabelValue, kvLabel("label2", "value2"), leLabel("0.1")),
			newPromPBTs("my-histogram_bucket", 30000, 5.0, jobLabelValue, kvLabel("label2", "value2"), leLabel("0.5")),
			newPromPBTs("my-histogram_bucket", 30000, 15.0, jobLabelValue, kvLabel("label2", "value2"), leLabel("1")),
			newPromPBTs("my-histogram_bucket", 30000, 15.0, jobLabelValue, kvLabel("label2", "value2"), leLabel("5")),
			newPromPBTs("my-histogram_bucket", 30000, 15.0, jobLabelValue, kvLabel("label2", "value2"), leLabel("+Inf")),
			newPromPBTs("my-sumless-histogram_count", 30000, 15.0, jobLabelValue, kvLabel("label2", "value2")),
			newPromPBTs("my-sumless-histogram_bucket", 30000, 0.0, jobLabelValue, kvLabel("label2", "value2"), leLabel("0.1")),
			newPromPBTs("my-sumless-histogram_bucket", 30000, 5.0, jobLabelValue, kvLabel("label2", "value2"), leLabel("0.5")),
			newPromPBTs("my-sumless-histogram_bucket", 30000, 15.0, jobLabelValue, kvLabel("label2", "value2"), leLabel("1")),
			newPromPBTs("my-sumless-histogram_bucket", 30000, 15.0, jobLabelValue, kvLabel("label2", "value2"), leLabel("5")),
			newPromPBTs("my-sumless-histogram_bucket", 30000, 15.0, jobLabelValue, kvLabel("label2", "value2"), leLabel("+Inf")),
			newPromPBTs("my-sum", 150000, 15.5, jobLabelValue, kvLabel("label5", "value5")),
			newPromPBTs("my-summary_sum", 35000, 32.5, jobLabelValue, kvLabel("label6", "value6")),
			newPromPBTs("my-summary_count", 35000, 5.0, jobLabelValue, kvLabel("label6", "value6")),
			newPromPBTs("my-summary", 35000, 7.5, jobLabelValue, kvLabel("label6", "value6"), kvLabel("quantile", "0.1")),
			newPromPBTs("my-summary", 35000, 10.0, jobLabelValue, kvLabel("label6", "value6"), kvLabel("quantile", "0.5")),
			newPromPBTs("my-summary", 35000, 15.0, jobLabelValue, kvLabel("label6", "value6"), kvLabel("quantile", "1")),
		},
		[]prompb.MetricMetadata{
			{
				MetricFamilyName: "my-gauge",
				Help:             "I'm a gauge",
				Type:             uint32(prompb.MetricMetadataGAUGE),
				Unit:             "",
			},
			{
				MetricFamilyName: "my-gauge-unknown",
				Help:             "I'm not a gauge",
				Type:             uint32(prompb.MetricMetadataUNKNOWN),
				Unit:             "",
			},
			{
				MetricFamilyName: "my-histogram",
				Help:             "I'm a Histogram",
				Type:             uint32(prompb.MetricMetadataHISTOGRAM),
				Unit:             "",
			},
			{
				MetricFamilyName: "my-sumless-histogram",
				Help:             "I'm a Histogram",
				Type:             uint32(prompb.MetricMetadataHISTOGRAM),
				Unit:             "",
			},
			{
				MetricFamilyName: "my-sum",
				Help:             "I might be a counter or gauge, depending on the IsMonotonic",
				Type:             uint32(prompb.MetricMetadataGAUGE),
				Unit:             "",
			},
			{
				MetricFamilyName: "my-summary",
				Help:             "I'm a Summary",
				Type:             uint32(prompb.MetricMetadataSUMMARY),
				Unit:             "",
			},
		},
		false,
	)

	// Test gauge
	f(
		[]*pb.Metric{
			generateGauge("my-gauge", ""),
		},
		[]prompb.TimeSeries{
			newPromPBTs("my-gauge", 15000, 15.0, jobLabelValue, kvLabel("label1", "value1")),
		},
		[]prompb.MetricMetadata{
			{
				MetricFamilyName: "my-gauge",
				Help:             "I'm a gauge",
				Type:             uint32(prompb.MetricMetadataGAUGE),
				Unit:             "",
			},
		},
		false,
	)

	// Test gauge with unit and prometheus naming
	f(
		[]*pb.Metric{
			generateGauge("my-gauge", "ms"),
		},
		[]prompb.TimeSeries{
			newPromPBTs("my_gauge_milliseconds", 15000, 15.0, jobLabelValue, kvLabel("label1", "value1")),
		},
		[]prompb.MetricMetadata{
			{
				MetricFamilyName: "my_gauge_milliseconds",
				Help:             "I'm a gauge",
				Type:             uint32(prompb.MetricMetadataGAUGE),
				Unit:             "ms",
			},
		},
		true,
	)

	// Test gauge with unit inside metric
	f(
		[]*pb.Metric{
			generateGauge("my-gauge-milliseconds", "ms"),
		},
		[]prompb.TimeSeries{
			newPromPBTs("my_gauge_milliseconds", 15000, 15.0, jobLabelValue, kvLabel("label1", "value1")),
		},
		[]prompb.MetricMetadata{
			{
				MetricFamilyName: "my_gauge_milliseconds",
				Help:             "I'm a gauge",
				Type:             uint32(prompb.MetricMetadataGAUGE),
				Unit:             "ms",
			},
		},
		true,
	)

	// Test gauge with ratio suffix
	f(
		[]*pb.Metric{
			generateGauge("my-gauge-milliseconds", "1"),
		},
		[]prompb.TimeSeries{
			newPromPBTs("my_gauge_milliseconds_ratio", 15000, 15.0, jobLabelValue, kvLabel("label1", "value1")),
		},
		[]prompb.MetricMetadata{
			{
				MetricFamilyName: "my_gauge_milliseconds_ratio",
				Help:             "I'm a gauge",
				Type:             uint32(prompb.MetricMetadataGAUGE),
				Unit:             "1",
			},
		},
		true,
	)

	// Test sum with total suffix
	f(
		[]*pb.Metric{
			generateSum("my-sum", "ms", true),
		},
		[]prompb.TimeSeries{
			newPromPBTs("my_sum_milliseconds_total", 150000, 15.5, jobLabelValue, kvLabel("label5", "value5")),
		},
		[]prompb.MetricMetadata{
			{
				MetricFamilyName: "my_sum_milliseconds_total",
				Help:             "I might be a counter or gauge, depending on the IsMonotonic",
				Type:             uint32(prompb.MetricMetadataCOUNTER),
				Unit:             "ms",
			},
		},
		true,
	)

	// Test sum with total suffix, which exists in a metric name
	f(
		[]*pb.Metric{
			generateSum("my-total-sum", "ms", true),
		},
		[]prompb.TimeSeries{
			newPromPBTs("my_sum_milliseconds_total", 150000, 15.5, jobLabelValue, kvLabel("label5", "value5")),
		},
		[]prompb.MetricMetadata{
			{
				MetricFamilyName: "my_sum_milliseconds_total",
				Help:             "I might be a counter or gauge, depending on the IsMonotonic",
				Type:             uint32(prompb.MetricMetadataCOUNTER),
				Unit:             "ms",
			},
		},
		true,
	)

	// Test sum with total and complex suffix
	f(
		[]*pb.Metric{
			generateSum("my-total-sum", "m/s", true),
		},
		[]prompb.TimeSeries{
			newPromPBTs("my_sum_meters_per_second_total", 150000, 15.5, jobLabelValue, kvLabel("label5", "value5")),
		},
		[]prompb.MetricMetadata{
			{
				MetricFamilyName: "my_sum_meters_per_second_total",
				Help:             "I might be a counter or gauge, depending on the IsMonotonic",
				Type:             uint32(prompb.MetricMetadataCOUNTER),
				Unit:             "m/s",
			},
		},
		true,
	)

	// Test exponential histograms
	f(
		[]*pb.Metric{
			generateExpHistogram("test-histogram", "m/s"),
		},
		[]prompb.TimeSeries{
			newPromPBTs("test_histogram_meters_per_second_bucket", 15000, 5.0, jobLabelValue, kvLabel("label1", "value1"), kvLabel("vmrange", "1.061e+00...1.067e+00")),
			newPromPBTs("test_histogram_meters_per_second_bucket", 15000, 10.0, jobLabelValue, kvLabel("label1", "value1"), kvLabel("vmrange", "1.067e+00...1.073e+00")),
			newPromPBTs("test_histogram_meters_per_second_bucket", 15000, 1.0, jobLabelValue, kvLabel("label1", "value1"), kvLabel("vmrange", "1.085e+00...1.091e+00")),
			newPromPBTs("test_histogram_meters_per_second_count", 15000, 20.0, jobLabelValue, kvLabel("label1", "value1")),
			newPromPBTs("test_histogram_meters_per_second_sum", 15000, 4578.0, jobLabelValue, kvLabel("label1", "value1")),
		},
		[]prompb.MetricMetadata{
			{
				MetricFamilyName: "test_histogram_meters_per_second",
				Type:             uint32(prompb.MetricMetadataHISTOGRAM),
				Unit:             "m/s",
			},
		},
		true,
	)

	// Test gauge with deeply nested attributes
	f(
		[]*pb.Metric{
			{
				Name:        "my-gauge",
				Description: "it's a test",
				Unit:        "",
				Gauge: &pb.Gauge{
					DataPoints: []*pb.NumberDataPoint{
						{
							Attributes: []*pb.KeyValue{
								{
									Key: "label1",
									Value: &pb.AnyValue{
										StringValue: ptrTo("value1"),
									},
								},
								{
									Key:   "emptylabelvalue",
									Value: &pb.AnyValue{},
								},
								{
									Key: "emptylabel",
								},
								{
									Key: "label_array",
									Value: &pb.AnyValue{
										ArrayValue: &pb.ArrayValue{
											Values: []*pb.AnyValue{
												{
													StringValue: ptrTo("value5"),
												},
												{
													KeyValueList: &pb.KeyValueList{},
												},
											},
										},
									},
								},
								{
									Key: "nested_label",
									Value: &pb.AnyValue{
										KeyValueList: &pb.KeyValueList{
											Values: []*pb.KeyValue{
												{
													Key: "empty_value",
												},
												{
													Key: "value_top_2",
													Value: &pb.AnyValue{
														StringValue: ptrTo("valuetop"),
													},
												},
												{
													Key: "nested_kv_list",
													Value: &pb.AnyValue{
														KeyValueList: &pb.KeyValueList{
															Values: []*pb.KeyValue{
																{
																	Key:   "integer",
																	Value: &pb.AnyValue{IntValue: ptrTo(int64(15))},
																},
																{
																	Key:   "doable",
																	Value: &pb.AnyValue{DoubleValue: ptrTo(5.1)},
																},
																{
																	Key:   "string",
																	Value: &pb.AnyValue{StringValue: ptrTo("value2")},
																},
															},
														},
													},
												},
											},
										},
									},
								},
							},
							IntValue:     ptrTo(int64(15)),
							TimeUnixNano: uint64(15 * time.Second),
						},
					},
				},
			},
		},
		[]prompb.TimeSeries{
			newPromPBTs("my-gauge",
				15000,
				15.0,
				jobLabelValue,
				kvLabel("label1", "value1"),
				kvLabel("emptylabelvalue", ""),
				kvLabel("emptylabel", ""),
				kvLabel("label_array", `["value5",{}]`),
				kvLabel("nested_label", `{"empty_value":null,"value_top_2":"valuetop","nested_kv_list":{"integer":15,"doable":5.1,"string":"value2"}}`)),
		},
		[]prompb.MetricMetadata{
			{
				MetricFamilyName: "my-gauge",
				Help:             "it's a test",
				Type:             uint32(prompb.MetricMetadataGAUGE),
			},
		},
		false,
	)
}

func checkParseStream(data []byte, checkSeries func(tss []prompb.TimeSeries, mms []prompb.MetricMetadata) error) error {
	// Verify parsing without compression
	if err := ParseStream(bytes.NewBuffer(data), "", nil, checkSeries); err != nil {
		return fmt.Errorf("error when parsing data: %w", err)
	}

	// Verify parsing with gzip compression
	var bb bytes.Buffer
	var zw io.WriteCloser = gzip.NewWriter(&bb)
	if _, err := zw.Write(data); err != nil {
		return fmt.Errorf("cannot compress data: %w", err)
	}
	if err := zw.Close(); err != nil {
		return fmt.Errorf("cannot close gzip writer: %w", err)
	}
	if err := ParseStream(&bb, "gzip", nil, checkSeries); err != nil {
		return fmt.Errorf("error when parsing compressed data: %w", err)
	}

	// Verify parsing with zstd  compression
	zw, _ = zstd.NewWriter(&bb)
	if _, err := zw.Write(data); err != nil {
		return fmt.Errorf("cannot compress data: %w", err)
	}
	if err := zw.Close(); err != nil {
		return fmt.Errorf("cannot close zstd writer: %w", err)
	}
	if err := ParseStream(&bb, "zstd", nil, checkSeries); err != nil {
		return fmt.Errorf("error when parsing compressed data: %w", err)
	}

	return nil
}

func attributesFromKV(k, v string) []*pb.KeyValue {
	return []*pb.KeyValue{
		{
			Key: k,
			Value: &pb.AnyValue{
				StringValue: &v,
			},
		},
	}
}

func generateExpHistogram(name, unit string) *pb.Metric {
	sum := float64(4578)
	return &pb.Metric{
		Name: name,
		Unit: unit,
		ExponentialHistogram: &pb.ExponentialHistogram{
			AggregationTemporality: pb.AggregationTemporalityCumulative,
			DataPoints: []*pb.ExponentialHistogramDataPoint{
				{
					Attributes:   attributesFromKV("label1", "value1"),
					TimeUnixNano: uint64(15 * time.Second),
					Count:        20,
					Sum:          &sum,
					Scale:        7,
					Positive: &pb.Buckets{
						Offset:       7,
						BucketCounts: []uint64{0, 0, 0, 0, 5, 10, 0, 0, 1},
					},
				},
			},
		},
	}
}

func generateGauge(name, unit string) *pb.Metric {
	n := int64(15)
	points := []*pb.NumberDataPoint{
		{
			Attributes:   attributesFromKV("label1", "value1"),
			IntValue:     &n,
			TimeUnixNano: uint64(15 * time.Second),
		},
	}
	return &pb.Metric{
		Name:        name,
		Description: "I'm a gauge",
		Unit:        unit,
		Gauge: &pb.Gauge{
			DataPoints: points,
		},
	}
}

func generateGaugeUnknown(name, unit string) *pb.Metric {
	m := generateGauge(name, unit)
	m.Description = "I'm not a gauge"
	m.Metadata = append(m.Metadata, &pb.KeyValue{
		Key: "prometheus.type",
		Value: &pb.AnyValue{
			StringValue: ptrTo("unknown"),
		},
	})
	return m
}

func generateHistogram(name, unit string, hasSum bool) *pb.Metric {
	point := &pb.HistogramDataPoint{
		Attributes:     attributesFromKV("label2", "value2"),
		Count:          15,
		ExplicitBounds: []float64{0.1, 0.5, 1.0, 5.0},
		BucketCounts:   []uint64{0, 5, 10, 0, 0},
		TimeUnixNano:   uint64(30 * time.Second),
	}
	if hasSum {
		point.Sum = func() *float64 { v := 30.0; return &v }()
	}
	return &pb.Metric{
		Name:        name,
		Unit:        unit,
		Description: "I'm a Histogram",
		Histogram: &pb.Histogram{
			AggregationTemporality: pb.AggregationTemporalityCumulative,
			DataPoints:             []*pb.HistogramDataPoint{point},
		},
	}
}

func generateSum(name, unit string, isMonotonic bool) *pb.Metric {
	d := float64(15.5)
	points := []*pb.NumberDataPoint{
		{
			Attributes:   attributesFromKV("label5", "value5"),
			DoubleValue:  &d,
			TimeUnixNano: uint64(150 * time.Second),
		},
	}
	return &pb.Metric{
		Name:        name,
		Unit:        unit,
		Description: "I might be a counter or gauge, depending on the IsMonotonic",
		Sum: &pb.Sum{
			AggregationTemporality: pb.AggregationTemporalityCumulative,
			DataPoints:             points,
			IsMonotonic:            isMonotonic,
		},
	}
}

func generateSummary(name, unit string) *pb.Metric {
	points := []*pb.SummaryDataPoint{
		{
			Attributes:   attributesFromKV("label6", "value6"),
			TimeUnixNano: uint64(35 * time.Second),
			Sum:          32.5,
			Count:        5,
			QuantileValues: []*pb.ValueAtQuantile{
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
	return &pb.Metric{
		Name: name,
		Unit: unit,
		Summary: &pb.Summary{
			DataPoints: points,
		},
		Description: "I'm a Summary",
	}
}

func generateOTLPSamples(srcs []*pb.Metric) *pb.ResourceMetrics {
	otlpMetrics := &pb.ResourceMetrics{
		Resource: &pb.Resource{
			Attributes: attributesFromKV("job", "vm"),
		},
	}
	otlpMetrics.ScopeMetrics = []*pb.ScopeMetrics{
		{
			Metrics: append([]*pb.Metric{}, srcs...),
		},
	}
	return otlpMetrics
}

func newPromPBTs(metricName string, t int64, v float64, extraLabels ...prompb.Label) prompb.TimeSeries {
	if t <= 0 {
		// Set the current timestamp if t isn't set.
		t = int64(fasttime.UnixTimestamp()) * 1000
	}
	ts := prompb.TimeSeries{
		Labels: []prompb.Label{
			{
				Name:  "__name__",
				Value: metricName,
			},
		},
		Samples: []prompb.Sample{
			{
				Value:     v,
				Timestamp: t,
			},
		},
	}
	ts.Labels = append(ts.Labels, extraLabels...)
	return ts
}

func prettifyLabel(label prompb.Label) string {
	return fmt.Sprintf("name=%q value=%q", label.Name, label.Value)
}

func prettifySample(sample prompb.Sample) string {
	return fmt.Sprintf("sample=%f timestamp: %d", sample.Value, sample.Timestamp)
}

func sortByMetricName(tss []prompb.TimeSeries) {
	sort.Slice(tss, func(i, j int) bool {
		return getMetricName(tss[i].Labels) < getMetricName(tss[j].Labels)
	})
}

func getMetricName(labels []prompb.Label) string {
	for _, l := range labels {
		if l.Name == "__name__" {
			return l.Value
		}
	}
	return ""
}

func sortLabels(labels []prompb.Label) {
	sort.Slice(labels, func(i, j int) bool {
		return labels[i].Name < labels[j].Name
	})
}

func ptrTo[T any](v T) *T {
	return &v
}
