package tests

import (
	"fmt"
	"path/filepath"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/apptest"
	"github.com/google/go-cmp/cmp"
)

func TestMixedPrometheusQueries(t *testing.T) {
	tc := apptest.NewTestCase(t)
	defer tc.Stop()

	const (
		accountID1 = 12
		projectID1 = 34
		accountID2 = 56
		projectID2 = 78
		numMetrics = 10
	)
	tenantID1 := fmt.Sprintf("%d:%d", accountID1, projectID1)
	tenantID2 := fmt.Sprintf("%d:%d", accountID2, projectID2)
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli()
	end := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC).UnixMilli()
	data := apptest.GenerateTestData("metric", numMetrics, start, end)
	emptySeries := []map[string]string{}
	emptyLabels := []string{}
	emptyLabelValues := []string{}
	emptyQueryResults := []*apptest.QueryResult{}
	emptyMetadata := map[string][]apptest.MetadataEntry{}
	emptyMetricNamesStats := []apptest.MetricNamesStatsRecord{}

	vmsingle := tc.MustStartVmsingle("vmsingle", []string{
		"-storageDataPath=" + filepath.Join(tc.Dir(), "vmsingle"),
		"-retentionPeriod=100y",
		fmt.Sprintf("-accountID=%d", accountID1),
		fmt.Sprintf("-projectID=%d", projectID1),
	})
	vmselect := tc.MustStartVmselect("vmselect", []string{
		"-storageNode=" + vmsingle.VmselectAddr(),
	})

	vmsingle.PrometheusAPIV1ImportPrometheus(tc.T(), data.Samples, apptest.QueryOpts{})
	vmsingle.ForceFlush(t)

	// Ensure vmsingle returns data.
	apptest.AssertSeries(tc, vmsingle, "metric.*", "", start, end, data.WantSeries)
	apptest.AssertSeriesCount(tc, vmsingle, "", start, end, numMetrics)
	apptest.AssertLabels(tc, vmsingle, "metric.*", "", start, end, data.WantLabels)
	apptest.AssertLabelValues(tc, vmsingle, "metric.*", "label", "", start, end, data.WantLabelValues)
	apptest.AssertQueryResults(tc, vmsingle, "metric.*", "", start, end, data.Step, data.WantQueryResults)
	apptest.AssertMetadata(tc, vmsingle, "", "", data.WantMetadata)
	for i := range data.WantMetricNamesStats {
		data.WantMetricNamesStats[i].QueryRequestsCount = 1
	}
	apptest.AssertMetricNamesStats(tc, vmsingle, "", "", data.WantMetricNamesStats)

	// Check that current vmsingle tenant (configured via flags) is tenant1.
	gotAdminTenantsResponse := vmselect.APIV1AdminTenants(t, apptest.QueryOpts{})
	wantAdminTenantsResponse := &apptest.AdminTenantsResponse{
		Status: "success",
		Data:   []string{tenantID1},
	}
	if diff := cmp.Diff(wantAdminTenantsResponse, gotAdminTenantsResponse); diff != "" {
		t.Fatalf("unexpected tenants (-want, +got):\n%s", diff)
	}

	// Ensure vmselect returns data for tenant1.
	apptest.AssertSeries(tc, vmselect, "metric.*", tenantID1, start, end, data.WantSeries)
	apptest.AssertSeriesCount(tc, vmselect, tenantID1, start, end, numMetrics)
	apptest.AssertLabels(tc, vmselect, "metric.*", tenantID1, start, end, data.WantLabels)
	apptest.AssertLabelValues(tc, vmselect, "metric.*", "label", tenantID1, start, end, data.WantLabelValues)
	apptest.AssertQueryResults(tc, vmselect, "metric.*", tenantID1, start, end, data.Step, data.WantQueryResults)
	apptest.AssertMetadata(tc, vmselect, "", tenantID1, data.WantMetadata)
	for i := range data.WantMetricNamesStats {
		data.WantMetricNamesStats[i].QueryRequestsCount = 2
	}
	apptest.AssertMetricNamesStats(tc, vmselect, "", tenantID1, data.WantMetricNamesStats)

	// Ensure vmselect does not return any data for tenant2.
	apptest.AssertSeries(tc, vmselect, "metric.*", tenantID2, start, end, emptySeries)
	apptest.AssertSeriesCount(tc, vmselect, tenantID2, start, end, 0)
	apptest.AssertLabels(tc, vmselect, "metric.*", tenantID2, start, end, emptyLabels)
	apptest.AssertLabelValues(tc, vmselect, "metric.*", "label", tenantID2, start, end, emptyLabelValues)
	apptest.AssertQueryResults(tc, vmselect, "metric.*", tenantID2, start, end, data.Step, emptyQueryResults)
	apptest.AssertMetadata(tc, vmselect, "", tenantID2, emptyMetadata)
	apptest.AssertMetricNamesStats(tc, vmselect, "", tenantID2, emptyMetricNamesStats)

	// Ensure vmselect returns data for multitenant.
	for _, v := range data.WantSeries {
		v["vm_account_id"] = strconv.Itoa(accountID1)
		v["vm_project_id"] = strconv.Itoa(projectID1)
	}
	apptest.AssertSeries(tc, vmselect, "metric.*", "multitenant", start, end, data.WantSeries)
	data.WantLabels = append(data.WantLabels, "vm_account_id", "vm_project_id")
	apptest.AssertLabels(tc, vmselect, "metric.*", "multitenant", start, end, data.WantLabels)
	apptest.AssertLabelValues(tc, vmselect, "metric.*", "label", "multitenant", start, end, data.WantLabelValues)
	for _, v := range data.WantQueryResults {
		v.Metric["vm_account_id"] = strconv.Itoa(accountID1)
		v.Metric["vm_project_id"] = strconv.Itoa(projectID1)
	}
	apptest.AssertQueryResults(tc, vmselect, "metric.*", "multitenant", start, end, data.Step, data.WantQueryResults)
	apptest.AssertMetadata(tc, vmselect, "", "multitenant", data.WantMetadata)
	for i := range data.WantMetricNamesStats {
		data.WantMetricNamesStats[i].QueryRequestsCount = 3
	}
	apptest.AssertMetricNamesStats(tc, vmselect, "", "multitenant", data.WantMetricNamesStats)
}

func TestMixedDeleteSeries(t *testing.T) {
	tc := apptest.NewTestCase(t)
	defer tc.Stop()

	const (
		accountID1 = 12
		projectID1 = 34
		accountID2 = 56
		projectID2 = 78
		numMetrics = 10
	)
	tenantID1 := fmt.Sprintf("%d:%d", accountID1, projectID1)
	tenantID2 := fmt.Sprintf("%d:%d", accountID2, projectID2)
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli()
	end := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC).UnixMilli()
	data1 := apptest.GenerateTestData("metric1", numMetrics, start, end)
	data2 := apptest.GenerateTestData("metric2", numMetrics, start, end)
	emptySeries := []map[string]string{}

	vmsingle := tc.MustStartVmsingle("vmsingle", []string{
		"-storageDataPath=" + filepath.Join(tc.Dir(), "vmsingle"),
		"-retentionPeriod=100y",
		fmt.Sprintf("-accountID=%d", accountID1),
		fmt.Sprintf("-projectID=%d", projectID1),
	})
	vmselect := tc.MustStartVmselect("vmselect", []string{
		"-storageNode=" + vmsingle.VmselectAddr(),
	})

	vmsingle.PrometheusAPIV1ImportPrometheus(tc.T(), data1.Samples, apptest.QueryOpts{})
	vmsingle.PrometheusAPIV1ImportPrometheus(tc.T(), data2.Samples, apptest.QueryOpts{})
	vmsingle.ForceFlush(t)

	wantSeries12 := slices.Concat(data1.WantSeries, data2.WantSeries)
	apptest.AssertSeries(tc, vmsingle, "metric.*", "", start, end, wantSeries12)

	vmselect.PrometheusAPIV1AdminTSDBDeleteSeries(tc.T(), `{__name__=~"metric1.*"}`, apptest.QueryOpts{
		Tenant: tenantID1,
	})
	apptest.AssertSeries(tc, vmsingle, "metric.*", "", start, end, data2.WantSeries)
	vmselect.PrometheusAPIV1AdminTSDBDeleteSeries(tc.T(), `{__name__=~"metric2.*"}`, apptest.QueryOpts{
		Tenant: tenantID2,
	})
	apptest.AssertSeries(tc, vmsingle, "metric.*", "", start, end, data2.WantSeries)
	vmselect.PrometheusAPIV1AdminTSDBDeleteSeries(tc.T(), `{__name__=~"metric2.*"}`, apptest.QueryOpts{
		Tenant: "multitenant",
	})
	apptest.AssertSeries(tc, vmsingle, "metric.*", "", start, end, emptySeries)
}

func TestMixedGraphiteQueries(t *testing.T) {
	tc := apptest.NewTestCase(t)
	defer tc.Stop()

	const (
		accountID1 = 12
		projectID1 = 34
		accountID2 = 56
		projectID2 = 78
		numMetrics = 10
	)
	tenantID1 := fmt.Sprintf("%d:%d", accountID1, projectID1)
	tenantID2 := fmt.Sprintf("%d:%d", accountID2, projectID2)
	start := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli()
	end := time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC).UnixMilli()
	data := apptest.GenerateGraphiteTestData("metric", numMetrics, start, end)
	emptyMetricsIndex := []string{}
	emptyMetricsFind := []apptest.GraphiteMetric{}
	emptyMetricsExpand := []string{}
	emptyRenderedTargets := []apptest.GraphiteRenderedTarget{}

	vmsingle := tc.MustStartVmsingle("vmsingle", []string{
		"-storageDataPath=" + filepath.Join(tc.Dir(), "vmsingle"),
		"-retentionPeriod=100y",
		fmt.Sprintf("-accountID=%d", accountID1),
		fmt.Sprintf("-projectID=%d", projectID1),
	})
	vmselect := tc.MustStartVmselect("vmselect", []string{
		"-storageNode=" + vmsingle.VmselectAddr(),
	})

	vmsingle.GraphiteWrite(tc.T(), data.Samples, apptest.QueryOpts{})
	vmsingle.ForceFlush(t)

	// Ensure vmsingle returns data.
	apptest.AssertGraphiteMetricsIndex(tc, vmsingle, "", data.WantMetricsIndex)
	apptest.AssertGraphiteMetricsFind(tc, vmsingle, "metric.*", "", data.WantMetricsFind)
	apptest.AssertGraphiteMetricsExpand(tc, vmsingle, "metric.*", "", data.WantMetricsExpand)
	apptest.AssertGraphiteRender(tc, vmsingle, "metric.*", "", start, end, data.Step, data.WantRenderedTargets)

	// Ensure vmselect returns data for tenant1.
	apptest.AssertGraphiteMetricsIndex(tc, vmselect, tenantID1, data.WantMetricsIndex)
	apptest.AssertGraphiteMetricsFind(tc, vmselect, "metric.*", tenantID1, data.WantMetricsFind)
	apptest.AssertGraphiteMetricsExpand(tc, vmselect, "metric.*", tenantID1, data.WantMetricsExpand)
	apptest.AssertGraphiteRender(tc, vmselect, "metric.*", tenantID1, start, end, data.Step, data.WantRenderedTargets)

	// Ensure vmselect does not return any data for tenant2.
	apptest.AssertGraphiteMetricsIndex(tc, vmselect, tenantID2, emptyMetricsIndex)
	apptest.AssertGraphiteMetricsFind(tc, vmselect, "metric.*", tenantID2, emptyMetricsFind)
	apptest.AssertGraphiteMetricsExpand(tc, vmselect, "metric.*", tenantID2, emptyMetricsExpand)
	apptest.AssertGraphiteRender(tc, vmselect, "metric.*", tenantID2, start, end, data.Step, emptyRenderedTargets)
}
