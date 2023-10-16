package promql

import "testing"

func TestIsMetricSelectorWithRollup(t *testing.T) {
	childQuery, _, _ := IsMetricSelectorWithRollup("metric_name{label='value'}[365d] or vector(0)")
	if childQuery != "" {
		t.Fatalf("AAAAA: %v", childQuery)
	} else {
		t.Fatalf("BBBBB")
	}
}
