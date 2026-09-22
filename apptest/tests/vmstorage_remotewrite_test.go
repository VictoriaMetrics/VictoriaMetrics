package tests

import (
	"net/http"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/VictoriaMetrics/VictoriaMetrics/apptest"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
)

func TestClusterVmstoragePrometheusRemoteWrite(t *testing.T) {
	vmstorage, vmselect := startVmstorageRemoteWriteCluster(t)

	vmstorage.PrometheusAPIV1Write(t, prompb.WriteRequest{
		Timeseries: []prompb.TimeSeries{
			{
				Labels: []prompb.Label{
					{Name: "__name__", Value: "vmstorage_remote_write"},
					{Name: "job", Value: "direct"},
				},
				Samples: []prompb.Sample{
					{Value: 42, Timestamp: 1652169600000},
				},
			},
		},
	}, apptest.QueryOpts{})
	vmstorage.ForceFlush(t)

	cmpOpt := cmpopts.IgnoreFields(apptest.PrometheusAPIV1QueryResponse{}, "Status", "Data.ResultType")
	got := vmselect.PrometheusAPIV1Query(t, `vmstorage_remote_write`, apptest.QueryOpts{
		Tenant: "0:0",
		Time:   "2022-05-10T08:00:00.000Z",
	})
	want := apptest.NewPrometheusAPIV1QueryResponse(t,
		`{"data":{"result":[{"metric":{"__name__":"vmstorage_remote_write","job":"direct"},"value":[1652169600,"42"]}]}}`,
	)
	if diff := cmp.Diff(want, got, cmpOpt); diff != "" {
		t.Fatalf("unexpected /api/v1/query response (-want, +got):\n%s", diff)
	}
}

func TestClusterVmstoragePrometheusRemoteWriteMultitenancy(t *testing.T) {
	vmstorage, vmselect := startVmstorageRemoteWriteCluster(t)

	// insert data for tenant 1:2 using tenant URL.
	vmstorage.PrometheusAPIV1Write(t, prompb.WriteRequest{
		Timeseries: []prompb.TimeSeries{
			{
				Labels: []prompb.Label{
					{Name: "__name__", Value: "vmstorage_remote_write_tenant_url"},
					{Name: "job", Value: "tenant_url"},
				},
				Samples: []prompb.Sample{
					{Value: 12, Timestamp: 1652169600000},
				},
			},
		},
	}, apptest.QueryOpts{Tenant: "1:2"})

	// insert data for tenant 7:9 using multitenancy URL with tenant in label
	vmstorage.PrometheusAPIV1Write(t, prompb.WriteRequest{
		Timeseries: []prompb.TimeSeries{
			{
				Labels: []prompb.Label{
					{Name: "__name__", Value: "vmstorage_remote_write_multitenant"},
					{Name: "vm_account_id", Value: "7"},
					{Name: "vm_project_id", Value: "9"},
					{Name: "job", Value: "tenant_label"},
				},
				Samples: []prompb.Sample{
					{Value: 9, Timestamp: 1652169600000},
				},
			},
		},
	}, apptest.QueryOpts{Tenant: "multitenant"})
	vmstorage.ForceFlush(t)

	// query data for tenant 1:2
	cmpOpt := cmpopts.IgnoreFields(apptest.PrometheusAPIV1QueryResponse{}, "Status", "Data.ResultType")
	gotQuery := vmselect.PrometheusAPIV1Query(t, `vmstorage_remote_write_tenant_url`, apptest.QueryOpts{
		Tenant: "1:2",
		Time:   "2022-05-10T08:00:00.000Z",
	})
	wantQuery := apptest.NewPrometheusAPIV1QueryResponse(t,
		`{"data":{"result":[{"metric":{"__name__":"vmstorage_remote_write_tenant_url","job":"tenant_url"},"value":[1652169600,"12"]}]}}`,
	)
	if diff := cmp.Diff(wantQuery, gotQuery, cmpOpt); diff != "" {
		t.Fatalf("unexpected /api/v1/query response for url tenant (-want, +got):\n%s", diff)
	}

	// query data for tenant 7:9
	gotQuery = vmselect.PrometheusAPIV1Query(t, `vmstorage_remote_write_multitenant`, apptest.QueryOpts{
		Tenant: "7:9",
		Time:   "2022-05-10T08:00:00.000Z",
	})
	wantQuery = apptest.NewPrometheusAPIV1QueryResponse(t,
		`{"data":{"result":[{"metric":{"__name__":"vmstorage_remote_write_multitenant","job":"tenant_label"},"value":[1652169600,"9"]}]}}`,
	)
	if diff := cmp.Diff(wantQuery, gotQuery, cmpOpt); diff != "" {
		t.Fatalf("unexpected /api/v1/query response for multitenant labels (-want, +got):\n%s", diff)
	}

	// query data for tenant 0:0, expecting no data.
	gotQuery = vmselect.PrometheusAPIV1Query(t, `vmstorage_remote_write_tenant_url`, apptest.QueryOpts{
		Tenant: "0:0",
		Time:   "2022-05-10T08:00:00.000Z",
	})
	wantQuery = apptest.NewPrometheusAPIV1QueryResponse(t, `{"data":{"result":[]}}`)
	if diff := cmp.Diff(wantQuery, gotQuery, cmpOpt); diff != "" {
		t.Fatalf("unexpected /api/v1/query response for url tenant from another tenant (-want, +got):\n%s", diff)
	}
}

func TestClusterVmstoragePrometheusRemoteWriteMetadata(t *testing.T) {
	vmstorage, vmselect := startVmstorageRemoteWriteCluster(t)

	vmstorage.PrometheusAPIV1Write(t, prompb.WriteRequest{
		Metadata: []prompb.MetricMetadata{
			{
				MetricFamilyName: "vmstorage_remote_write_metadata_label",
				Help:             "direct remote write metadata",
				Type:             prompb.MetricTypeGauge,
			},
		},
	}, apptest.QueryOpts{Tenant: "7:9"})
	vmstorage.ForceFlush(t)

	gotMetadata := vmselect.PrometheusAPIV1Metadata(t, ``, -1, apptest.QueryOpts{Tenant: "7:9"})
	wantMetadata := &apptest.PrometheusAPIV1Metadata{
		Status: "success",
		Data: map[string][]apptest.MetadataEntry{
			"vmstorage_remote_write_metadata_label": {{Help: "direct remote write metadata", Type: "gauge"}},
		},
	}
	if diff := cmp.Diff(wantMetadata, gotMetadata); diff != "" {
		t.Fatalf("unexpected /api/v1/metadata response (-want, +got):\n%s", diff)
	}

	gotMetadata = vmselect.PrometheusAPIV1Metadata(t, ``, -1, apptest.QueryOpts{Tenant: "8:9"})
	wantMetadata = &apptest.PrometheusAPIV1Metadata{
		Status: "success",
		Data:   map[string][]apptest.MetadataEntry{},
	}
	if diff := cmp.Diff(wantMetadata, gotMetadata); diff != "" {
		t.Fatalf("unexpected /api/v1/metadata response for another tenant (-want, +got):\n%s", diff)
	}
}

func TestClusterVmstoragePrometheusRemoteWriteDisabled(t *testing.T) {
	tc := apptest.NewTestCase(t)
	defer tc.Stop()

	vmstorage := tc.MustStartVmstorage("vmstorage", []string{
		"-storageDataPath=" + tc.Dir() + "/vmstorage",
		"-retentionPeriod=100y",
	})

	vmstorage.PrometheusAPIV1WriteWithStatusCode(t, prompb.WriteRequest{
		Timeseries: []prompb.TimeSeries{
			{
				Labels: []prompb.Label{
					{Name: "__name__", Value: "vmstorage_remote_write_disabled"},
				},
				Samples: []prompb.Sample{
					{Value: 1, Timestamp: 1652169600000},
				},
			},
		},
	}, apptest.QueryOpts{}, http.StatusBadRequest)
}

func TestClusterVmstoragePrometheusRemoteWriteReadOnly(t *testing.T) {
	tc := apptest.NewTestCase(t)
	defer tc.Stop()

	vmstorage := tc.MustStartVmstorage("vmstorage", []string{
		"-storageDataPath=" + tc.Dir() + "/vmstorage",
		"-retentionPeriod=100y",
		"-enableIngestionAPI",
		"-storage.minFreeDiskSpaceBytes=1000000000000000000", // set min free disk space to a very high value to make vmstorage read-only
	})

	vmstorage.PrometheusAPIV1WriteWithStatusCode(t, prompb.WriteRequest{
		Timeseries: []prompb.TimeSeries{
			{
				Labels: []prompb.Label{
					{Name: "__name__", Value: "vmstorage_remote_write_read_only"},
				},
				Samples: []prompb.Sample{
					{Value: 1, Timestamp: 1652169600000},
				},
			},
		},
	}, apptest.QueryOpts{}, http.StatusServiceUnavailable)
}

func startVmstorageRemoteWriteCluster(t *testing.T) (*apptest.Vmstorage, *apptest.Vmselect) {
	t.Helper()

	tc := apptest.NewTestCase(t)
	t.Cleanup(tc.Stop)

	vmstorage := tc.MustStartVmstorage("vmstorage", []string{
		"-storageDataPath=" + tc.Dir() + "/vmstorage",
		"-retentionPeriod=100y",
		"-enableIngestionAPI",
	})
	vmselect := tc.MustStartVmselect("vmselect", []string{
		"-storageNode=" + vmstorage.VmselectAddr(),
	})
	return vmstorage, vmselect
}
