package promscrape

import (
	"fmt"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutil"
)

func labelSet(idx int) *promutil.Labels {
	return promutil.NewLabelsFromMap(map[string]string{
		"__address__":                                           fmt.Sprintf("10.0.%d.%d:9100", idx>>8, idx&0xff),
		"__meta_kubernetes_namespace":                           "aboba",
		"__meta_kubernetes_pod_name":                            fmt.Sprintf("aboba-%d", idx),
		"__meta_kubernetes_pod_uid":                             fmt.Sprintf("00000000-0000-0000-0000-%012d", idx),
		"__meta_kubernetes_pod_ip":                              fmt.Sprintf("10.0.%d.%d", idx>>8, idx&0xff),
		"__meta_kubernetes_pod_node_name":                       fmt.Sprintf("node-%d", idx%50),
		"__meta_kubernetes_pod_label_app":                       "zombo",
		"__meta_kubernetes_pod_label_release":                   "prod",
		"__meta_kubernetes_pod_annotation_prometheus_io_scrape": "true",
		"__meta_kubernetes_pod_annotation_prometheus_io_port":   "9100",
		"job":      "k8spod",
		"instance": fmt.Sprintf("10.0.%d.%d:9100", idx>>8, idx&0xff),
	})
}

func BenchmarkNewCompressedLabelsHit(b *testing.B) {
	const numTargets = 1000
	labelss := make([]*promutil.Labels, numTargets)
	for i := range labelss {
		labelss[i] = labelSet(i)
	}
	for _, l := range labelss {
		_ = newCompressedLabels(l)
	}

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			_ = newCompressedLabels(labelss[i%numTargets])
			i++
		}
	})
}

func BenchmarkNewCompressedLabelsMiss(b *testing.B) {
	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		base := 0
		for pb.Next() {
			_ = newCompressedLabels(labelSet(base))
			base++
		}
	})
}
