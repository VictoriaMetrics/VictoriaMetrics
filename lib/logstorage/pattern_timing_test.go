package logstorage

import (
	"testing"
)

func BenchmarkPatternApply(b *testing.B) {
	a := []string{
		`{"level":"error","ts":1716113701.63973,"caller":"gcm/export.go:498","msg":"Failed to export self-observability metrics to Cloud Monitoring","error":"rpc error: code = PermissionDenied desc = Permission monitoring.timeSeries.create denied (or the resource may not exist).","stacktrace":"google3/cloud/kubernetes/metrics/common/gcm/gcm.(*exporter).startSelfObservability\n\tcloud/kubernetes/metrics/common/gcm/export.go:498","foo":"bar"}`,
		`{"level":"error","ts":1716113370.2321634,"caller":"gcm/export.go:434","msg":"Failed to export metrics to Cloud Monitoring","error":"rpc error: code = PermissionDenied desc = Permission monitoring.timeSeries.create denied (or the resource may not exist).","stacktrace":"google3/cloud/kubernetes/metrics/common/gcm/gcm.(*exporter).exportBuffer\n\tcloud/kubernetes/metrics/common/gcm/export.go:434\ngoogle3/cloud/kubernetes/metrics/common/gcm/gcm.(*exporter).flush\n\tcloud/kubernetes/metrics/common/gcm/export.go:383\ngoogle3/cloud/kubernetes/metrics/common/gcm/gcm.(*exporter).Flush\n\tcloud/kubernetes/metrics/common/gcm/export.go:365\ngoogle3/cloud/kubernetes/metrics/components/collector/adapter/adapter.(*adapter).Finalize\n\tcloud/kubernetes/metrics/components/collector/adapter/consume.go:131\ngoogle3/cloud/kubernetes/metrics/components/collector/prometheus/prometheus.(*parser).ParseText\n\tcloud/kubernetes/metrics/components/collector/prometheus/parse.go:158\ngoogle3/cloud/kubernetes/metrics/components/collector/collector.runScrapeLoop\n\tcloud/kubernetes/metrics/components/collector/collector.go:103\ngoogle3/cloud/kubernetes/metrics/components/collector/collector.Run\n\tcloud/kubernetes/metrics/components/collector/collector.go:81\ngoogle3/cloud/kubernetes/metrics/components/collector/collector.Start.func1\n\tcloud/kubernetes/metrics/components/collector/multi_target_collector.go:45","foo":"bar"}`,
		`{"level":"error","ts":1716113127.7496774,"caller":"collector/collector.go:105","msg":"Failed to process metrics","scrape_target":"http://localhost:8093/metrics","error":"failed to finalize exporting: \"2 errors occurred:\\n\\t* failed to export 1 (out of 1) batches of metrics to Cloud Monitoring\\n\\t* failed to export 1 (out of 1) batches of metrics to Cloud Monitoring\\n\\n\"","stacktrace":"google3/cloud/kubernetes/metrics/components/collector/collector.runScrapeLoop\n\tcloud/kubernetes/metrics/components/collector/collector.go:105\ngoogle3/cloud/kubernetes/metrics/components/collector/collector.Run\n\tcloud/kubernetes/metrics/components/collector/collector.go:81\ngoogle3/cloud/kubernetes/metrics/components/collector/collector.Start.func1\n\tcloud/kubernetes/metrics/components/collector/multi_target_collector.go:45","foo":"bar"}`,
		`{"level":"error","ts":1716113547.6429873,"caller":"gcm/export.go:498","msg":"Failed to export self-observability metrics to Cloud Monitoring","error":"rpc error: code = PermissionDenied desc = Permission monitoring.timeSeries.create denied (or the resource may not exist).","stacktrace":"google3/cloud/kubernetes/metrics/common/gcm/gcm.(*exporter).startSelfObservability\n\tcloud/kubernetes/metrics/common/gcm/export.go:498", "foo":"bar"}`,
		`{"level":"error","ts":1716113541.4445803,"caller":"periodicexporter/periodic_exporter.go:180","msg":"Failed to flush metrics to Cloud Monitoring","error":"1 error occurred:\n\t* failed to export 1 (out of 1) batches of metrics to Cloud Monitoring\n\n","stacktrace":"google3/cloud/kubernetes/metrics/common/periodicexporter/periodicexporter.(*Exporter).exportAll\n\tcloud/kubernetes/metrics/common/periodicexporter/periodic_exporter.go:180\ngoogle3/cloud/kubernetes/metrics/common/periodicexporter/periodicexporter.(*Exporter).periodicExporter\n\tcloud/kubernetes/metrics/common/periodicexporter/periodic_exporter.go:157","foo":"bar"}`,
	}

	b.Run("single-small-field-at-start", func(b *testing.B) {
		benchmarkPatternApply(b, `"level":"<level>"`, a)
	})
	b.Run("single-small-field-at-start-unquote", func(b *testing.B) {
		benchmarkPatternApply(b, `"level":<level>`, a)
	})
	b.Run("single-small-field-at-end", func(b *testing.B) {
		benchmarkPatternApply(b, `"foo":"<foo>"`, a)
	})
	b.Run("single-small-field-at-end-unquote", func(b *testing.B) {
		benchmarkPatternApply(b, `"foo":<foo>`, a)
	})
	b.Run("single-medium-field", func(b *testing.B) {
		benchmarkPatternApply(b, `"msg":"<message>"`, a)
	})
	b.Run("single-medium-field-unquote", func(b *testing.B) {
		benchmarkPatternApply(b, `"msg":<message>`, a)
	})
	b.Run("single-large-field", func(b *testing.B) {
		benchmarkPatternApply(b, `"stacktrace":"<stacktrace>"`, a)
	})
	b.Run("single-large-field-unquote", func(b *testing.B) {
		benchmarkPatternApply(b, `"stacktrace":<stacktrace>`, a)
	})
	b.Run("two-fields", func(b *testing.B) {
		benchmarkPatternApply(b, `"level":"<level>",<_>"msg":"<msg>"`, a)
	})
	b.Run("two-fields-unquote", func(b *testing.B) {
		benchmarkPatternApply(b, `"level":<level>,<_>"msg":<msg>`, a)
	})
	b.Run("many-fields", func(b *testing.B) {
		benchmarkPatternApply(b, `"level":"<level>","ts":"<ts>","caller":"<caller>","msg":"<msg>","error":"<error>"`, a)
	})
	b.Run("many-fields-unquote", func(b *testing.B) {
		benchmarkPatternApply(b, `"level":<level>,"ts":<ts>,"caller":<caller>,"msg":<msg>,"error":<error>`, a)
	})
}

func benchmarkPatternApply(b *testing.B, patternStr string, a []string) {
	ptnMain, err := parsePattern(patternStr)
	if err != nil {
		b.Fatalf("cannot parse pattern %q: %s", patternStr, err)
	}

	n := 0
	for _, s := range a {
		n += len(s)
	}

	b.ReportAllocs()
	b.SetBytes(int64(n))
	b.RunParallel(func(pb *testing.PB) {
		ptn := ptnMain.clone()
		sink := 0
		for pb.Next() {
			for _, s := range a {
				ptn.apply(s)
				for _, v := range ptn.matches {
					sink += len(v)
				}
			}
		}
		GlobalSink.Add(uint64(sink))
	})
}
