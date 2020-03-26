package provider

import (
	"reflect"
	"sort"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/config"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/datasource"
)

func TestAlertsFromMetrics(t *testing.T) {
	now := time.Now()
	metrics := []datasource.Metric{
		{
			Labels: []datasource.Label{
				{Name: "__name__", Value: "foo"},
				{Name: "label", Value: "value"},
			},
			Timestamp: 10,
			Value:     20,
		},
		{
			Labels: []datasource.Label{
				{Name: "__name__", Value: "bar"},
				{Name: "label", Value: "value"},
			},
			Timestamp: 10,
			Value:     30,
		},
	}
	rule := config.Rule{
		Name: "alertname",
		Expr: "up==0",
		Labels: map[string]string{
			"label2": "value",
		},
		Annotations: map[string]string{
			"tpl": "{{$value}} {{ $labels.label}}",
		},
	}
	alerts := AlertsFromMetrics(metrics, "group", rule, now, now)
	if len(alerts) != 2 {
		t.Fatalf("expecting 2 alerts got %d", len(alerts))
	}

	f := func(got, exp Alert) {
		t.Helper()
		if got.Group != exp.Group ||
			got.Value != exp.Value ||
			got.End != exp.End ||
			got.Name != exp.Name ||
			got.Start != exp.Start {
			t.Errorf("alerts are not equal: \nwant %#v \ngot  %#v", exp, got)
		}
		sort.Slice(got.Labels, func(i, j int) bool {
			return got.Labels[i].Name < got.Labels[j].Name
		})
		sort.Slice(exp.Labels, func(i, j int) bool {
			return got.Labels[i].Name < got.Labels[j].Name
		})
		if !reflect.DeepEqual(got.Labels, exp.Labels) {
			t.Errorf("alerts labels are not equal: want  %+v got %+v", exp.Labels, got.Labels)
		}
		if !reflect.DeepEqual(got.Annotations, exp.Annotations) {
			t.Errorf("alerts annotations are not equal: want %+v got %+v", exp.Annotations, got.Annotations)
		}
	}
	f(alerts[0], Alert{
		Group: "group",
		Name:  "alertname",
		Labels: []datasource.Label{
			{Name: "__name__", Value: "foo"},
			{Name: "label", Value: "value"},
			{Name: "label2", Value: "value"},
		},
		Annotations: map[string]string{
			"tpl": "20 value",
		},
		Start: now,
		End:   now,
		Value: 20,
	})
	f(alerts[1], Alert{
		Group: "group",
		Name:  "alertname",
		Labels: []datasource.Label{
			{Name: "__name__", Value: "bar"},
			{Name: "label", Value: "value"},
			{Name: "label2", Value: "value"},
		},
		Annotations: map[string]string{
			"tpl": "30 value",
		},
		Start: now,
		End:   now,
		Value: 30,
	})
}
