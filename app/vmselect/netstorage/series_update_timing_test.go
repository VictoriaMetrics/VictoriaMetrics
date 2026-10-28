package netstorage

import (
	"fmt"
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/storage"
)

func BenchmarkSeriesUpdateContext(b *testing.B) {
	for _, revisions := range []int{1, 10} {
		b.Run(fmt.Sprintf("revisions-%d", revisions), func(b *testing.B) {
			const seriesCount = 1000
			const blocksPerRevision = 2
			var metricNames [][]byte
			lookup := make([]string, 0, seriesCount)
			for i := range seriesCount {
				mn := storage.MetricName{
					MetricGroup: []byte("foo"),
				}
				mn.AddTag("instance", fmt.Sprintf("host-%d", i))
				lookup = append(lookup, string(mn.Marshal(nil)))
				mn.AddTag("rev", "0")
				for rev := range revisions {
					mn.Tags[1].Value = fmt.Appendf(mn.Tags[1].Value[:0], "%d", rev)
					metricName := mn.Marshal(nil)
					for range blocksPerRevision {
						metricNames = append(metricNames, metricName)
					}
				}
			}
			benchmarkSeriesUpdateContext(b, metricNames, lookup)
		})
	}
}

func benchmarkSeriesUpdateContext(b *testing.B, metricNames [][]byte, lookup []string) {
	b.ReportAllocs()
	b.SetBytes(int64(len(metricNames)))
	for b.Loop() {
		suc := newSeriesUpdateContext("rev", len(lookup))
		for i, metricName := range metricNames {
			isPatch, err := suc.tryAddUpdate(metricName, blockRef{
				addr: tmpBlockAddr{offset: uint64(i)},
			})
			if err != nil {
				b.Fatalf("unexpected error: %s", err)
			}
			if !isPatch {
				b.Fatalf("expecting patch for series #%d", i)
			}
		}
		for _, metricName := range lookup {
			if len(suc.getUpdatesByMetricName(metricName)) == 0 {
				b.Fatalf("missing patches for series %q", metricName)
			}
		}
	}
}
