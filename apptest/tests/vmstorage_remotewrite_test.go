package tests

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/VictoriaMetrics/VictoriaMetrics/apptest"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
	"github.com/golang/snappy"
)

func TestClusterVmstoragePrometheusRemoteWrite(t *testing.T) {
	tc := apptest.NewTestCase(t)
	defer tc.Stop()

	vmstorage := tc.MustStartVmstorage("vmstorage", []string{
		"-storageDataPath=" + tc.Dir() + "/vmstorage",
		"-retentionPeriod=100y",
		"-enableIngestionAPI",
	})
	vmselect := tc.MustStartVmselect("vmselect", []string{
		"-storageNode=" + vmstorage.VmselectAddr(),
	})

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

func TestClusterVmstoragePrometheusRemoteWriteMultitenant(t *testing.T) {
	tc := apptest.NewTestCase(t)
	defer tc.Stop()

	vmstorage := tc.MustStartVmstorage("vmstorage", []string{
		"-storageDataPath=" + tc.Dir() + "/vmstorage",
		"-retentionPeriod=100y",
		"-enableIngestionAPI",
	})
	vmselect := tc.MustStartVmselect("vmselect", []string{
		"-storageNode=" + vmstorage.VmselectAddr(),
	})

	vmstorage.PrometheusAPIV1Write(t, prompb.WriteRequest{
		Timeseries: []prompb.TimeSeries{
			{
				Labels: []prompb.Label{
					{Name: "__name__", Value: "vmstorage_remote_write_multitenant"},
					{Name: "vm_account_id", Value: "7"},
					{Name: "vm_project_id", Value: "9"},
					{Name: "job", Value: "direct"},
				},
				Samples: []prompb.Sample{
					{Value: 9, Timestamp: 1652169600000},
				},
			},
		},
		Metadata: []prompb.MetricMetadata{
			{
				MetricFamilyName: "vmstorage_remote_write_multitenant",
				Help:             "direct remote write metadata",
				Type:             prompb.MetricTypeGauge,
				AccountID:        7,
				ProjectID:        9,
			},
		},
	}, apptest.QueryOpts{Tenant: "multitenant"})
	vmstorage.ForceFlush(t)

	cmpOpt := cmpopts.IgnoreFields(apptest.PrometheusAPIV1QueryResponse{}, "Status", "Data.ResultType")
	gotQuery := vmselect.PrometheusAPIV1Query(t, `vmstorage_remote_write_multitenant`, apptest.QueryOpts{
		Tenant: "7:9",
		Time:   "2022-05-10T08:00:00.000Z",
	})
	wantQuery := apptest.NewPrometheusAPIV1QueryResponse(t,
		`{"data":{"result":[{"metric":{"__name__":"vmstorage_remote_write_multitenant","job":"direct"},"value":[1652169600,"9"]}]}}`,
	)
	if diff := cmp.Diff(wantQuery, gotQuery, cmpOpt); diff != "" {
		t.Fatalf("unexpected /api/v1/query response (-want, +got):\n%s", diff)
	}

	gotQuery = vmselect.PrometheusAPIV1Query(t, `vmstorage_remote_write_multitenant`, apptest.QueryOpts{
		Tenant: "8:9",
		Time:   "2022-05-10T08:00:00.000Z",
	})
	wantQuery = apptest.NewPrometheusAPIV1QueryResponse(t, `{"data":{"result":[]}}`)
	if diff := cmp.Diff(wantQuery, gotQuery, cmpOpt); diff != "" {
		t.Fatalf("unexpected /api/v1/query response for another tenant (-want, +got):\n%s", diff)
	}

	gotMetadata := vmselect.PrometheusAPIV1Metadata(t, ``, -1, apptest.QueryOpts{Tenant: "7:9"})
	wantMetadata := &apptest.PrometheusAPIV1Metadata{
		Status: "success",
		Data: map[string][]apptest.MetadataEntry{
			"vmstorage_remote_write_multitenant": {{Help: "direct remote write metadata", Type: "gauge"}},
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

	wr := prompb.WriteRequest{
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
	}
	data := snappy.Encode(nil, wr.MarshalProtobuf(nil))
	headers := make(http.Header)
	headers.Set("Content-Type", "application/x-protobuf")
	_, statusCode := tc.Client().Post(t, fmt.Sprintf("http://%s/insert/0:0/prometheus/api/v1/write", vmstorage.HTTPAddr()), data, headers)
	if statusCode != http.StatusBadRequest {
		t.Fatalf("unexpected status code: got %d; want %d when -enableIngestionAPI is disabled", statusCode, http.StatusBadRequest)
	}
}
