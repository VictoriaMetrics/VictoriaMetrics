package pb

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/flagutil"
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
	buildMetricsData := func() []byte {
		scopeName := "my-scope"
		scopeVersion := "v1.0"
		envVal := "prod"
		intVal := int64(1)
		md := &MetricsData{
			ResourceMetrics: []*ResourceMetrics{
				{
					Resource: &Resource{
						Attributes: []*KeyValue{
							{Key: "job", Value: &AnyValue{StringValue: strPtr("vm")}},
							{Key: "region", Value: &AnyValue{StringValue: strPtr("us-east-1")}},
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
												Attributes:   []*KeyValue{{Key: "label1", Value: &AnyValue{StringValue: strPtr("value1")}}},
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
		return md.MarshalProtobuf(nil)
	}

	decode := func(t *testing.T, data []byte) []prompb.Label {
		t.Helper()
		mp := &testMetricPusher{}
		if err := DecodeMetricsData(data, mp); err != nil {
			t.Fatalf("DecodeMetricsData error: %v", err)
		}
		if len(mp.samples) != 1 {
			t.Fatalf("expected 1 sample, got %d", len(mp.samples))
		}
		return mp.samples[0].labels
	}

	checkLabels := func(t *testing.T, got []prompb.Label, want map[string]string) {
		t.Helper()
		gotMap := make(map[string]string, len(got))
		for _, l := range got {
			gotMap[string([]byte(l.Name))] = string([]byte(l.Value))
		}
		if !reflect.DeepEqual(gotMap, want) {
			t.Errorf("unexpected labels:\n got:  %s\n want: %s", fmtLabelMap(gotMap), fmtLabelMap(want))
		}
	}

	// (default) promoteScopeMetadata=true + promoteAllResourceAttributes=true:
	// got all scope labels and resource attrs
	t.Run("scope_and_all_resource_attrs", func(t *testing.T) {
		labels := decode(t, buildMetricsData())
		checkLabels(t, labels, map[string]string{
			"job":                  "vm",
			"region":               "us-east-1",
			"scope.name":           "my-scope",
			"scope.version":        "v1.0",
			"scope.attributes.env": "prod",
			"label1":               "value1",
		})
	})

	// promoteScopeMetadata=false + promoteAllResourceAttributes=true:
	// got all resource attrs, no scope labels.
	t.Run("no_scope_all_resource_attrs", func(t *testing.T) {
		prevScope := *promoteScopeMetadata
		*promoteScopeMetadata = false
		defer func() { *promoteScopeMetadata = prevScope }()

		labels := decode(t, buildMetricsData())
		checkLabels(t, labels, map[string]string{
			"job":    "vm",
			"region": "us-east-1",
			"label1": "value1",
		})
	})

	// promoteScopeMetadata=true + promoteAllResourceAttributes=false + promoteResourceAttributes=[region]:
	// got the `region`` attr
	t.Run("scope_selected_resource_attrs", func(t *testing.T) {
		prevAll := *promoteAllResourceAttributes
		*promoteAllResourceAttributes = false
		defer func() { *promoteAllResourceAttributes = prevAll }()

		prevPromote := *promoteResourceAttributes
		*promoteResourceAttributes = flagutil.ArrayString{"region"}
		defer func() { *promoteResourceAttributes = prevPromote }()

		labels := decode(t, buildMetricsData())
		checkLabels(t, labels, map[string]string{
			"region":               "us-east-1",
			"scope.name":           "my-scope",
			"scope.version":        "v1.0",
			"scope.attributes.env": "prod",
			"label1":               "value1",
		})
	})

	// promoteScopeMetadata=true + promoteAllResourceAttributes=true + ignoreResourceAttributes=[region]:
	// got all resource attrs except `region`
	t.Run("scope_all_resource_attrs_ignore_region", func(t *testing.T) {
		prevIgnore := *ignoreResourceAttributes
		*ignoreResourceAttributes = flagutil.ArrayString{"region"}
		defer func() { *ignoreResourceAttributes = prevIgnore }()

		labels := decode(t, buildMetricsData())
		checkLabels(t, labels, map[string]string{
			"job":                  "vm",
			"scope.name":           "my-scope",
			"scope.version":        "v1.0",
			"scope.attributes.env": "prod",
			"label1":               "value1",
		})
	})

	// promoteScopeMetadata=false + promoteAllResourceAttributes=false + promoteResourceAttributes=[job]:
	// got only `job` attr
	t.Run("no_scope_selected_resource_attrs", func(t *testing.T) {
		prevScope := *promoteScopeMetadata
		*promoteScopeMetadata = false
		defer func() { *promoteScopeMetadata = prevScope }()

		prevAll := *promoteAllResourceAttributes
		*promoteAllResourceAttributes = false
		defer func() { *promoteAllResourceAttributes = prevAll }()

		prevPromote := *promoteResourceAttributes
		*promoteResourceAttributes = flagutil.ArrayString{"job"}
		defer func() { *promoteResourceAttributes = prevPromote }()

		labels := decode(t, buildMetricsData())
		checkLabels(t, labels, map[string]string{
			"job":    "vm",
			"label1": "value1",
		})
	})
}

func strPtr(s string) *string {
	return &s
}

func fmtLabelMap(m map[string]string) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, len(keys))
	for i, k := range keys {
		parts[i] = fmt.Sprintf("%s=%q", k, m[k])
	}
	return "{" + strings.Join(parts, ", ") + "}"
}
