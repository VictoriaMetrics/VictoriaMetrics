package tests

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/VictoriaMetrics/VictoriaMetrics/apptest"
)

func testMetricsIndex(t *testing.T, sut apptest.PrometheusWriteQuerier) {
	// verify index is empty at the start
	expected := apptest.GraphiteMetricsIndexResponse{}
	tenant := "1:2"
	got := sut.GraphiteMetricsIndex(t, apptest.QueryOpts{Tenant: tenant})
	if diff := cmp.Diff(expected, got); diff != "" {
		t.Errorf("unexpected response (-want, +got):\n%s", diff)
	}

	// Mon Feb  5 09:57:36 CET 2024
	const ingestTimestamp = ` 1707123456700`
	dataSet := []string{
		`metric_name_1{label="foo"} 10`,
		`metric_name_1{label="bar"} 10`,
		`metric_name_2{label="baz"} 20`,
		`metric_name_1{label="baz"} 10`,
		`metric_name_3{label="baz"} 30`,
	}

	for idx := range dataSet {
		dataSet[idx] += ingestTimestamp
	}

	sut.PrometheusAPIV1ImportPrometheus(t, dataSet, apptest.QueryOpts{Tenant: tenant})
	sut.ForceFlush(t)

	// verify ingested metrics correctly returned in index response
	expected = []string{"metric_name_1", "metric_name_2", "metric_name_3"}

	got = sut.GraphiteMetricsIndex(t, apptest.QueryOpts{Tenant: tenant})
	if diff := cmp.Diff(expected, got); diff != "" {
		t.Errorf("unexpected response (-want, +got):\n%s", diff)
	}
}

func TestSingleMetricsIndex(t *testing.T) {
	tc := apptest.NewTestCase(t)
	defer tc.Stop()

	sut := tc.MustStartDefaultVmsingle()

	testMetricsIndex(tc.T(), sut)
}

func TestClusterMetricsIndex(t *testing.T) {
	tc := apptest.NewTestCase(t)
	defer tc.Stop()

	sut := tc.MustStartDefaultCluster()

	testMetricsIndex(tc.T(), sut)
}
