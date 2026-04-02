//go:build synctest

package rule

import (
	"context"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/datasource"
	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmalert/notifier"
)

// TestAlertingRule_ActiveAtPreservedInAnnotations ensures that the fix for
// https://github.com/VictoriaMetrics/VictoriaMetrics/issues/9543 is preserved
// while allowing query templates in labels (https://github.com/VictoriaMetrics/VictoriaMetrics/issues/9783)
func TestAlertingRule_ActiveAtPreservedInAnnotations(t *testing.T) {
	// wrap into synctest because of time manipulations
	synctest.Test(t, func(t *testing.T) {
		fq := &datasource.FakeQuerier{}

		ar := &AlertingRule{
			Name: "TestActiveAtPreservation",
			Labels: map[string]string{
				"test_query_in_label": `{{ "static_value" }}`,
			},
			Annotations: map[string]string{
				"description": "Alert active since {{ $activeAt }}",
			},
			alerts: make(map[uint64]*notifier.Alert),
			q:      fq,
			state: &ruleState{
				entries: make([]StateEntry, 10),
			},
		}

		// Mock query result - return empty result to make suppress_for_mass_alert = false
		// (no need to add anything to fq for empty result)

		// Add a metric that should trigger the alert
		fq.Add(metricWithValueAndLabels(t, 1, "instance", "server1"))

		// First execution - creates new alert
		ts1 := time.Now()
		_, err := ar.exec(context.TODO(), ts1, 0)
		if err != nil {
			t.Fatalf("unexpected error on first exec: %s", err)
		}

		if len(ar.alerts) != 1 {
			t.Fatalf("expected 1 alert, got %d", len(ar.alerts))
		}

		firstAlert := ar.GetAlerts()[0]
		// Verify first execution: activeAt should be ts1 and annotation should reflect it
		if !firstAlert.ActiveAt.Equal(ts1) {
			t.Fatalf("expected activeAt to be %v, got %v", ts1, firstAlert.ActiveAt)
		}

		// Extract time from annotation (format will be like "Alert active since 2025-09-30 08:55:13.638551611 -0400 EDT m=+0.002928464")
		expectedTimeStr := ts1.Format("2006-01-02 15:04:05")
		if !strings.Contains(firstAlert.Annotations["description"], expectedTimeStr) {
			t.Fatalf("first exec annotation should contain time %s, got: %s", expectedTimeStr, firstAlert.Annotations["description"])
		}

		// Second execution - should preserve activeAt in annotation

		// Ensure different timestamp with different seconds
		// sleep is non-blocking thanks to synctest
		time.Sleep(2 * time.Second)
		ts2 := time.Now()
		_, err = ar.exec(context.TODO(), ts2, 0)
		if err != nil {
			t.Fatalf("unexpected error on second exec: %s", err)
		}

		// Get the alert again (should be the same alert)
		if len(ar.alerts) != 1 {
			t.Fatalf("expected 1 alert, got %d", len(ar.alerts))
		}
		secondAlert := ar.GetAlerts()[0]

		// Critical test: activeAt should still be ts1, not ts2
		if !secondAlert.ActiveAt.Equal(ts1) {
			t.Fatalf("activeAt should be preserved as %v, but got %v", ts1, secondAlert.ActiveAt)
		}

		// Critical test: annotation should still contain ts1 time, not ts2
		if !strings.Contains(secondAlert.Annotations["description"], expectedTimeStr) {
			t.Fatalf("second exec annotation should still contain original time %s, got: %s", expectedTimeStr, secondAlert.Annotations["description"])
		}

		// Additional verification: annotation should NOT contain ts2 time
		ts2TimeStr := ts2.Format("2006-01-02 15:04:05")
		if strings.Contains(secondAlert.Annotations["description"], ts2TimeStr) {
			t.Fatalf("annotation should NOT contain new eval time %s, got: %s", ts2TimeStr, secondAlert.Annotations["description"])
		}

		// Verify query template in labels still works (this would fail if query templates were broken)
		if firstAlert.Labels["test_query_in_label"] != "static_value" {
			t.Fatalf("expected test_query_in_label=static_value, got %s", firstAlert.Labels["test_query_in_label"])
		}
	})
}
