package stream

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fasttime"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/protoparser/opentelemetry/pb"
)

func TestParseMetricsStream(t *testing.T) {
	f := func(samples []*pb.Metric, tssExpected []prompbmarshal.TimeSeries, usePromNaming bool) {
		t.Helper()

		prevPromNaming := *usePrometheusNaming
		*usePrometheusNaming = usePromNaming
		defer func() {
			*usePrometheusNaming = prevPromNaming
		}()

		checkSeries := func(tss []prompbmarshal.TimeSeries) error {
			if len(tss) != len(tssExpected) {
				return fmt.Errorf("not expected tss count, got: %d, want: %d", len(tss), len(tssExpected))
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
			return nil
		}

		req := &pb.ExportMetricsServiceRequest{
			ResourceMetrics: []*pb.ResourceMetrics{
				generateOTLPSamples(samples),
			},
		}

		// Verify protobuf parsing
		pbData := req.MarshalProtobuf(nil)
		contentType := "application/x-protobuf"
		if err := checkParseMetricsStream(pbData, contentType, checkSeries); err != nil {
			t.Fatalf("cannot parse protobuf: %s", err)
		}
	}

	jobLabelValue := prompbmarshal.Label{
		Name:  "job",
		Value: "vm",
	}
	leLabel := func(value string) prompbmarshal.Label {
		return prompbmarshal.Label{
			Name:  "le",
			Value: value,
		}
	}
	kvLabel := func(k, v string) prompbmarshal.Label {
		return prompbmarshal.Label{
			Name:  k,
			Value: v,
		}
	}

	// Test all metric types
	f(
		[]*pb.Metric{
			generateGauge("my-gauge", ""),
			generateHistogram("my-histogram", ""),
			generateSum("my-sum", "", false),
			generateSummary("my-summary", ""),
		},
		[]prompbmarshal.TimeSeries{
			newPromPBTs("my-gauge", 15000, 15.0, jobLabelValue, kvLabel("label1", "value1")),
			newPromPBTs("my-histogram_count", 30000, 15.0, jobLabelValue, kvLabel("label2", "value2")),
			newPromPBTs("my-histogram_sum", 30000, 30.0, jobLabelValue, kvLabel("label2", "value2")),
			newPromPBTs("my-histogram_bucket", 30000, 0.0, jobLabelValue, kvLabel("label2", "value2"), leLabel("0.1")),
			newPromPBTs("my-histogram_bucket", 30000, 5.0, jobLabelValue, kvLabel("label2", "value2"), leLabel("0.5")),
			newPromPBTs("my-histogram_bucket", 30000, 15.0, jobLabelValue, kvLabel("label2", "value2"), leLabel("1")),
			newPromPBTs("my-histogram_bucket", 30000, 15.0, jobLabelValue, kvLabel("label2", "value2"), leLabel("5")),
			newPromPBTs("my-histogram_bucket", 30000, 15.0, jobLabelValue, kvLabel("label2", "value2"), leLabel("+Inf")),
			newPromPBTs("my-sum", 150000, 15.5, jobLabelValue, kvLabel("label5", "value5")),
			newPromPBTs("my-summary_sum", 35000, 32.5, jobLabelValue, kvLabel("label6", "value6")),
			newPromPBTs("my-summary_count", 35000, 5.0, jobLabelValue, kvLabel("label6", "value6")),
			newPromPBTs("my-summary", 35000, 7.5, jobLabelValue, kvLabel("label6", "value6"), kvLabel("quantile", "0.1")),
			newPromPBTs("my-summary", 35000, 10.0, jobLabelValue, kvLabel("label6", "value6"), kvLabel("quantile", "0.5")),
			newPromPBTs("my-summary", 35000, 15.0, jobLabelValue, kvLabel("label6", "value6"), kvLabel("quantile", "1")),
		},
		false,
	)

	// Test gauge
	f(
		[]*pb.Metric{
			generateGauge("my-gauge", ""),
		},
		[]prompbmarshal.TimeSeries{
			newPromPBTs("my-gauge", 15000, 15.0, jobLabelValue, kvLabel("label1", "value1")),
		},
		false,
	)

	// Test gauge with unit and prometheus naming
	f(
		[]*pb.Metric{
			generateGauge("my-gauge", "ms"),
		},
		[]prompbmarshal.TimeSeries{
			newPromPBTs("my_gauge_milliseconds", 15000, 15.0, jobLabelValue, kvLabel("label1", "value1")),
		},
		true,
	)

	// Test gauge with unit inside metric
	f(
		[]*pb.Metric{
			generateGauge("my-gauge-milliseconds", "ms"),
		},
		[]prompbmarshal.TimeSeries{
			newPromPBTs("my_gauge_milliseconds", 15000, 15.0, jobLabelValue, kvLabel("label1", "value1")),
		},
		true,
	)

	// Test gauge with ratio suffix
	f(
		[]*pb.Metric{
			generateGauge("my-gauge-milliseconds", "1"),
		},
		[]prompbmarshal.TimeSeries{
			newPromPBTs("my_gauge_milliseconds_ratio", 15000, 15.0, jobLabelValue, kvLabel("label1", "value1")),
		},
		true,
	)

	// Test sum with total suffix
	f(
		[]*pb.Metric{
			generateSum("my-sum", "ms", true),
		},
		[]prompbmarshal.TimeSeries{
			newPromPBTs("my_sum_milliseconds_total", 150000, 15.5, jobLabelValue, kvLabel("label5", "value5")),
		},
		true,
	)

	// Test sum with total suffix, which exists in a metric name
	f(
		[]*pb.Metric{
			generateSum("my-total-sum", "ms", true),
		},
		[]prompbmarshal.TimeSeries{
			newPromPBTs("my_sum_milliseconds_total", 150000, 15.5, jobLabelValue, kvLabel("label5", "value5")),
		},
		true,
	)

	// Test sum with total and complex suffix
	f(
		[]*pb.Metric{
			generateSum("my-total-sum", "m/s", true),
		},
		[]prompbmarshal.TimeSeries{
			newPromPBTs("my_sum_meters_per_second_total", 150000, 15.5, jobLabelValue, kvLabel("label5", "value5")),
		},
		true,
	)
}

func checkParseMetricsStream(data []byte, contentType string, checkSeries func(tss []prompbmarshal.TimeSeries) error) error {
	// Verify parsing without compression
	if err := ParseMetricsStream(bytes.NewBuffer(data), contentType, false, nil, checkSeries); err != nil {
		return fmt.Errorf("error when parsing data: %w", err)
	}

	// Verify parsing with compression
	var bb bytes.Buffer
	zw := gzip.NewWriter(&bb)
	if _, err := zw.Write(data); err != nil {
		return fmt.Errorf("cannot compress data: %w", err)
	}
	if err := zw.Close(); err != nil {
		return fmt.Errorf("cannot close gzip writer: %w", err)
	}
	if err := ParseMetricsStream(&bb, contentType, true, nil, checkSeries); err != nil {
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
		Name: name,
		Unit: unit,
		Gauge: &pb.Gauge{
			DataPoints: points,
		},
	}
}

func generateHistogram(name, unit string) *pb.Metric {
	points := []*pb.HistogramDataPoint{
		{

			Attributes:     attributesFromKV("label2", "value2"),
			Count:          15,
			Sum:            func() *float64 { v := 30.0; return &v }(),
			ExplicitBounds: []float64{0.1, 0.5, 1.0, 5.0},
			BucketCounts:   []uint64{0, 5, 10, 0, 0},
			TimeUnixNano:   uint64(30 * time.Second),
		},
	}
	return &pb.Metric{
		Name: name,
		Unit: unit,
		Histogram: &pb.Histogram{
			AggregationTemporality: pb.AggregationTemporalityCumulative,
			DataPoints:             points,
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
		Name: name,
		Unit: unit,
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

func newPromPBTs(metricName string, t int64, v float64, extraLabels ...prompbmarshal.Label) prompbmarshal.TimeSeries {
	if t <= 0 {
		// Set the current timestamp if t isn't set.
		t = int64(fasttime.UnixTimestamp()) * 1000
	}
	ts := prompbmarshal.TimeSeries{
		Labels: []prompbmarshal.Label{
			{
				Name:  "__name__",
				Value: metricName,
			},
		},
		Samples: []prompbmarshal.Sample{
			{
				Value:     v,
				Timestamp: t,
			},
		},
	}
	ts.Labels = append(ts.Labels, extraLabels...)
	return ts
}

func prettifyLabel(label prompbmarshal.Label) string {
	return fmt.Sprintf("name=%q value=%q", label.Name, label.Value)
}

func prettifySample(sample prompbmarshal.Sample) string {
	return fmt.Sprintf("sample=%f timestamp: %d", sample.Value, sample.Timestamp)
}

func sortByMetricName(tss []prompbmarshal.TimeSeries) {
	sort.Slice(tss, func(i, j int) bool {
		return getMetricName(tss[i].Labels) < getMetricName(tss[j].Labels)
	})
}

func getMetricName(labels []prompbmarshal.Label) string {
	for _, l := range labels {
		if l.Name == "__name__" {
			return l.Value
		}
	}
	return ""
}

func sortLabels(labels []prompbmarshal.Label) {
	sort.Slice(labels, func(i, j int) bool {
		return labels[i].Name < labels[j].Name
	})
}
