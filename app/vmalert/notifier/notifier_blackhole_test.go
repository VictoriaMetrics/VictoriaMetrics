package notifier

import (
	"context"
	"testing"
	"time"

	metricset "github.com/VictoriaMetrics/metrics"
)

func TestBlackHoleNotifier_Send(t *testing.T) {
	bh := newBlackHoleNotifier()
	if err := bh.Send(context.Background(), []Alert{{
		GroupID:     0,
		Name:        "alert0",
		Start:       time.Now().UTC(),
		End:         time.Now().UTC(),
		Annotations: map[string]string{"a": "b", "c": "d", "e": "f"},
	}}, nil); err != nil {
		t.Errorf("unexpected error %s", err)
	}

	alertCount := bh.metrics.alertsSent.Get()
	if alertCount != 1 {
		t.Errorf("expect value 1; instead got %d", alertCount)
	}
}

func TestBlackHoleNotifier_Close(t *testing.T) {
	bh := newBlackHoleNotifier()
	if err := bh.Send(context.Background(), []Alert{{
		GroupID:     0,
		Name:        "alert0",
		Start:       time.Now().UTC(),
		End:         time.Now().UTC(),
		Annotations: map[string]string{"a": "b", "c": "d", "e": "f"},
	}}, nil); err != nil {
		t.Errorf("unexpected error %s", err)
	}

	bh.Close()

	defaultMetrics := metricset.GetDefaultSet()
	alertMetricName := "vmalert_alerts_sent_total{addr=\"blackhole\"}"
	for _, name := range defaultMetrics.ListMetricNames() {
		if name == alertMetricName {
			t.Errorf("Metric name should have unregistered.But still present")
		}
	}
}
