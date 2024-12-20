package tests

import (
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/apptest"
)

// Data used in tests
var testData = []string{
	"foo_bar 1.00 1652169600000", // 2022-05-10T08:00:00Z
	"foo_bar 2.00 1652169660000", // 2022-05-10T08:01:00Z
	"foo_bar 3.00 1652169720000", // 2022-05-10T08:02:00Z
	"foo_bar 5.00 1652169840000", // 2022-05-10T08:04:00Z, one point missed
	"foo_bar 5.50 1652169960000", // 2022-05-10T08:06:00Z, one point missed
	"foo_bar 5.50 1652170020000", // 2022-05-10T08:07:00Z
	"foo_bar 4.00 1652170080000", // 2022-05-10T08:08:00Z
	"foo_bar 3.50 1652170260000", // 2022-05-10T08:11:00Z, two points missed
	"foo_bar 3.25 1652170320000", // 2022-05-10T08:12:00Z
	"foo_bar 3.00 1652170380000", // 2022-05-10T08:13:00Z
	"foo_bar 2.00 1652170440000", // 2022-05-10T08:14:00Z
	"foo_bar 1.00 1652170500000", // 2022-05-10T08:15:00Z
	"foo_bar 4.00 1652170560000", // 2022-05-10T08:16:00Z
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

func TestSingleMaxIngestionRateDoesNotIncrementsMetric(t *testing.T) {
	tc := apptest.NewTestCase(t)
	defer tc.Stop()
	sut := tc.MustStartVmsingle("vmsingle", []string{"-maxIngestionRate=15"})
	sut.PrometheusAPIV1ImportPrometheus(t, testData, apptest.QueryOpts{})
	if got, want := sut.GetMetric(t, "vm_max_ingestion_rate_limit_reached_total"), 0.0; got != want {
		t.Fatalf("Unexpected vm_max_ingestion_rate_limit_reached_total: got %f, want >0", got)
	}
}
