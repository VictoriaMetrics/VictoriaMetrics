package tests

import (
	"net/http"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/apptest"
)

// Data used in tests
var testDataMemoryLimits = []string{
	`foo_bar{job="some_value"} 1.00`,
	`foo_bar{job="other_value"} 2.00`,
}

func TestMaxMemoryUsageExceeded(t *testing.T) {
	tc := apptest.NewTestCase(t)
	defer tc.Stop()

	querier := tc.MustStartCluster(&apptest.ClusterOptions{
		VminsertInstance:   "vminsert",
		VmselectInstance:   "vmselect",
		Vmstorage1Instance: "vmstorage1",
		Vmstorage2Instance: "vmstorage2",
		VmselectFlags:      []string{"-search.maxMemoryPerQuery=1"},
		VminsertFlags:      []string{"-clusternativeListenAddr=127.0.0.1:0"},
	})

	querier.PrometheusAPIV1ImportPrometheus(t, testDataMemoryLimits, apptest.QueryOpts{})

	querier.ForceFlush(t)

	// Test labels endpoint
	labelsResp := querier.PrometheusAPIV1Labels(t, apptest.QueryOpts{
		ExpectedResponseCode: http.StatusUnprocessableEntity,
	})
	if labelsResp.Status != "error" {
		t.Fatalf("unexpected status for labels; got %q, want %q", labelsResp.Status, "error")
	}

	// Test label values endpoint
	labelValuesResp := querier.PrometheusAPIV1LabelValues(t, "job", apptest.QueryOpts{
		ExpectedResponseCode: http.StatusUnprocessableEntity,
	})
	if labelValuesResp.Status != "error" {
		t.Fatalf("unexpected status for label values; got %q, want %q", labelValuesResp.Status, "error")
	}

	// Test series endpoint
	seriesResp := querier.PrometheusAPIV1Series(t, "foo_bar", apptest.QueryOpts{
		ExpectedResponseCode: http.StatusUnprocessableEntity,
	})
	if seriesResp.Status != "error" {
		t.Fatalf("unexpected status for series; got %q, want %q", seriesResp.Status, "error")
	}
}
