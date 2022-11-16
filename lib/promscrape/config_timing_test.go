package promscrape

import (
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
)

func BenchmarkInternLabelStrings(b *testing.B) {
	b.ReportAllocs()
	b.SetBytes(1)
	b.RunParallel(func(pb *testing.PB) {
		labels := []prompbmarshal.Label{
			{
				Name:  "job",
				Value: "node-exporter",
			},
			{
				Name:  "instance",
				Value: "foo.bar.baz:1234",
			},
			{
				Name:  "__meta_kubernetes_namespace",
				Value: "default",
			},
		}
		for pb.Next() {
			internLabelStrings(labels)
		}
	})
}
