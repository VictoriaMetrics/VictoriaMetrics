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

// testTagSeries tests the registration of new time series in index.
//
// See https://graphite.readthedocs.io/en/stable/tags.html#adding-series-to-the-tagdb.
func testTagSeries(tc *apptest.TestCase, sut apptest.PrometheusWriteQuerier, getStorageMetric func(string) int) {
	t := tc.T()

	assertNewTimeseriesCreatedTotal := func(want int) {
		tc.Assert(&apptest.AssertOptions{
			Msg: "unexpected vm_new_timeseries_created_total",
			Got: func() any {
				return getStorageMetric("vm_new_timeseries_created_total")
			},
			Want: want,
		})
	}

	rec := "disk.used;rack=a1;datacenter=dc1;server=web01"
	got := sut.GraphiteTagsTagSeries(t, rec, apptest.QueryOpts{})
	// Want time series with sorted tags and enclosed in double quotes.
	want := `"disk.used;datacenter=dc1;rack=a1;server=web01"`
	if got != want {
		t.Fatalf("unexpected tag series: got %s, want %s", got, want)
	}
	assertNewTimeseriesCreatedTotal(1)

	recs := []string{
		"metric.yyy;t2=a;t1=b;t3=c",
		"metric.zzz;t5=d;t4=e;t6=f",
		"metric.xxx;t8=g;t7=h;t9=i",
	}
	gotMulti := sut.GraphiteTagsTagMultiSeries(t, recs, apptest.QueryOpts{})
	wantMulti := []string{
		"metric.yyy;t1=b;t2=a;t3=c",
		"metric.zzz;t4=e;t5=d;t6=f",
		"metric.xxx;t7=h;t8=g;t9=i",
	}
	if diff := cmp.Diff(wantMulti, gotMulti); diff != "" {
		t.Fatalf("unexpected tag series (-want, +got):\n%s", diff)
	}
	assertNewTimeseriesCreatedTotal(4)
}

func TestSingleTagSeries(t *testing.T) {
	tc := apptest.NewTestCase(t)
	defer tc.Stop()

	sut := tc.MustStartDefaultVmsingle()
	getStorageMetric := func(name string) int {
		return sut.GetIntMetric(t, name)
	}

	testTagSeries(tc, sut, getStorageMetric)
}

func TestClusterTagSeries(t *testing.T) {
	tc := apptest.NewTestCase(t)
	defer tc.Stop()

	sut := tc.MustStartDefaultCluster()
	getStorageMetric := func(name string) int {
		var v int
		for _, s := range sut.Vmstorages {
			v += s.GetIntMetric(t, name)
		}
		return v
	}

	testTagSeries(tc, sut, getStorageMetric)
}
