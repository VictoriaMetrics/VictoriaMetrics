package utils

import (
	"testing"

	"github.com/VictoriaMetrics/metrics"
)

func isMetricRegistered(name string) bool {
	metricNames := metrics.GetDefaultSet().ListMetricNames()
	for _, mn := range metricNames {
		if mn == name {
			return true
		}
	}

	return false
}

func TestMetricIsUnregistered(t *testing.T) {
	metricName := "example_runs_total"
	c := GetOrCreateCounter(metricName)
	if !isMetricRegistered(metricName) {
		t.Errorf("Expected metric %s to be present", metricName)
	}

	c.Unregister()
	if isMetricRegistered(metricName) {
		t.Errorf("Expected metric %s to be unregistered", metricName)
	}
}

func TestMetricIsRemovedIfNoUses(t *testing.T) {
	metricName := "example_runs_total"
	c := GetOrCreateCounter(metricName)
	c2 := GetOrCreateCounter(metricName)

	if !isMetricRegistered(metricName) {
		t.Errorf("Expected metric %s to be present", metricName)
	}

	c.Unregister()
	// metric should still be registered since c2 is using it
	if !isMetricRegistered(metricName) {
		t.Errorf("Expected metric %s to be present", metricName)
	}

	c2.Unregister()
	if isMetricRegistered(metricName) {
		t.Errorf("Expected metric %s to be unregistered", metricName)
	}
}
