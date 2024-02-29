package streamaggr

import (
	"testing"

	"github.com/VictoriaMetrics/metrics"
)

var benchSeriesDedupPush = newBenchEncodedSeries(10e3, 5)

func BenchmarkDedupPush(b *testing.B) {
	ms := metrics.NewSet()
	defer ms.UnregisterAllMetrics()

	dd := newDeduplicator(ms, "test", 0, nil)

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			dd.push(benchSeriesDedupPush)
		}
	})
}

var benchSeriesDedupFlush = newBenchEncodedSeries(20e4, 5)

func BenchmarkDedupFlush(b *testing.B) {
	ms := metrics.NewSet()
	defer ms.UnregisterAllMetrics()

	noOp := func(tss []encodedTss) {}
	dd := newDeduplicator(ms, "test", 0, noOp)
	dd.push(benchSeriesDedupFlush)

	sm := dd.sm.Load()
	reset := func() {
		for _, sh := range sm.shards {
			sh.drained = false
		}
		dd.sm.Store(sm)
	}

	b.Run("flush", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			dd.flush()
			reset()
		}
	})
}

func newBenchEncodedSeries(seriesCount, samplesPerSeries int) []encodedTss {
	tss := newBenchSeries(seriesCount, samplesPerSeries)
	etss := make([]encodedTss, len(tss))
	le := &labelsEncoder{}
	for i := range tss {
		ts := tss[i]
		etss[i] = encodedTss{
			labels:  string(le.encode(nil, nil, ts.Labels)),
			samples: ts.Samples,
		}
	}
	return etss
}
