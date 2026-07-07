package promscrape

import (
	"fmt"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutil"
)

func BenchmarkNewCompressedLabels(b *testing.B) {
	const numTargets = 1000
	labelSet := func(idx int) *promutil.Labels {
		return promutil.NewLabelsFromMap(map[string]string{
			"__address__":                                           fmt.Sprintf("10.0.%d.%d:9100", idx>>8, idx&0xff),
			"__meta_kubernetes_namespace":                           "default",
			"__meta_kubernetes_pod_name":                            fmt.Sprintf("test-%d", idx),
			"__meta_kubernetes_pod_uid":                             fmt.Sprintf("00000000-0000-0000-0000-%012d", idx),
			"__meta_kubernetes_pod_ip":                              fmt.Sprintf("10.0.%d.%d", idx>>8, idx&0xff),
			"__meta_kubernetes_pod_node_name":                       fmt.Sprintf("node-%d", idx%50),
			"__meta_kubernetes_pod_label_app":                       "monitoring",
			"__meta_kubernetes_pod_label_release":                   "prod",
			"__meta_kubernetes_pod_annotation_prometheus_io_scrape": "true",
			"__meta_kubernetes_pod_annotation_prometheus_io_port":   "9100",
			"job":      "k8spod",
			"instance": fmt.Sprintf("10.0.%d.%d:9100", idx>>8, idx&0xff),
		})
	}

	labelss := make([]*promutil.Labels, numTargets)
	for i := range labelss {
		labelss[i] = labelSet(i)
	}
	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		var i int
		for pb.Next() {
			_ = newCompressedLabels(labelss[i%numTargets])
			i++
		}
	})
}
