package rule

import (
	"fmt"
	"reflect"
	"sort"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/datasource"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/notifier"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
)

// CompareRules is a test helper func for other tests
func CompareRules(t *testing.T, a, b Rule) error {
	t.Helper()
	switch v := a.(type) {
	case *AlertingRule:
		br, ok := b.(*AlertingRule)
		if !ok {
			return fmt.Errorf("rule %q supposed to be of type AlertingRule", b.ID())
		}
		return compareAlertingRules(t, v, br)
	case *RecordingRule:
		br, ok := b.(*RecordingRule)
		if !ok {
			return fmt.Errorf("rule %q supposed to be of type RecordingRule", b.ID())
		}
		return compareRecordingRules(t, v, br)
	default:
		return fmt.Errorf("unexpected rule type received %T", a)
	}
}

func compareRecordingRules(t *testing.T, a, b *RecordingRule) error {
	t.Helper()
	if a.Expr != b.Expr {
		return fmt.Errorf("expected to have expression %q; got %q", a.Expr, b.Expr)
	}
	if !reflect.DeepEqual(a.Labels, b.Labels) {
		return fmt.Errorf("expected to have labels %#v; got %#v", a.Labels, b.Labels)
	}
	return nil
}

func compareAlertingRules(t *testing.T, a, b *AlertingRule) error {
	t.Helper()
	if a.Expr != b.Expr {
		return fmt.Errorf("expected to have expression %q; got %q", a.Expr, b.Expr)
	}
	if a.For != b.For {
		return fmt.Errorf("expected to have for %q; got %q", a.For, b.For)
	}
	if a.KeepFiringFor != b.KeepFiringFor {
		return fmt.Errorf("expected to have KeepFiringFor %q; got %q", a.KeepFiringFor, b.KeepFiringFor)
	}
	if !reflect.DeepEqual(a.Annotations, b.Annotations) {
		return fmt.Errorf("expected to have annotations %#v; got %#v", a.Annotations, b.Annotations)
	}
	if !reflect.DeepEqual(a.Labels, b.Labels) {
		return fmt.Errorf("expected to have labels %#v; got %#v", a.Labels, b.Labels)
	}
	if a.Type.String() != b.Type.String() {
		return fmt.Errorf("expected to have Type %#v; got %#v", a.Type.String(), b.Type.String())
	}
	return nil
}

func metricWithValueAndLabels(t *testing.T, value float64, labels ...string) datasource.Metric {
	return metricWithValuesAndLabels(t, []float64{value}, labels...)
}

func metricWithValuesAndLabels(t *testing.T, values []float64, labels ...string) datasource.Metric {
	t.Helper()
	m := metricWithLabels(t, labels...)
	m.Values = values
	for i := range values {
		m.Timestamps = append(m.Timestamps, int64(i))
	}
	return m
}

func metricWithLabels(t *testing.T, labels ...string) datasource.Metric {
	t.Helper()
	if len(labels) == 0 || len(labels)%2 != 0 {
		t.Fatalf("expected to get even number of labels")
	}
	m := datasource.Metric{Values: []float64{1}, Timestamps: []int64{1}}
	for i := 0; i < len(labels); i += 2 {
		m.Labels = append(m.Labels, datasource.Label{
			Name:  labels[i],
			Value: labels[i+1],
		})
	}
	return m
}

func toPromLabels(t testing.TB, labels ...string) []prompbmarshal.Label {
	t.Helper()
	if len(labels) == 0 || len(labels)%2 != 0 {
		t.Fatalf("expected to get even number of labels")
	}
	var ls []prompbmarshal.Label
	for i := 0; i < len(labels); i += 2 {
		ls = append(ls, prompbmarshal.Label{
			Name:  labels[i],
			Value: labels[i+1],
		})
	}
	return ls
}

func compareTimeSeries(t *testing.T, a, b []prompbmarshal.TimeSeries) error {
	t.Helper()
	if len(a) != len(b) {
		return fmt.Errorf("expected number of timeseries %d; got %d", len(a), len(b))
	}
	for i := range a {
		expTS, gotTS := a[i], b[i]
		if len(expTS.Samples) != len(gotTS.Samples) {
			return fmt.Errorf("expected number of samples %d; got %d", len(expTS.Samples), len(gotTS.Samples))
		}
		for i, exp := range expTS.Samples {
			got := gotTS.Samples[i]
			if got.Value != exp.Value {
				return fmt.Errorf("expected value %.2f; got %.2f", exp.Value, got.Value)
			}
			// timestamp validation isn't always correct for now.
			// this must be improved with time mock.
			/*if got.Timestamp != exp.Timestamp {
				return fmt.Errorf("expected timestamp %d; got %d", exp.Timestamp, got.Timestamp)
			}*/
		}
		if len(expTS.Labels) != len(gotTS.Labels) {
			return fmt.Errorf("expected number of labels %d (%v); got %d (%v)",
				len(expTS.Labels), expTS.Labels, len(gotTS.Labels), gotTS.Labels)
		}
		for i, exp := range expTS.Labels {
			got := gotTS.Labels[i]
			if got.Name != exp.Name {
				return fmt.Errorf("expected label name %q; got %q", exp.Name, got.Name)
			}
			if got.Value != exp.Value {
				return fmt.Errorf("expected label value %q; got %q", exp.Value, got.Value)
			}
		}
	}
	return nil
}

func compareAlerts(t *testing.T, as, bs []notifier.Alert) {
	t.Helper()
	if len(as) != len(bs) {
		t.Fatalf("expected to have length %d; got %d", len(as), len(bs))
	}
	sort.Slice(as, func(i, j int) bool {
		return as[i].ID < as[j].ID
	})
	sort.Slice(bs, func(i, j int) bool {
		return bs[i].ID < bs[j].ID
	})
	for i := range as {
		a, b := as[i], bs[i]
		if a.Name != b.Name {
			t.Fatalf("expected t have Name %q; got %q", a.Name, b.Name)
		}
		if a.State != b.State {
			t.Fatalf("expected t have State %q; got %q", a.State, b.State)
		}
		if a.Value != b.Value {
			t.Fatalf("expected t have Value %f; got %f", a.Value, b.Value)
		}
		if !reflect.DeepEqual(a.Annotations, b.Annotations) {
			t.Fatalf("expected to have annotations %#v; got %#v", a.Annotations, b.Annotations)
		}
		if !reflect.DeepEqual(a.Labels, b.Labels) {
			t.Fatalf("expected to have labels %#v; got %#v", a.Labels, b.Labels)
		}
	}
}
