package tests

import (
	"fmt"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/VictoriaMetrics/VictoriaMetrics/apptest"
	at "github.com/VictoriaMetrics/VictoriaMetrics/apptest"
)

func TestSingleMetricNamesStats(t *testing.T) {
	os.RemoveAll(t.Name())
	tc := at.NewTestCase(t)
	defer tc.Stop()
	sut := tc.MustStartVmsingle("vmsingle", []string{"-storage.trackMetricNamesStats=true", "-retentionPeriod=100y"})

	const ingestDateTime = `2024-02-05T08:57:36.700Z`
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

	sut.PrometheusAPIV1ImportPrometheus(t, dataSet, at.QueryOpts{})
	sut.ForceFlush(t)

	// verify ingest request correctly registered
	expected := apptest.MetricNamesStatsResponse{
		Records: []at.MetricNamesStatsRecord{
			{MetricName: "metric_name_1"},
			{MetricName: "metric_name_2"},
			{MetricName: "metric_name_3"},
		},
	}
	got := sut.APIV1StatusMetricNamesStats(t, "", "", "", at.QueryOpts{})
	if diff := cmp.Diff(expected, got); diff != "" {
		t.Errorf("unexpected response (-want, +got):\n%s", diff)
	}

	// verify query request correctly registered
	sut.PrometheusAPIV1Query(t, `{__name__!=""}`, at.QueryOpts{Time: ingestDateTime})
	expected = apptest.MetricNamesStatsResponse{
		Records: []at.MetricNamesStatsRecord{
			{MetricName: "metric_name_1", QueryRequestsCount: 3},
			{MetricName: "metric_name_2", QueryRequestsCount: 1},
			{MetricName: "metric_name_3", QueryRequestsCount: 1},
		},
	}
	got = sut.APIV1StatusMetricNamesStats(t, "", "", "", at.QueryOpts{})
	if diff := cmp.Diff(expected, got); diff != "" {
		t.Errorf("unexpected response (-want, +got):\n%s", diff)
	}

	// perform query request for single metric and check counter increase
	sut.PrometheusAPIV1Query(t, `metric_name_2`, at.QueryOpts{Time: ingestDateTime})
	expected = apptest.MetricNamesStatsResponse{
		Records: []at.MetricNamesStatsRecord{
			{MetricName: "metric_name_1", QueryRequestsCount: 3},
			{MetricName: "metric_name_2", QueryRequestsCount: 2},
			{MetricName: "metric_name_3", QueryRequestsCount: 1},
		},
	}
	got = sut.APIV1StatusMetricNamesStats(t, "", "", "", at.QueryOpts{})
	if diff := cmp.Diff(expected, got); diff != "" {
		t.Errorf("unexpected response (-want, +got):\n%s", diff)
	}

	// verify le filter
	expected = apptest.MetricNamesStatsResponse{
		Records: []at.MetricNamesStatsRecord{
			{MetricName: "metric_name_2", QueryRequestsCount: 2},
			{MetricName: "metric_name_3", QueryRequestsCount: 1},
		},
	}
	got = sut.APIV1StatusMetricNamesStats(t, "", "2", "", at.QueryOpts{})
	if diff := cmp.Diff(expected, got); diff != "" {
		t.Errorf("unexpected response (-want, +got):\n%s", diff)
	}

	// reset state and check empty request response
	sut.APIV1AdminStatusMetricNamesStatsReset(t, at.QueryOpts{})
	expected = apptest.MetricNamesStatsResponse{
		Records: []at.MetricNamesStatsRecord{},
	}
	got = sut.APIV1StatusMetricNamesStats(t, "", "", "", at.QueryOpts{})
	if diff := cmp.Diff(expected, got); diff != "" {
		t.Errorf("unexpected response (-want, +got):\n%s", diff)
	}

}

func TestClusterMetricNamesStats(t *testing.T) {

	os.RemoveAll(t.Name())

	tc := apptest.NewTestCase(t)
	defer tc.Stop()
	vmstorage1 := tc.MustStartVmstorage("vmstorage-1", []string{
		"-storageDataPath=" + tc.Dir() + "/vmstorage-1",
		"-retentionPeriod=100y",
		"-storage.trackMetricNamesStats",
	})
	vmstorage2 := tc.MustStartVmstorage("vmstorage-2", []string{
		"-storageDataPath=" + tc.Dir() + "/vmstorage-2",
		"-retentionPeriod=100y",
		"-storage.trackMetricNamesStats",
	})

	vminsert := tc.MustStartVminsert("vminsert", []string{
		fmt.Sprintf("-storageNode=%s,%s", vmstorage1.VminsertAddr(), vmstorage2.VminsertAddr()),
	})
	vmselect := tc.MustStartVmselect("vmselect", []string{
		fmt.Sprintf("-storageNode=%s,%s", vmstorage1.VmselectAddr(), vmstorage2.VmselectAddr()),
	})
	// verify empty stats
	resp := vmselect.MetricNamesStats(t, "", "", "", apptest.QueryOpts{Tenant: "0:0"})
	if len(resp.Records) != 0 {
		t.Fatalf("unexpected resp Records: %d, want: %d", len(resp.Records), 0)
	}

	const ingestDateTime = `2024-02-05T08:57:36.700Z`
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

	// ingest per tenant data and verify it with search
	tenantIDs := []string{"1:1", "1:15", "15:15"}
	for _, tenantID := range tenantIDs {
		vminsert.PrometheusAPIV1ImportPrometheus(t, dataSet, apptest.QueryOpts{Tenant: tenantID})
		vmstorage1.ForceFlush(t)
		vmstorage2.ForceFlush(t)

		// verify ingest request correctly registered
		expected := apptest.MetricNamesStatsResponse{
			Records: []at.MetricNamesStatsRecord{
				{MetricName: "metric_name_1"},
				{MetricName: "metric_name_2"},
				{MetricName: "metric_name_3"},
			},
		}
		gotStats := vmselect.MetricNamesStats(t, "", "", "", apptest.QueryOpts{Tenant: tenantID})
		if diff := cmp.Diff(expected, gotStats); diff != "" {
			t.Errorf("unexpected response (-want, +got):\n%s", diff)
		}

		// verify query request registered correctly
		vmselect.PrometheusAPIV1Query(t, `{__name__!=""}`, apptest.QueryOpts{
			Tenant: tenantID, Time: ingestDateTime,
		})

		expected = apptest.MetricNamesStatsResponse{
			Records: []at.MetricNamesStatsRecord{
				{MetricName: "metric_name_2", QueryRequestsCount: 1},
				{MetricName: "metric_name_3", QueryRequestsCount: 1},
				{MetricName: "metric_name_1", QueryRequestsCount: 3},
			},
		}
		gotStats = vmselect.MetricNamesStats(t, "", "", "", apptest.QueryOpts{Tenant: tenantID})
		if diff := cmp.Diff(expected, gotStats); diff != "" {
			t.Errorf("unexpected response tenant: %s (-want, +got):\n%s", tenantID, diff)
		}
	}

	// verify multitenant stats
	expected := apptest.MetricNamesStatsResponse{
		Records: []at.MetricNamesStatsRecord{
			{MetricName: "metric_name_2", QueryRequestsCount: 3},
			{MetricName: "metric_name_3", QueryRequestsCount: 3},
			{MetricName: "metric_name_1", QueryRequestsCount: 9},
		},
	}
	gotStats := vmselect.MetricNamesStats(t, "", "", "", apptest.QueryOpts{Tenant: "multitenant"})
	if diff := cmp.Diff(expected, gotStats); diff != "" {
		t.Errorf("unexpected response (-want, +got):\n%s", diff)
	}

	// reset cache and check empty state
	vmselect.MetricNamesStatsReset(t, at.QueryOpts{})
	resp = vmselect.MetricNamesStats(t, "", "", "", apptest.QueryOpts{Tenant: "multitenant"})
	if len(resp.Records) != 0 {
		t.Fatalf("want 0 records, got: %d", len(resp.Records))
	}
}
