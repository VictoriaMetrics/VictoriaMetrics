package mdx

import (
	"fmt"
	"sort"
	"strings"
	"sync/atomic"
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

	filter := NewFilter()

	f := func(input []prompb.TimeSeries, expectedOutput []prompb.TimeSeries, expectedInstanceMap map[string]int64) {
		t.Helper()
		output := filter.Filter(input, []prompb.TimeSeries{})
		if len(output) != len(expectedOutput) {
			t.Fatalf("unexpected output length; got %d; want %d", len(output), len(expectedOutput))
		}
		if timeSeriessToString(output) != timeSeriessToString(expectedOutput) {
			t.Fatalf("unexpected output; got %s; want %s", timeSeriessToString(output), timeSeriessToString(expectedOutput))
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
	f([]prompb.TimeSeries{{
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
		}},
		[]prompb.TimeSeries{
			{
				Labels: []prompb.Label{
					{Name: "__name__", Value: "vm_app_version"},
					{Name: "instance", Value: "victoria-metrics1:8428"},
					{Name: "job", Value: "test"},
				},
			},
			{
				Labels: []prompb.Label{
					{Name: "__name__", Value: "vm_app_version"},
					{Name: "instance", Value: "vmagent1:8429"},
					{Name: "job", Value: "test"},
				},
			},
		}, map[string]int64{
			fmt.Sprintf("%q:%q", "test", "victoria-metrics1:8428"): 0,
			fmt.Sprintf("%q:%q", "test", "vmagent1:8429"):          0,
		})

}

func TestMdxFilterByLabel(t *testing.T) {
	filter := Filter{
		vmInstance:    make(map[string]*atomic.Int64),
		filterByLabel: true,
	}
	*keepMetricsWithLabelName = "service"
	*keepMetricsWithLabelValue = "victoriametrics"

	f := func(input []prompb.TimeSeries, expectedOutput []prompb.TimeSeries) {
		t.Helper()
		output := filter.Filter(input, []prompb.TimeSeries{})
		if len(output) != len(expectedOutput) {
			t.Fatalf("unexpected output length; got %d; want %d", len(output), len(expectedOutput))
		}
		if timeSeriessToString(output) != timeSeriessToString(expectedOutput) {
			t.Fatalf("unexpected output; got %s; want %s", timeSeriessToString(output), timeSeriessToString(expectedOutput))
		}
	}
	f([]prompb.TimeSeries{{
		Labels: []prompb.Label{
			{Name: "__name__", Value: "up"},
			{Name: "instance", Value: "victoria-metrics1:8428"},
			{Name: "job", Value: "test"},
			{Name: "service", Value: "victoriametrics"},
		},
	},
		{
			Labels: []prompb.Label{
				{Name: "__name__", Value: "go_gc_duration_seconds"},
				{Name: "instance", Value: "node-exporter1"},
				{Name: "job", Value: "test"},
			},
		}},
		[]prompb.TimeSeries{
			{
				Labels: []prompb.Label{
					{Name: "__name__", Value: "up"},
					{Name: "instance", Value: "victoria-metrics1:8428"},
					{Name: "job", Value: "test"},
					{Name: "service", Value: "victoriametrics"},
				},
			},
		})
	*keepMetricsWithLabelName = ""
	*keepMetricsWithLabelValue = ""
}

func TestMdxInstanceCleanup(t *testing.T) {
	t.Helper()

	synctest.Test(t, func(t *testing.T) {
		filter := NewFilter()

		//  init instance list
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
			if len(filter.vmInstance) != len(expectedInstanceMap) {
				t.Fatalf("unexpected instance map length; got %d; want %d", len(filter.vmInstance), len(expectedInstanceMap))
			}
			for k := range expectedInstanceMap {
				if filter.vmInstance[k] == nil {
					t.Fatalf("missing instance in filter.vmInstance: %q", k)
				}
			}
		}
		f(map[string]int64{
			fmt.Sprintf("%q:%q", "test", "victoria-metrics1:8428"): 0,
			fmt.Sprintf("%q:%q", "test", "vmagent1:8429"):          0,
		})

		// receive samples from victoria-metrics1:8428 after 9 seconds.
		time.Sleep(59 * time.Minute)
		filter.Filter([]prompb.TimeSeries{
			{
				Labels: []prompb.Label{
					{Name: "__name__", Value: "vm_app_version"},
					{Name: "instance", Value: "victoria-metrics1:8428"},
					{Name: "job", Value: "test"},
				},
			}}, []prompb.TimeSeries{},
		)

		// no samples from vmagent1:8429 in the last 10 seconds, so it should be removed from the mdx instance list.
		time.Sleep(2 * time.Minute)
		f(map[string]int64{
			fmt.Sprintf("%q:%q", "test", "victoria-metrics1:8428"): 0,
		})
		filter.MustStop()
	})

}
