package tests

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/VictoriaMetrics/VictoriaMetrics/apptest"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/fs"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
)

func TestSingleMetricsMetadata(t *testing.T) {
	fs.MustRemoveDir(t.Name())

	tc := apptest.NewTestCase(t)
	defer tc.Stop()
	sut := tc.MustStartVmsingle("vmsingle", []string{
		"-storageDataPath=" + tc.Dir(),
		"-retentionPeriod=100y",
		"-enableMetadata",
	})
	// verify empty stats
	resp := sut.PrometheusAPIV1Metadata(t, "", 0, apptest.QueryOpts{})
	if len(resp.Data) != 0 {
		t.Fatalf("unexpected resp Records: %d, want: %d", len(resp.Data), 0)
	}

	const ingestTimestamp = 1707123456700
	prometheusTextDataSet := []string{
		`# HELP metric_name_1 some help message`,
		`# TYPE metric_name_1 gauge`,
		`metric_name_1{label="foo"} 10`,
		`metric_name_1{label="bar"} 10`,
		`metric_name_1{label="baz"} 10`,
		`# HELP metric_name_2 some help message`,
		`# TYPE metric_name_2 counter`,
		`metric_name_2{label="baz"} 20`,
		`# HELP metric_name_3 some help message`,
		`# TYPE metric_name_3 gauge`,
		`metric_name_3{label="baz"} 30`,
	}
	prometheusRemoteWriteDataSet := prompb.WriteRequest{
		Timeseries: []prompb.TimeSeries{
			{Labels: []prompb.Label{{Name: "__name__", Value: "metric_name_4"}}, Samples: []prompb.Sample{{Value: 40, Timestamp: ingestTimestamp}}},
			{Labels: []prompb.Label{{Name: "__name__", Value: "metric_name_5"}}, Samples: []prompb.Sample{{Value: 40, Timestamp: ingestTimestamp}}},
			{Labels: []prompb.Label{{Name: "__name__", Value: "metric_name_6"}}, Samples: []prompb.Sample{{Value: 40, Timestamp: ingestTimestamp}}},
		},
		Metadata: []prompb.MetricMetadata{
			{MetricFamilyName: "metric_name_4", Help: "some help message", Type: prompb.MetricTypeSummary},
			{MetricFamilyName: "metric_name_5", Help: "some help message", Type: prompb.MetricTypeSummary},
			{MetricFamilyName: "metric_name_6", Help: "some help message", Type: prompb.MetricTypeStateset},
		},
	}

	sut.PrometheusAPIV1ImportPrometheus(t, prometheusTextDataSet, apptest.QueryOpts{})
	sut.PrometheusAPIV1Write(t, prometheusRemoteWriteDataSet, apptest.QueryOpts{})
	sut.ForceFlush(t)
	expected := &apptest.PrometheusAPIV1Metadata{
		Status: "success",
		Data: map[string][]apptest.MetadataEntry{
			"metric_name_1": {{Help: "some help message", Type: "gauge"}},
			"metric_name_2": {{Help: "some help message", Type: "counter"}},
			"metric_name_3": {{Help: "some help message", Type: "gauge"}},
			"metric_name_4": {{Help: "some help message", Type: "summary"}},
			"metric_name_5": {{Help: "some help message", Type: "summary"}},
			"metric_name_6": {{Help: "some help message", Type: "stateset"}},
		},
	}
	gotStats := sut.PrometheusAPIV1Metadata(t, "", 0, apptest.QueryOpts{})
	if diff := cmp.Diff(expected, gotStats); diff != "" {
		t.Errorf("unexpected response (-want, +got):\n%s", diff)
	}

	// check query metric name filter
	tc.Assert(&apptest.AssertOptions{
		Msg: "unexpected /api/v1/metadata response",
		Got: func() any {
			return sut.PrometheusAPIV1Metadata(t, "metric_name_4", 0, apptest.QueryOpts{})
		},
		Want: &apptest.PrometheusAPIV1Metadata{
			Status: "success",
			Data: map[string][]apptest.MetadataEntry{
				"metric_name_4": {{Help: "some help message", Type: "summary"}},
			},
		},
	})

	// check query limit filter
	tc.Assert(&apptest.AssertOptions{
		Msg: "unexpected /api/v1/metadata response",
		Got: func() any {
			return sut.PrometheusAPIV1Metadata(t, "", 3, apptest.QueryOpts{})
		},
		Want: &apptest.PrometheusAPIV1Metadata{
			Status: "success",
			Data: map[string][]apptest.MetadataEntry{
				"metric_name_1": {{Help: "some help message", Type: "gauge"}},
				"metric_name_2": {{Help: "some help message", Type: "counter"}},
				"metric_name_3": {{Help: "some help message", Type: "gauge"}},
			},
		},
	})
}

func TestClusterMetricsMetadata(t *testing.T) {
	fs.MustRemoveDir(t.Name())

	tc := apptest.NewTestCase(t)
	defer tc.Stop()
	vmstorage1 := tc.MustStartVmstorage("vmstorage-1", []string{
		"-storageDataPath=" + tc.Dir() + "/vmstorage-1",
		"-retentionPeriod=100y",
	})
	vmstorage2 := tc.MustStartVmstorage("vmstorage-2", []string{
		"-storageDataPath=" + tc.Dir() + "/vmstorage-2",
		"-retentionPeriod=100y",
	})

	vminsert1 := tc.MustStartVminsert("vminsert1", []string{
		fmt.Sprintf("-storageNode=%s,%s", vmstorage1.VminsertAddr(), vmstorage2.VminsertAddr()),
		"-enableMetadata",
	})
	vminsert2 := tc.MustStartVminsert("vminsert-2", []string{
		fmt.Sprintf("-storageNode=%s,%s", vmstorage1.VminsertAddr(), vmstorage2.VminsertAddr()),
		"-enableMetadata",
	})
	vminsertGlobal := tc.MustStartVminsert("vminsert-global", []string{
		fmt.Sprintf("-storageNode=%s,%s", vminsert1.ClusternativeListenAddr(), vminsert2.ClusternativeListenAddr()),
		"-enableMetadata",
	})
	vmselect := tc.MustStartVmselect("vmselect", []string{
		fmt.Sprintf("-storageNode=%s,%s", vmstorage1.VmselectAddr(), vmstorage2.VmselectAddr()),
	})
	// verify empty stats
	resp := vmselect.PrometheusAPIV1Metadata(t, "", 0, apptest.QueryOpts{Tenant: "0:0"})
	if len(resp.Data) != 0 {
		t.Fatalf("unexpected resp Records: %d, want: %d", len(resp.Data), 0)
	}

	const ingestTimestamp = 1707123456700
	prometheusTextDataSet := []string{
		`# HELP metric_name_1 some help message`,
		`# TYPE metric_name_1 gauge`,
		`metric_name_1{label="foo"} 10`,
		`metric_name_1{label="bar"} 10`,
		`metric_name_1{label="baz"} 10`,
		`# HELP metric_name_2 some help message`,
		`# TYPE metric_name_2 counter`,
		`metric_name_2{label="baz"} 20`,
		`# HELP metric_name_3 some help message`,
		`# TYPE metric_name_3 gauge`,
		`metric_name_3{label="baz"} 30`,
	}
	prometheusRemoteWriteDataSet := prompb.WriteRequest{
		Timeseries: []prompb.TimeSeries{
			{Labels: []prompb.Label{{Name: "__name__", Value: "metric_name_4"}}, Samples: []prompb.Sample{{Value: 40, Timestamp: ingestTimestamp}}},
			{Labels: []prompb.Label{{Name: "__name__", Value: "metric_name_5"}}, Samples: []prompb.Sample{{Value: 40, Timestamp: ingestTimestamp}}},
			{Labels: []prompb.Label{{Name: "__name__", Value: "metric_name_6"}}, Samples: []prompb.Sample{{Value: 40, Timestamp: ingestTimestamp}}},
		},
		Metadata: []prompb.MetricMetadata{
			{MetricFamilyName: "metric_name_4", Help: "some help message", Type: prompb.MetricTypeSummary},
			{MetricFamilyName: "metric_name_5", Help: "some help message", Type: prompb.MetricTypeSummary},
			{MetricFamilyName: "metric_name_6", Help: "some help message", Type: prompb.MetricTypeStateset},
		},
	}

	assertMetadataIngestOn := func(t *testing.T, vminsert *apptest.Vminsert, tenantID string) {
		t.Helper()
		vminsert.PrometheusAPIV1ImportPrometheus(t, prometheusTextDataSet, apptest.QueryOpts{Tenant: tenantID})
		vminsert.PrometheusAPIV1Write(t, prometheusRemoteWriteDataSet, apptest.QueryOpts{Tenant: tenantID})
		vmstorage1.ForceFlush(t)
		vmstorage2.ForceFlush(t)
		expected := &apptest.PrometheusAPIV1Metadata{
			Status: "success",
			Data: map[string][]apptest.MetadataEntry{
				"metric_name_1": {{Help: "some help message", Type: "gauge"}},
				"metric_name_2": {{Help: "some help message", Type: "counter"}},
				"metric_name_3": {{Help: "some help message", Type: "gauge"}},
				"metric_name_4": {{Help: "some help message", Type: "summary"}},
				"metric_name_5": {{Help: "some help message", Type: "summary"}},
				"metric_name_6": {{Help: "some help message", Type: "stateset"}},
			},
		}
		gotStats := vmselect.PrometheusAPIV1Metadata(t, "", 0, apptest.QueryOpts{Tenant: tenantID})
		if diff := cmp.Diff(expected, gotStats); diff != "" {
			t.Errorf("unexpected response (-want, +got):\n%s", diff)
		}
	}

	assertMetadataIngestOn(t, vminsert1, "2:2")
	assertMetadataIngestOn(t, vminsert2, "3:3")
	assertMetadataIngestOn(t, vminsertGlobal, "5:5")

	// check query metric name filter
	tc.Assert(&apptest.AssertOptions{
		Msg: "unexpected /api/v1/metadata response",
		Got: func() any {
			return vmselect.PrometheusAPIV1Metadata(t, "metric_name_4", 0, apptest.QueryOpts{Tenant: "multitenant"})
		},
		Want: &apptest.PrometheusAPIV1Metadata{
			Status: "success",
			Data: map[string][]apptest.MetadataEntry{
				"metric_name_4": {{Help: "some help message", Type: "summary"}},
			},
		},
	})

	// check query limit filter
	tc.Assert(&apptest.AssertOptions{
		Msg: "unexpected /api/v1/metadata response",
		Got: func() any {
			return vmselect.PrometheusAPIV1Metadata(t, "", 3, apptest.QueryOpts{Tenant: "5:5"})
		},
		Want: &apptest.PrometheusAPIV1Metadata{
			Status: "success",
			Data: map[string][]apptest.MetadataEntry{
				"metric_name_1": {{Help: "some help message", Type: "gauge"}},
				"metric_name_2": {{Help: "some help message", Type: "counter"}},
				"metric_name_3": {{Help: "some help message", Type: "gauge"}},
			},
		},
	})
}
