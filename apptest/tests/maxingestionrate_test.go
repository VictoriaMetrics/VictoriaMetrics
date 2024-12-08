package tests

import (
	"github.com/VictoriaMetrics/VictoriaMetrics/apptest"
	"testing"
)

func TestSingleMaxIngestionRateIncrementsMetric(t *testing.T) {
	tc := apptest.NewTestCase(t)
	defer tc.Stop()
	sut := tc.MustStartVmsingle("vmsingle", []string{"-maxIngestionRate=5"})
	sut.PrometheusAPIV1ImportPrometheus(t, docData, apptest.QueryOpts{})
	maxIngestionRateMetric := sut.GetMetric(t, "vm_max_ingestion_rate_limit_reached_total")
	if maxIngestionRateMetric <= 0 {
		t.Errorf("Max Ingestion Rate metric not set unexpectedly")
	} else {
		t.Logf("MAX INGEST RATE HIT %d times", int(maxIngestionRateMetric))
	}
	sut.ForceFlush(t)
}
func TestSingleMaxIngestionRateDoesNotIncrementsMetric(t *testing.T) {
	tc := apptest.NewTestCase(t)
	defer tc.Stop()
	sut := tc.MustStartVmsingle("vmsingle", []string{"-maxIngestionRate=15"})
	sut.PrometheusAPIV1ImportPrometheus(t, docData, apptest.QueryOpts{})
	maxIngestionRateMetric := sut.GetMetric(t, "vm_max_ingestion_rate_limit_reached_total")
	if maxIngestionRateMetric > 0 {
		t.Errorf("Max Ingestion Rate set")
	} else {
		t.Logf("MAX INGEST RATE NOT SET as expected")
	}
	sut.ForceFlush(t)
}
