//go:build synctest

package mdx

import (
	"testing"
	"testing/synctest"
	"time"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
)

func TestMdxInstanceCleanup(t *testing.T) {

	synctest.Test(t, func(t *testing.T) {
		filter := NewFilter()
		defer filter.MustStop()
		assertFilterLen := func(expectedLen int) {
			t.Helper()
			if filter.VMInstancesCount() != expectedLen {
				t.Fatalf("unexpected instance map length; got %d; want %d", filter.VMInstancesCount(), expectedLen)
			}
		}

		ctx := GetContext()
		filter.Filter(ctx, []prompb.TimeSeries{
			{
				Labels: []prompb.Label{
					{Name: "__name__", Value: "vm_app_version"},
					{Name: "instance", Value: "victoria-metrics1:8428"},
					{Name: "job", Value: "test"},
				},
			},
			{
				Labels: []prompb.Label{
					{Name: "__name__", Value: "vm_up"},
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
			},
			{
				Labels: []prompb.Label{
					{Name: "__name__", Value: "scrape_targets_up"},
					{Name: "instance", Value: "vmagent1:8429"},
					{Name: "job", Value: "test"},
				},
			},
		},
		)
		PutContext(ctx)

		time.Sleep(1 * time.Minute)
		// the entries should not be cleaned.
		assertFilterLen(2)

		time.Sleep(58 * time.Minute)
		// receive samples from victoria-metrics1:8428 after 59 minutes.
		// so the entry will be refreshed.
		ctx = GetContext()
		filter.Filter(ctx, []prompb.TimeSeries{
			{
				Labels: []prompb.Label{
					{Name: "__name__", Value: "vm_up"},
					{Name: "instance", Value: "victoria-metrics1:8428"},
					{Name: "job", Value: "test"},
				},
			},
		},
		)
		PutContext(ctx)
		assertFilterLen(2)

		// entry for job:instance - test:vmagent1:8429 must be removed
		time.Sleep(4 * time.Minute)
		assertFilterLen(1)

		// no samples from vmagent1:8429 in the last hour, so it should be removed from the mdx instance list.
		time.Sleep(2 * time.Hour)

		assertFilterLen(0)
	})

}
