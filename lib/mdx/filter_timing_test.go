package mdx

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompb"
)

func BenchmarkFilter(b *testing.B) {
	f := func(name string, input, want []prompb.TimeSeries) {
		b.Helper()

		b.Run(name, func(b *testing.B) {
			filter := NewFilter()
			defer filter.MustStop()
			b.ReportAllocs()
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					ctx := GetContext()
					localInput := append([]prompb.TimeSeries{}, input...)
					tss := filter.Filter(ctx, localInput)
					if len(tss) != len(want) {
						diff := cmp.Diff(want, tss)
						b.Fatalf("unexpected result (-want, +got):\n%s", diff)
					}
					PutContext(ctx)
				}
			})
		})
	}

	input := []prompb.TimeSeries{
		{
			Labels: []prompb.Label{
				{Name: "__name__", Value: "vm_app_version"},
				{Name: "instance", Value: "victoria-metrics1:8428"},
				{Name: "job", Value: "test"},
			},
		},
		{
			Labels: []prompb.Label{
				{Name: "instance", Value: "victoria-metrics1:8428"},
				{Name: "job", Value: "test"},
				{Name: "__name__", Value: "http_requests_total"},
			},
		},
		{
			Labels: []prompb.Label{
				{Name: "instance", Value: "victoria-metrics1:8428"},
				{Name: "job", Value: "test"},
				{Name: "__name__", Value: "http_requests_errors_total"},
			},
		},
	}
	expected := []prompb.TimeSeries{
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
				{Name: "instance", Value: "victoria-metrics1:8428"},
				{Name: "job", Value: "test"},
				{Name: "__name__", Value: "http_requests_total"},
				{Name: "victoriametrics_app", Value: "true"},
			},
		},
		{
			Labels: []prompb.Label{
				{Name: "instance", Value: "victoria-metrics1:8428"},
				{Name: "job", Value: "test"},
				{Name: "__name__", Value: "http_requests_errors_total"},
				{Name: "victoriametrics_app", Value: "true"},
			},
		},
	}
	f("match vm_app_version", input, expected)
}
