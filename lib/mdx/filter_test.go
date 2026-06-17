package mdx

import (
	"fmt"
	"sort"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promrelabel"
)

func timeSeriessToString(tss []prompb.TimeSeries) string {
	a := make([]string, len(tss))
	for i, ts := range tss {
		a[i] = timeSeriesToString(ts)
	}
	sort.Strings(a)
	return strings.Join(a, "")
}

func timeSeriesToString(ts prompb.TimeSeries) string {
	labelsString := promrelabel.LabelsToString(ts.Labels)

	return fmt.Sprintf("%s\n", labelsString)
}

func TestMdxInstanceFilter(t *testing.T) {
	originalVmLabel := *vmLabel
	*vmLabel = "service=victoriametrics"
	filter := NewFilter()
	defer filter.MustStop()
	f := func(input []prompb.TimeSeries, expectedOutput []prompb.TimeSeries, expectedInstanceMap map[string]int64) {
		t.Helper()
		output := filter.Filter(input, nil)
		outputString := timeSeriessToString(output)
		expectedOutputString := timeSeriessToString(expectedOutput)
		if outputString != expectedOutputString {
			t.Fatalf("unexpected output; got %s; want %s", outputString, expectedOutputString)
		}
		if len(filter.vmInstance) != len(expectedInstanceMap) {
			t.Fatalf("unexpected instance map length; got %d; want %d", len(filter.vmInstance), len(expectedInstanceMap))
		}
		for k := range expectedInstanceMap {
			if filter.vmInstance[k] == nil {
				t.Fatalf("missing instance in filter.vmInstance: %q", k)
			}
		}
	}
	// the first call
	f([]prompb.TimeSeries{
		// 1. metrics with vm_app_version and different order of labels.
		{
			Labels: []prompb.Label{
				{Name: "__name__", Value: "vm_app_version"},
				{Name: "instance", Value: "victoria-metrics1:8428"},
				{Name: "job", Value: "test"},
			},
		},
		{
			Labels: []prompb.Label{
				{Name: "instance", Value: "victoria-metrics2:8428"},
				{Name: "__name__", Value: "vm_app_version"},
				{Name: "job", Value: "test"},
			},
		},
		{
			Labels: []prompb.Label{
				{Name: "job", Value: "test"},
				{Name: "instance", Value: "victoria-metrics3:8428"},
				{Name: "__name__", Value: "vm_app_version"},
			},
		},
		// 2.
		// metrics without vm_app_version but with service=victoriametrics that is specified in `-vm.label`.
		// it will be preserved, but won't be registered in instance map in MDX
		{
			Labels: []prompb.Label{
				{Name: "__name__", Value: "vm_slow_queries_total"},
				{Name: "job", Value: "test"},
				{Name: "instance", Value: "victoria-metrics4:8428"},
				{Name: "service", Value: "victoriametrics"},
			},
		},

		// 3. metrics with vm_app_version and service=victoriametrics should be preserved.
		{
			Labels: []prompb.Label{
				{Name: "instance", Value: "victoria-metrics5:8428"},
				{Name: "job", Value: "test"},
				{Name: "service", Value: "victoriametrics"},
				{Name: "__name__", Value: "vm_app_version"},
			},
		},
		// 4. metrics without vm_app_version and `service=victoriametrics` but with `victoriametrics_app=true`, which should be preserved.
		{
			Labels: []prompb.Label{
				{Name: "instance", Value: "victoria-metrics6:8428"},
				{Name: "job", Value: "test"},
				{Name: "__name__", Value: "vm_slow_queries_total"},
				{Name: "victoriametrics_app", Value: "true"},
			},
		},

		// 5. metrics without vm_app_version and service=victoriametrics and `victoriametrics_app=true`, which should be filtered out.
		{
			Labels: []prompb.Label{
				{Name: "__name__", Value: "go_gc_duration_seconds"},
				{Name: "instance", Value: "node-exporter1"},
				{Name: "job", Value: "test"},
			},
		},
		{
			Labels: []prompb.Label{
				{Name: "__name__", Value: "http_request_duration_seconds"},
				{Name: "instance", Value: "service1"},
				{Name: "job", Value: "test"},
			},
		},

		// 6. metrics with vm_app_version but job or instance is empty (or missing), they should be dropped.
		{
			Labels: []prompb.Label{
				{Name: "__name__", Value: "vm_app_version"},
				{Name: "instance", Value: ""},
				{Name: "job", Value: "test"},
			},
		},
		{
			Labels: []prompb.Label{
				{Name: "__name__", Value: "vm_app_version"},
				{Name: "job", Value: "test"},
			},
		},
		{
			Labels: []prompb.Label{
				{Name: "__name__", Value: "vm_app_version"},
				{Name: "instance", Value: "vmagent2:8429"},
				{Name: "job", Value: ""},
			},
		},
		{
			Labels: []prompb.Label{
				{Name: "__name__", Value: "vm_app_version"},
				{Name: "instance", Value: "vmagent2:8429"},
			},
		},
	},
		// `victoriametrics_app=true` should be added to all preserved metrics if absent.
		[]prompb.TimeSeries{
			{
				Labels: []prompb.Label{
					{Name: "__name__", Value: "vm_app_version"},
					{Name: "instance", Value: "victoria-metrics1:8428"},
					{Name: "job", Value: "test"},
					{Name: "victoriametrics_app", Value: "true"},
				},
			},
			{
				Labels: []prompb.Label{
					{Name: "__name__", Value: "vm_app_version"},
					{Name: "instance", Value: "victoria-metrics2:8428"},
					{Name: "job", Value: "test"},
					{Name: "victoriametrics_app", Value: "true"},
				},
			},
			{
				Labels: []prompb.Label{
					{Name: "__name__", Value: "vm_app_version"},
					{Name: "instance", Value: "victoria-metrics3:8428"},
					{Name: "job", Value: "test"},
					{Name: "victoriametrics_app", Value: "true"},
				},
			},
			{
				Labels: []prompb.Label{
					{Name: "__name__", Value: "vm_slow_queries_total"},
					{Name: "service", Value: "victoriametrics"},
					{Name: "instance", Value: "victoria-metrics4:8428"},
					{Name: "job", Value: "test"},
					{Name: "victoriametrics_app", Value: "true"},
				},
			},
			{
				Labels: []prompb.Label{
					{Name: "instance", Value: "victoria-metrics5:8428"},
					{Name: "job", Value: "test"},
					{Name: "__name__", Value: "vm_app_version"},
					{Name: "service", Value: "victoriametrics"},
					{Name: "victoriametrics_app", Value: "true"},
				},
			},
			{
				Labels: []prompb.Label{
					{Name: "instance", Value: "victoria-metrics6:8428"},
					{Name: "job", Value: "test"},
					{Name: "__name__", Value: "vm_slow_queries_total"},
					{Name: "victoriametrics_app", Value: "true"},
				},
			},
		},
		// only instances that are discovered via `vm_app_version` will be registered in instance map in MDX.
		map[string]int64{
			"\"test\":\"victoria-metrics1:8428\"": 0,
			"\"test\":\"victoria-metrics2:8428\"": 0,
			"\"test\":\"victoria-metrics3:8428\"": 0,
		})

	// the second call
	f([]prompb.TimeSeries{
		// 1. metrics without vm_app_version, but the instances were already registered in the previous call, so it will be preserved.
		{
			Labels: []prompb.Label{
				{Name: "__name__", Value: "vm_rows_inserted_total"},
				{Name: "instance", Value: "victoria-metrics1:8428"},
				{Name: "job", Value: "test"},
			},
		},
		{
			Labels: []prompb.Label{
				{Name: "__name__", Value: "vminsert_request_duration_seconds_bucket"},
				{Name: "instance", Value: "victoria-metrics2:8428"},
				{Name: "job", Value: "test"},
			},
		},
		// 2. metrics without vm_app_version, `service=victoriametrics` and `victoriametrics_app=true`, and the instance wasn't already registered in the previous call, so it will be dropped.
		{
			Labels: []prompb.Label{
				{Name: "__name__", Value: "vminsert_request_duration_seconds_bucket"},
				{Name: "instance", Value: "victoria-metrics7:8428"},
				{Name: "job", Value: "test"},
			},
		},
		// 3. metrics with service=victoriametrics.
		{
			Labels: []prompb.Label{
				{Name: "service", Value: "victoriametrics"},
				{Name: "instance", Value: "victoria-metrics4:8428"},
				{Name: "job", Value: "test"},
			},
		},
	},
		[]prompb.TimeSeries{
			{
				Labels: []prompb.Label{
					{Name: "__name__", Value: "vm_rows_inserted_total"},
					{Name: "instance", Value: "victoria-metrics1:8428"},
					{Name: "job", Value: "test"},
					{Name: "victoriametrics_app", Value: "true"},
				},
			},
			{
				Labels: []prompb.Label{
					{Name: "__name__", Value: "vminsert_request_duration_seconds_bucket"},
					{Name: "instance", Value: "victoria-metrics2:8428"},
					{Name: "job", Value: "test"},
					{Name: "victoriametrics_app", Value: "true"},
				},
			},
			{
				Labels: []prompb.Label{
					{Name: "service", Value: "victoriametrics"},
					{Name: "instance", Value: "victoria-metrics4:8428"},
					{Name: "job", Value: "test"},
					{Name: "victoriametrics_app", Value: "true"},
				},
			},
		},
		// only instances that are discovered via `vm_app_version` will be registered in instance map in MDX.
		map[string]int64{
			"\"test\":\"victoria-metrics1:8428\"": 0,
			"\"test\":\"victoria-metrics2:8428\"": 0,
			"\"test\":\"victoria-metrics3:8428\"": 0,
		})

	*vmLabel = originalVmLabel
}

func TestMdxInstanceCleanup(t *testing.T) {
	t.Helper()

	synctest.Test(t, func(t *testing.T) {
		filter := NewFilter()
		defer filter.MustStop()
		filter.Filter([]prompb.TimeSeries{
			{
				Labels: []prompb.Label{
					{Name: "__name__", Value: "vm_app_version"},
					{Name: "instance", Value: "victoria-metrics1:8428"},
					{Name: "job", Value: "test"},
				},
			},
			{
				Labels: []prompb.Label{
					{Name: "__name__", Value: "go_gc_duration_seconds"},
					{Name: "instance", Value: "node-exporter1"},
					{Name: "job", Value: "test"},
				},
			},
			{
				Labels: []prompb.Label{
					{Name: "__name__", Value: "http_request_duration_seconds"},
					{Name: "instance", Value: "service1"},
					{Name: "job", Value: "test"},
				},
			},
			{
				Labels: []prompb.Label{
					{Name: "__name__", Value: "vm_app_version"},
					{Name: "instance", Value: "vmagent1:8429"},
					{Name: "job", Value: "test"},
				},
			}}, []prompb.TimeSeries{},
		)
		f := func(expectedInstanceMap map[string]int64) {
			t.Helper()
			if filter.VmInstancesCount() != len(expectedInstanceMap) {
				t.Fatalf("unexpected instance map length; got %d; want %d", len(filter.vmInstance), len(expectedInstanceMap))
			}
			for k := range expectedInstanceMap {
				if filter.vmInstance[k] == nil {
					t.Fatalf("missing instance in filter.vmInstance: %q", k)
				}
			}
		}

		time.Sleep(59 * time.Minute)
		// the entries should not be cleaned.
		f(map[string]int64{
			"\"test\":\"victoria-metrics1:8428\"": 0,
			"\"test\":\"vmagent1:8429\"":          0,
		})

		// receive samples from victoria-metrics1:8428 after 59 minutes.
		// so the entry will be refreshed.
		filter.Filter([]prompb.TimeSeries{
			{
				Labels: []prompb.Label{
					{Name: "__name__", Value: "vm_app_version"},
					{Name: "instance", Value: "victoria-metrics1:8428"},
					{Name: "job", Value: "test"},
				},
			}}, []prompb.TimeSeries{},
		)

		time.Sleep(2 * time.Minute)

		// no samples from vmagent1:8429 in the last hour, so it should be removed from the mdx instance list.
		f(map[string]int64{
			"\"test\":\"victoria-metrics1:8428\"": 0,
		})
	})

}
