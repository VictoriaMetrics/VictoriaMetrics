package tests

import (
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

	// export all series, it must not increase counter for metric names stats
	sut.PrometheusAPIV1Export(t, `{__name__!=""}`, at.QueryOpts{})
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
