package mdx

import (
	"fmt"
	"sync"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
)

func TestMdxInstanceFilter(t *testing.T) {
	originalVmLabel := *vmLabel
	*vmLabel = "service=victoriametrics"
	t.Cleanup(func() {
		*vmLabel = originalVmLabel
	})
	f := func(input []prompb.TimeSeries, expectedOutput []prompb.TimeSeries) {
		t.Helper()
		filter := NewFilter()
		defer filter.MustStop()

		ctx := GetContext()
		defer PutContext(ctx)
		inputCopy := append([]prompb.TimeSeries{}, input...)
		output := filter.Filter(ctx, inputCopy)
		if diff := cmp.Diff(expectedOutput, output); len(diff) > 0 {
			t.Fatalf("unexpected result (-want, +got):\n%s", diff)
		}
		// make sure that result is the same over multiple calls
		inputCopy = append([]prompb.TimeSeries{}, input...)
		output = filter.Filter(ctx, inputCopy)
		if diff := cmp.Diff(expectedOutput, output); len(diff) > 0 {
			t.Fatalf("unexpected result (-want, +got):\n%s", diff)
		}

	}
	// metrics with vm_app_version and different order of labels.
	f([]prompb.TimeSeries{
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
	},
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
					{Name: "instance", Value: "victoria-metrics2:8428"},
					{Name: "__name__", Value: "vm_app_version"},
					{Name: "job", Value: "test"},
					{Name: "victoriametrics_app", Value: "true"},
				},
			},
			{
				Labels: []prompb.Label{
					{Name: "job", Value: "test"},
					{Name: "instance", Value: "victoria-metrics3:8428"},
					{Name: "__name__", Value: "vm_app_version"},
					{Name: "victoriametrics_app", Value: "true"},
				},
			}},
	)
	// metrics without vm_app_version but with service=victoriametrics that is specified in `-mdx.label`.
	f([]prompb.TimeSeries{
		{
			Labels: []prompb.Label{
				{Name: "__name__", Value: "vm_slow_queries_total"},
				{Name: "job", Value: "test"},
				{Name: "instance", Value: "victoria-metrics4:8428"},
				{Name: "service", Value: "victoriametrics"},
			},
		},
	},
		[]prompb.TimeSeries{
			{
				Labels: []prompb.Label{
					{Name: "__name__", Value: "vm_slow_queries_total"},
					{Name: "job", Value: "test"},
					{Name: "instance", Value: "victoria-metrics4:8428"},
					{Name: "service", Value: "victoriametrics"},
					{Name: "victoriametrics_app", Value: "true"},
				},
			},
		})
	// metrics without vm_app_version but with service=victoriametrics that is specified in `-mdx.label`.
	f([]prompb.TimeSeries{
		{
			Labels: []prompb.Label{
				{Name: "__name__", Value: "vm_slow_queries_total"},
				{Name: "job", Value: "test"},
				{Name: "instance", Value: "victoria-metrics4:8428"},
				{Name: "service", Value: "victoriametrics"},
			},
		}},
		[]prompb.TimeSeries{
			// 2.
			// metrics without vm_app_version but with service=victoriametrics that is specified in `-mdx.label`.
			{
				Labels: []prompb.Label{
					{Name: "__name__", Value: "vm_slow_queries_total"},
					{Name: "job", Value: "test"},
					{Name: "instance", Value: "victoria-metrics4:8428"},
					{Name: "service", Value: "victoriametrics"},
					{Name: "victoriametrics_app", Value: "true"},
				},
			}})

	// metrics with vm_app_version and service=victoriametrics should be preserved.
	f([]prompb.TimeSeries{
		{
			Labels: []prompb.Label{
				{Name: "instance", Value: "victoria-metrics5:8428"},
				{Name: "job", Value: "test"},
				{Name: "service", Value: "victoriametrics"},
				{Name: "__name__", Value: "vm_app_version"},
			},
		},
	},
		[]prompb.TimeSeries{
			{
				Labels: []prompb.Label{
					{Name: "instance", Value: "victoria-metrics5:8428"},
					{Name: "job", Value: "test"},
					{Name: "service", Value: "victoriametrics"},
					{Name: "__name__", Value: "vm_app_version"},
					{Name: "victoriametrics_app", Value: "true"},
				},
			},
		},
	)
	// metrics without vm_app_version and `service=victoriametrics` but with `victoriametrics_app=true`, which should be preserved.
	f([]prompb.TimeSeries{
		{
			Labels: []prompb.Label{
				{Name: "instance", Value: "victoria-metrics6:8428"},
				{Name: "job", Value: "test"},
				{Name: "__name__", Value: "vm_slow_queries_total"},
				{Name: "victoriametrics_app", Value: "true"},
			},
		},
	},
		[]prompb.TimeSeries{
			{
				Labels: []prompb.Label{
					{Name: "instance", Value: "victoria-metrics6:8428"},
					{Name: "job", Value: "test"},
					{Name: "__name__", Value: "vm_slow_queries_total"},
					{Name: "victoriametrics_app", Value: "true"},
				},
			},
		})

	// metrics without vm_app_version and service=victoriametrics and `victoriametrics_app=true`, which should be filtered out.
	f([]prompb.TimeSeries{
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
	},
		[]prompb.TimeSeries{},
	)

	// metrics with vm_app_version but job or instance is empty (or missing), they should be dropped.
	f([]prompb.TimeSeries{
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
		[]prompb.TimeSeries{})

	// metrics without vm_app_version, but the instances were already registered with first timeseries
	f([]prompb.TimeSeries{
		{
			Labels: []prompb.Label{
				{Name: "__name__", Value: "vm_app_version"},
				{Name: "instance", Value: "victoria-metrics1:8428"},
				{Name: "job", Value: "test"},
			},
		},
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
				{Name: "instance", Value: "victoria-metrics1:8428"},
				{Name: "job", Value: "test"},
			},
		},
	},
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
					{Name: "__name__", Value: "vm_rows_inserted_total"},
					{Name: "instance", Value: "victoria-metrics1:8428"},
					{Name: "job", Value: "test"},
					{Name: "victoriametrics_app", Value: "true"},
				},
			},
			{
				Labels: []prompb.Label{
					{Name: "__name__", Value: "vminsert_request_duration_seconds_bucket"},
					{Name: "instance", Value: "victoria-metrics1:8428"},
					{Name: "job", Value: "test"},
					{Name: "victoriametrics_app", Value: "true"},
				},
			},
		})

	// metrics without vm_app_version, `service=victoriametrics` and `victoriametrics_app=true`, and the instance wasn't already registered in the previous call, so it will be dropped.
	f([]prompb.TimeSeries{
		{
			Labels: []prompb.Label{
				{Name: "__name__", Value: "vminsert_request_duration_seconds_bucket"},
				{Name: "instance", Value: "victoria-metrics7:8428"},
				{Name: "job", Value: "test"},
			},
		},
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
					{Name: "service", Value: "victoriametrics"},
					{Name: "instance", Value: "victoria-metrics4:8428"},
					{Name: "job", Value: "test"},
					{Name: "victoriametrics_app", Value: "true"},
				},
			}},
	)

	// metrics with duplicate victoriametrics_app label
	f([]prompb.TimeSeries{
		{
			Labels: []prompb.Label{
				{Name: "__name__", Value: "vm_app_version"},
				{Name: "job", Value: "test"},
				{Name: "instance", Value: "victoria-metrics4:8428"},
				{Name: "service", Value: "victoriametrics"},
				{Name: "victoriametrics_app", Value: "other_value"},
			},
		},
	},
		[]prompb.TimeSeries{
			{
				Labels: []prompb.Label{
					{Name: "__name__", Value: "vm_app_version"},
					{Name: "job", Value: "test"},
					{Name: "instance", Value: "victoria-metrics4:8428"},
					{Name: "service", Value: "victoriametrics"},
					{Name: "victoriametrics_app", Value: "other_value"},
					{Name: "victoriametrics_app", Value: "true"},
				},
			},
		})

	// metrics with duplicate job and instance labels
	// last value wins
	f([]prompb.TimeSeries{
		{
			Labels: []prompb.Label{
				{Name: "__name__", Value: "vm_app_version"},
				{Name: "job", Value: "test"},
				{Name: "job", Value: "test2"},
				{Name: "instance", Value: "victoria-metrics4:8428"},
				{Name: "instance", Value: "victoria-metrics5:8428"},
				{Name: "service", Value: "victoriametrics"},
			},
		},
		{
			Labels: []prompb.Label{
				{Name: "__name__", Value: "http_requests_total"},
				{Name: "job", Value: "test2"},
				{Name: "instance", Value: "victoria-metrics5:8428"},
				{Name: "service", Value: "victoriametrics"},
			},
		},
	},
		[]prompb.TimeSeries{
			{
				Labels: []prompb.Label{
					{Name: "__name__", Value: "vm_app_version"},
					{Name: "job", Value: "test"},
					{Name: "job", Value: "test2"},
					{Name: "instance", Value: "victoria-metrics4:8428"},
					{Name: "instance", Value: "victoria-metrics5:8428"},
					{Name: "service", Value: "victoriametrics"},
					{Name: "victoriametrics_app", Value: "true"},
				},
			},
			{
				Labels: []prompb.Label{
					{Name: "__name__", Value: "http_requests_total"},
					{Name: "job", Value: "test2"},
					{Name: "instance", Value: "victoria-metrics5:8428"},
					{Name: "service", Value: "victoriametrics"},
					{Name: "victoriametrics_app", Value: "true"},
				},
			}})

}

func TestMdxInstanceFilterConcurrent(t *testing.T) {
	originalVmLabel := *vmLabel
	*vmLabel = "service=victoriametrics"
	t.Cleanup(func() { *vmLabel = originalVmLabel })

	filter := NewFilter()
	defer filter.MustStop()

	const concurrency = 8
	const iterations = 200

	generateSeries := func(g int) []prompb.TimeSeries {
		return []prompb.TimeSeries{
			{Labels: []prompb.Label{
				{Name: "__name__", Value: "vm_app_version"},
				{Name: "job", Value: "test"},
				{Name: "instance", Value: fmt.Sprintf("vm-%d:8428", g)},
			}},
			// shared job:instance
			{Labels: []prompb.Label{
				{Name: "__name__", Value: "vm_app_version"},
				{Name: "job", Value: "test"},
				{Name: "instance", Value: "vmagent:8428"},
			}},
		}
	}

	var wg sync.WaitGroup
	for worker := range concurrency {
		wg.Go(func() {
			input := generateSeries(worker)
			var expectedOutput []prompb.TimeSeries
			for _, inputTs := range input {
				labels := append([]prompb.Label{}, inputTs.Labels...)
				labels = append(labels, prompb.Label{Name: vmAppLabelName, Value: vmAppLabelValue})
				expectedOutput = append(expectedOutput, prompb.TimeSeries{Labels: labels})
			}
			for range iterations {
				ctx := GetContext()
				inputCopy := append([]prompb.TimeSeries{}, input...)
				output := filter.Filter(ctx, inputCopy)
				if diff := cmp.Diff(expectedOutput, output); len(diff) > 0 {
					t.Errorf("unexpected result (-want, +got):\n%s", diff)
				}
				PutContext(ctx)

			}
		})
	}
	wg.Wait()

	// goroutines + 1 shared
	if got := filter.VMInstancesCount(); got != concurrency+1 {
		t.Errorf("unexpected instance count: got %d, want %d", got, concurrency+1)
	}
}
