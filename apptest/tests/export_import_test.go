package tests

import (
	"os"
	"testing"

	at "github.com/VictoriaMetrics/VictoriaMetrics/apptest"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestSingleExportImportNative(t *testing.T) {
	os.RemoveAll(t.Name())

	tc := at.NewTestCase(t)
	defer tc.Stop()

	sut := tc.MustStartDefaultVmsingle()

	testExportImportNative(tc.T(), sut)
}

func TestClusterExportImportNative(t *testing.T) {
	os.RemoveAll(t.Name())

	tc := at.NewTestCase(t)
	defer tc.Stop()

	sut := tc.MustStartDefaultCluster()

	testExportImportNative(tc.T(), sut)
	testExportImportNativeMultiTenant(tc.T(), sut)
}

// testExportImportNative test export and import in VictoriaMetrics’ native format.
// see: https://docs.victoriametrics.com/#how-to-import-data-in-native-format
func testExportImportNative(t *testing.T, sut at.PrometheusWriteQuerier) {
	// create test data
	sut.PrometheusAPIV1ImportPrometheus(t, []string{
		`native_export_import 10 1707123456700`, // 2024-02-05T08:57:36.700Z
	}, at.QueryOpts{
		ExtraLabels: []string{"el1=elv1", "el2=elv2", "vm_account_id=1", "vm_project_id=1"},
	})
	sut.ForceFlush(t)

	// export test data via native export API
	exportResult := sut.PrometheusAPIV1ExportNative(t, "native_export_import", at.QueryOpts{
		Start: "2024-02-05T08:50:00.700Z",
		End:   "2024-02-05T09:00:00.700Z",
	})

	// re-import test data via native import API
	sut.PrometheusAPIV1ImportNative(t, exportResult, at.QueryOpts{})
	sut.ForceFlush(t)

	// check query result
	got := sut.PrometheusAPIV1QueryRange(t, "native_export_import", at.QueryOpts{
		Start: "2024-02-05T08:57:36.700Z",
		End:   "2024-02-05T08:57:36.700Z",
		Step:  "60s",
	})

	cmpOptions := []cmp.Option{
		cmpopts.IgnoreFields(at.PrometheusAPIV1QueryResponse{}, "Status", "Data.ResultType"),
		cmpopts.EquateNaNs(),
	}
	want := at.NewPrometheusAPIV1QueryResponse(t, `{"data": {"result": [{"metric": {"__name__": "native_export_import", "el1": "elv1", "el2":"elv2", "vm_account_id": "1", "vm_project_id": "1"}, "values": []}]}}`)
	want.Data.Result[0].Samples = []*at.Sample{
		at.NewSample(t, "2024-02-05T08:57:36.700Z", 10),
	}
	if diff := cmp.Diff(want, got, cmpOptions...); diff != "" {
		t.Errorf("unexpected response (-want, +got):\n%s", diff)
	}
}

// testExportImportNativeMultiTenant test export and import in VictoriaMetrics’ native format, with multi-tenant.
// see: https://docs.victoriametrics.com/#how-to-import-data-in-native-format
func testExportImportNativeMultiTenant(t *testing.T, sut at.PrometheusWriteQuerier) {
	// create test data
	sut.PrometheusAPIV1ImportPrometheus(t, []string{
		`native_export_import_multi_tenant 10 1707123456700`, // 2024-02-05T08:57:36.700Z
	}, at.QueryOpts{
		ExtraLabels: []string{"el1=elv1", "el2=elv2"},
	})
	sut.ForceFlush(t)

	// export test data via native export API
	exportResult := sut.PrometheusAPIV1ExportNative(t, "native_export_import_multi_tenant", at.QueryOpts{
		Start:  "2024-02-05T08:50:00.700Z",
		End:    "2024-02-05T09:00:00.700Z",
		Tenant: "multitenant",
	})

	// re-import test data via native import API
	sut.PrometheusAPIV1ImportNative(t, exportResult, at.QueryOpts{
		Tenant: "multitenant",
	})
	sut.ForceFlush(t)

	// check query result
	got := sut.PrometheusAPIV1QueryRange(t, "native_export_import_multi_tenant", at.QueryOpts{
		Start: "2024-02-05T08:57:36.700Z",
		End:   "2024-02-05T08:57:36.700Z",
		Step:  "60s",
	})

	cmpOptions := []cmp.Option{
		cmpopts.IgnoreFields(at.PrometheusAPIV1QueryResponse{}, "Status", "Data.ResultType"),
		cmpopts.EquateNaNs(),
	}
	want := at.NewPrometheusAPIV1QueryResponse(t, `{"data": {"result": [{"metric": {"__name__": "native_export_import_multi_tenant", "el1": "elv1", "el2":"elv2"}, "values": []}]}}`)
	want.Data.Result[0].Samples = []*at.Sample{
		at.NewSample(t, "2024-02-05T08:57:36.700Z", 10),
	}
	if diff := cmp.Diff(want, got, cmpOptions...); diff != "" {
		t.Errorf("unexpected response (-want, +got):\n%s", diff)
	}
}
