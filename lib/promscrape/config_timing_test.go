package promscrape

import (
	"fmt"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/promutils"
)

func BenchmarkGetScrapeWork(b *testing.B) {
	swc := &scrapeWorkConfig{
		jobName:              "job-1",
		scheme:               "http",
		metricsPath:          "/metrics",
		scrapeIntervalString: "30s",
		scrapeTimeoutString:  "10s",
	}
	target := "host1.com:1234"
	extraLabels := promutils.NewLabelsFromMap(map[string]string{
		"env":        "prod",
		"datacenter": "dc-foo",
	})
	metaLabels := promutils.NewLabelsFromMap(map[string]string{
		"__meta_foo":                         "bar",
		"__meta_kubernetes_namespace":        "default",
		"__address__":                        "foobar.com",
		"__meta_sfdfdf_dsfds_fdfdfds_fdfdfd": "true",
	})
	b.ReportAllocs()
	b.SetBytes(1)
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			sw, err := swc.getScrapeWork(target, extraLabels, metaLabels)
			if err != nil {
				panic(fmt.Errorf("BUG: getScrapeWork returned non-nil error: %w", err))
			}
			if sw == nil {
				panic(fmt.Errorf("BUG: getScrapeWork returned nil ScrapeWork"))
			}
		}
	})
}
