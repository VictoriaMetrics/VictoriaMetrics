package opentelemetry

import (
	"bytes"
	"fmt"
	"testing"

	"google.golang.org/protobuf/proto"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
	colmetricpb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	v11 "go.opentelemetry.io/proto/otlp/common/v1"
	metricpb "go.opentelemetry.io/proto/otlp/metrics/v1"
	metricsv1 "go.opentelemetry.io/proto/otlp/metrics/v1"
)

func TestParseStream(t *testing.T) {
	f := func(name string, isJson bool, tss []prompb.TimeSeries) {
		t.Run(name, func(t *testing.T) {
			pbRequest := colmetricpb.ExportMetricsServiceRequest{
				ResourceMetrics: []*metricpb.ResourceMetrics{generateOTLPSamples(tss)},
			}

			data, err := proto.Marshal(&pbRequest)
			if err != nil {
				t.Fatalf("cannot marshal data: %s", err)
			}
			fmt.Println("data ", len(data))
			bb := bytes.NewBuffer(data)
			err = ParseStream(bb, isJson, false, func(tss []prompb.TimeSeries) error {
				fmt.Println("got ts: ", len(tss))
				return nil
			})
			if err != nil {
				t.Fatalf("unexpected error: %s", err)
			}
		})
	}
	f("test single metric", false, []prompb.TimeSeries{
		{
			Labels:  []prompb.Label{{Name: metricNameLabel, Value: []byte(`metric1`)}, {Name: []byte(`job`), Value: []byte(`vmsingle`)}},
			Samples: []prompb.Sample{{Value: 15.1, Timestamp: 164}},
		},
	})

}

func generateOTLPSamples(timeseries []prompb.TimeSeries) *metricpb.ResourceMetrics {
	otlpMetrics := &metricpb.ResourceMetrics{}
	metrics := []*metricsv1.Metric{}
	for _, ts := range timeseries {
		name := ""
		otlpLabels := make([]*v11.KeyValue, 0, len(ts.Labels))
		for _, l := range ts.Labels {
			if string(l.Name) == "__name__" {
				name = string(l.Value)
				continue
			}

			otlpLabels = append(otlpLabels, &v11.KeyValue{
				Key:   string(l.Name),
				Value: &v11.AnyValue{Value: &v11.AnyValue_StringValue{StringValue: string(l.Value)}},
			})
		}

		for _, sample := range ts.Samples {
			metrics = append(metrics, &metricsv1.Metric{
				Name: name,
				Data: &metricsv1.Metric_Gauge{
					Gauge: &metricpb.Gauge{
						DataPoints: []*metricpb.NumberDataPoint{
							{
								Attributes:   otlpLabels,
								TimeUnixNano: uint64(sample.Timestamp) * 1000,
								Value: &metricsv1.NumberDataPoint_AsDouble{
									AsDouble: sample.Value,
								},
							},
						},
					},
				},
			})
		}
	}
	otlpMetrics.ScopeMetrics = []*metricsv1.ScopeMetrics{
		{
			Metrics: metrics,
		},
	}
	return otlpMetrics
}
