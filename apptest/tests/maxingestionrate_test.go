package tests

import (
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/apptest"
)

// Data used in tests
var testData = []string{
	"foo_bar 1.00",
	"foo_bar 2.00",
}

func TestSingleMaxIngestionRateIncrementsMetric(t *testing.T) {
	tc := apptest.NewTestCase(t)
	defer tc.Stop()
	sut := tc.MustStartVmsingle("vmsingle", []string{"-maxIngestionRate=1"})
	sut.PrometheusAPIV1ImportPrometheus(t, testData, apptest.QueryOpts{})
	if got := sut.GetMetric(t, "vm_max_ingestion_rate_limit_reached_total"); got <= 0 {
		t.Fatalf("Unexpected vm_max_ingestion_rate_limit_reached_total: got %f, want >0", got)
	}
}

func TestSingleMaxIngestionRateDoesNotIncrementMetric(t *testing.T) {
	tc := apptest.NewTestCase(t)
	defer tc.Stop()
	sut := tc.MustStartVmsingle("vmsingle", []string{"-maxIngestionRate=15"})
	sut.PrometheusAPIV1ImportPrometheus(t, testData, apptest.QueryOpts{})
	if got, want := sut.GetMetric(t, "vm_max_ingestion_rate_limit_reached_total"), 0.0; got != want {
		t.Fatalf("Unexpected vm_max_ingestion_rate_limit_reached_total: got %f, want >0", got)
	}
}
