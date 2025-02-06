package notifier

import (
	"context"
	"fmt"
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
		t.Fatalf("unexpected error %s", err)
	}

	alertCount := bh.metrics.alertsSent.Get()
	if alertCount != 1 {
		t.Fatalf("expect value 1; instead got %d", alertCount)
	}
}

func TestBlackHoleNotifier_Close(t *testing.T) {
	addr := "blackhole-close"
	bh := newBlackHoleNotifier()
	bh.addr = addr
	if err := bh.Send(context.Background(), []Alert{{
		GroupID:     0,
		Name:        "alert1",
		Start:       time.Now().UTC(),
		End:         time.Now().UTC(),
		Annotations: map[string]string{"a": "b", "c": "d", "e": "f"},
	}}, nil); err != nil {
		t.Fatalf("unexpected error %s", err)
	}

	bh.Close()

	defaultMetrics := metricset.GetDefaultSet()
	alertMetricName := fmt.Sprintf("vmalert_alerts_sent_total{addr=%q}", addr)
	for _, name := range defaultMetrics.ListMetricNames() {
		if name == alertMetricName {
			t.Fatalf("Metric name should have unregistered. But still present")
		}
	}
}
