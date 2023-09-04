package promutils

import (
	"testing"
)

func BenchmarkLabelsInternStrings(b *testing.B) {
	b.ReportAllocs()
	b.SetBytes(1)
	b.RunParallel(func(pb *testing.PB) {
		labels := NewLabelsFromMap(map[string]string{
			"job":                         "node-exporter",
			"instance":                    "foo.bar.baz:1234",
			"__meta_kubernetes_namespace": "default",
		})
		for pb.Next() {
			labels.InternStrings()
		}
	})
}
