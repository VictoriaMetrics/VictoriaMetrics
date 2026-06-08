package pb

import (
	"reflect"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutil"
)

type testMetricPusher struct {
	samples  []testSample
	metadata []MetricMetadata
}

type testSample struct {
	mm     MetricMetadata
	suffix string
	labels []prompb.Label
	ts     uint64
	value  float64
}

func (p *testMetricPusher) PushSample(mm *MetricMetadata, suffix string, ls *promutil.Labels, timestampNsecs uint64, value float64, _ uint32) {
	labels := make([]prompb.Label, len(ls.Labels))
	copy(labels, ls.Labels)
	p.samples = append(p.samples, testSample{
		mm:     *mm,
		suffix: suffix,
		labels: labels,
		ts:     timestampNsecs,
		value:  value,
	})
}

func (p *testMetricPusher) PushMetricMetadata(mm *MetricMetadata) {
	p.metadata = append(p.metadata, *mm)
}

func TestDecodeScopeMetrics(t *testing.T) {
	scopeName := "my-scope"
	scopeVersion := "v1.0"
	envVal := "prod"
	intVal := int64(1)
	md := &MetricsData{
		ResourceMetrics: []*ResourceMetrics{
			{
				Resource: &Resource{
					Attributes: []*KeyValue{
						{Key: "job", Value: &AnyValue{StringValue: new("vm")}},
						{Key: "region", Value: &AnyValue{StringValue: new("us-east-1")}},
					},
				},
				ScopeMetrics: []*ScopeMetrics{
					{
						Scope: &InstrumentationScope{
							Name:    &scopeName,
							Version: &scopeVersion,
							Attributes: []*KeyValue{
								{Key: "env", Value: &AnyValue{StringValue: &envVal}},
							},
						},
						Metrics: []*Metric{
							{
								Name:        "my-gauge",
								Description: "a test gauge",
								Gauge: &Gauge{
									DataPoints: []*NumberDataPoint{
										{
											Attributes:   []*KeyValue{{Key: "label1", Value: &AnyValue{StringValue: new("value1")}}},
											IntValue:     &intVal,
											TimeUnixNano: 1000,
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
	data := md.MarshalProtobuf(nil)

	f := func(options DecodeMetricsOptions, wantLabels map[string]string) {
		t.Helper()
		mp := &testMetricPusher{}
		if err := DecodeMetricsData(data, mp, options); err != nil {
			t.Fatalf("DecodeMetricsData error: %v", err)
		}
		if len(mp.samples) != 1 {
			t.Fatalf("expected 1 sample, got %d", len(mp.samples))
		}
		gotMap := make(map[string]string, len(mp.samples[0].labels))
		for _, l := range mp.samples[0].labels {
			gotMap[l.Name] = l.Value
		}
		if !reflect.DeepEqual(gotMap, wantLabels) {
			t.Errorf("unexpected labels:\n got:  %v\n want: %v", gotMap, wantLabels)
		}
	}

	// PromoteScopeMetadata=true + PromoteAllResourceAttributes=true:
	// got all scope labels and resource attrs
	f(DecodeMetricsOptions{
		DisableScopeMetadata:      false,
		DisableResourceAttributes: false,
	}, map[string]string{
		"job":                  "vm",
		"region":               "us-east-1",
		"scope.name":           "my-scope",
		"scope.version":        "v1.0",
		"scope.attributes.env": "prod",
		"label1":               "value1",
	})

	// PromoteScopeMetadata=false + PromoteAllResourceAttributes=true:
	// got all resource attrs, no scope labels
	f(DecodeMetricsOptions{
		DisableScopeMetadata:      true,
		DisableResourceAttributes: false,
	}, map[string]string{
		"job":    "vm",
		"region": "us-east-1",
		"label1": "value1",
	})

	// PromoteScopeMetadata=true + PromoteAllResourceAttributes=false + ResourceAttributesList=[region]:
	// got only the `region` attr from resource
	f(DecodeMetricsOptions{
		DisableScopeMetadata:      false,
		DisableResourceAttributes: true,
		ResourceAttributesList:    map[string]struct{}{"region": {}},
	}, map[string]string{
		"region":               "us-east-1",
		"scope.name":           "my-scope",
		"scope.version":        "v1.0",
		"scope.attributes.env": "prod",
		"label1":               "value1",
	})

	// PromoteScopeMetadata=true + PromoteAllResourceAttributes=true + ResourceAttributesList=[region]:
	// got all resource attrs except `region` (ignore list)
	f(DecodeMetricsOptions{
		DisableScopeMetadata:      false,
		DisableResourceAttributes: false,
		ResourceAttributesList:    map[string]struct{}{"region": {}},
	}, map[string]string{
		"job":                  "vm",
		"scope.name":           "my-scope",
		"scope.version":        "v1.0",
		"scope.attributes.env": "prod",
		"label1":               "value1",
	})

	// PromoteScopeMetadata=false + PromoteAllResourceAttributes=false + ResourceAttributesList=[job]:
	// got only `job` attr
	f(DecodeMetricsOptions{
		DisableScopeMetadata:      true,
		DisableResourceAttributes: true,
		ResourceAttributesList:    map[string]struct{}{"job": {}},
	}, map[string]string{
		"job":    "vm",
		"label1": "value1",
	})
}
