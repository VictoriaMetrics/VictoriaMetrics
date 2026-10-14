package promql

import (
	"fmt"
	"slices"
	"testing"

	"github.com/VictoriaMetrics/metricsql"

	"github.com/VictoriaMetrics/VictoriaMetrics/app/vmselect/netstorage"
	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
)

func BenchmarkIncrementalAggrMaxSeriesInMemory(b *testing.B) {
	const seriesLen = 1_000_000
	metricNames := make([]string, seriesLen)
	for i := range metricNames {
		var mn storage.MetricName
		mn.MetricGroup = []byte("kube_pod_container_resource_requests")
		mn.AddTag("container", fmt.Sprintf("container-%d", i%20))
		mn.AddTag("instance", fmt.Sprintf("10.0.%d.%d:8080", i/256%256, i%256))
		mn.AddTag("job", "kube-state-metrics")
		mn.AddTag("namespace", fmt.Sprintf("namespace-%d", i%100))
		mn.AddTag("node", fmt.Sprintf("node-%d", i%1000))
		mn.AddTag("pod", fmt.Sprintf("pod-%d", i))
		mn.AddTag("resource", "memory")
		mn.AddTag("unit", "byte")
		metricNames[i] = string(mn.Marshal(nil))
	}

	for _, q := range []string{
		`sum(x)`,
		`sum(x) by (namespace)`,
		`sum(x) by (node)`,
		`max(x) by (pod)`,
		`sum(x) without (pod, instance)`,
	} {
		b.Run(q, func(b *testing.B) {
			expr, err := metricsql.Parse(q)
			if err != nil {
				b.Fatalf("cannot parse %q: %s", q, err)
			}
			iafc := &incrementalAggrFuncContext{
				ae:         expr.(*metricsql.AggrFuncExpr),
				byWorkerID: make([]incrementalAggrContextMap, netstorage.MaxWorkers()),
			}
			b.ReportAllocs()
			for b.Loop() {
				if _, err := iafc.maxSeriesInMemory(slices.Values(metricNames), seriesLen); err != nil {
					b.Fatalf("unexpected error: %s", err)
				}
			}
		})
	}
}
