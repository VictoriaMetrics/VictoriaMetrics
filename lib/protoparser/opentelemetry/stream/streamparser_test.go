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

func TestParseStream(t *testing.T) {
	f := func(samples []*pb.Metric, tssExpected []prompbmarshal.TimeSeries) {
		t.Helper()

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
		pbData, err := req.MarshalVT()
		if err != nil {
			t.Fatalf("cannot marshal to protobuf: %s", err)
		}
		if err := checkParseStream(pbData, checkSeries); err != nil {
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
			generateGauge("my-gauge"),
			generateHistogram("my-histogram"),
			generateSum("my-sum"),
			generateSummary("my-summary"),
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
		})

	// Test gauge
	f(
		[]*pb.Metric{
			generateGauge("my-gauge"),
		},
		[]prompbmarshal.TimeSeries{
			newPromPBTs("my-gauge", 15000, 15.0, jobLabelValue, kvLabel("label1", "value1")),
		},
	)
}

func checkParseStream(data []byte, checkSeries func(tss []prompbmarshal.TimeSeries) error) error {
	// Verify parsing without compression
	if err := ParseStream(bytes.NewBuffer(data), false, checkSeries); err != nil {
		return fmt.Errorf("error when parsing data: %w", err)
	}

	// Verify parsing with compression
	var bb bytes.Buffer
	zw := gzip.NewWriter(&bb)
	if _, err := zw.Write(data); err != nil {
		return fmt.Errorf("cannot compress data: %s", err)
	}
	if err := zw.Close(); err != nil {
		return fmt.Errorf("cannot close gzip writer: %s", err)
	}
	if err := ParseStream(&bb, true, checkSeries); err != nil {
		return fmt.Errorf("error when parsing compressed data: %w", err)
	}

	return nil
}

func attributesFromKV(k, v string) []*pb.KeyValue {
	return []*pb.KeyValue{
		{
			Key: k,
			Value: &pb.AnyValue{
				Value: &pb.AnyValue_StringValue{
					StringValue: v,
				},
			},
		},
	}
}

func generateGauge(name string) *pb.Metric {
	points := []*pb.NumberDataPoint{
		{
			Attributes:   attributesFromKV("label1", "value1"),
			Value:        &pb.NumberDataPoint_AsInt{AsInt: 15},
			TimeUnixNano: uint64(15 * time.Second),
		},
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

func generateHistogram(name string) *pb.Metric {
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
		Data: &pb.Metric_Histogram{
			Histogram: &pb.Histogram{
				AggregationTemporality: pb.AggregationTemporality_AGGREGATION_TEMPORALITY_CUMULATIVE,
				DataPoints:             points,
			},
		},
	}
}

func generateSum(name string) *pb.Metric {
	points := []*pb.NumberDataPoint{
		{
			Attributes:   attributesFromKV("label5", "value5"),
			Value:        &pb.NumberDataPoint_AsDouble{AsDouble: 15.5},
			TimeUnixNano: uint64(150 * time.Second),
		},
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

func generateSummary(name string) *pb.Metric {
	points := []*pb.SummaryDataPoint{
		{
			Attributes:   attributesFromKV("label6", "value6"),
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
	return &pb.Metric{
		Name: name,
		Data: &pb.Metric_Summary{
			Summary: &pb.Summary{
				DataPoints: points,
			},
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
