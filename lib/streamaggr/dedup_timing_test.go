package streamaggr

import (
	"testing"

	"github.com/VictoriaMetrics/VictoriaMetrics/lib/prompbmarshal"
)

var benchSeriesDedupPush = newBenchSeries(10e3, 5)

func BenchmarkDedupPush(b *testing.B) {
	dd := newDeduplicator(0, nil)

	b.SetBytes(int64(len(benchSeriesDedupPush)))
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			for _, ts := range benchSeriesDedupPush {
				dd.push(ts)
			}
		}
	})
}

var benchSeriesDedupFlush = newBenchSeries(20e4, 5)

func BenchmarkDedupFlush(b *testing.B) {
	noOp := func(tss []prompbmarshal.TimeSeries, matchIdxs []byte) {}
	dd := newDeduplicator(0, noOp)
	for _, ts := range benchSeriesDedupFlush {
		dd.push(ts)
	}

	de := dd.encoder.Load()
	reset := func() {
		for _, sh := range de.sm.shards {
			sh.drained = false
		}
		dd.encoder.Store(de)
	}

	b.SetBytes(int64(len(benchSeriesDedupFlush)))
	for i := 0; i < b.N; i++ {
		dd.flush()
		reset()
	}
}
